package cccontrollers

import (
	"SITEKAD/models"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)



func StartCleaningHandler(c *gin.Context) {
	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var activeCleaning models.PengerjaanTugas
	batasWaktu := time.Now().Add(-8 * time.Hour)

	err := models.DB.Where(
		"penempatan_id = ? AND status = ? AND waktu_mulai > ?",
		currentUser.Id,
		"berlangsung",
		batasWaktu,
	).First(&activeCleaning).Error

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":       "Anda sudah memiliki sesi cleaning yang sedang berlangsung",
			"cleaning_id": activeCleaning.Ptid,
		})
		return
	}

	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memeriksa sesi cleaning"})
		return
	}

	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")

	newCleaning := models.PengerjaanTugas{
		PenempatanId: currentUser.Id,
		WaktuMulai:   tanggalHariIni + " " + jamSaatIni,
		Status:       "berlangsung",
	}

	if err := models.DB.Create(&newCleaning).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai sesi cleaning"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "Sesi cleaning berhasil dimulai",
		"cleaning_id": newCleaning.Ptid,
	})
}

type ScanCleaningLocationPayload struct {
	CleaningID     int64  `json:"cleaning_id" binding:"required"`
	CheckpointKode string `json:"checkpoint_kode" binding:"required"`
}

func ScanCleaningLocationHandler(c *gin.Context) {
	var payload ScanCleaningLocationPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var cleaning models.PengerjaanTugas
	err := models.DB.Where(
		"ptid = ? AND penempatan_id = ? AND status = ?",
		payload.CleaningID, currentUser.Id, "berlangsung",
	).First(&cleaning).Error

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sesi cleaning tidak valid atau bukan milik Anda"})
		return
	}

	var checkpoint models.Checkpoint
	if err := models.DB.Where("kode_qr = ?", payload.CheckpointKode).First(&checkpoint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kode QR lokasi tidak terdaftar"})
		return
	}

	var existingLog models.CleaningService
	err = models.DB.Where("ptid = ? AND cid = ?", cleaning.Ptid, checkpoint.Cid).First(&existingLog).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Lokasi ini sudah pernah Anda scan dalam sesi ini"})
		return
	}

	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")

	newLog := models.CleaningService{
		Ptid:      cleaning.Ptid,
		Cid:       checkpoint.Cid,
		WaktuScan: tanggalHariIni + " " + jamSaatIni,
	}

	if err := models.DB.Create(&newLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat lokasi cleaning"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Lokasi '" + checkpoint.NamaLokasi + "' berhasil dicatat. Silakan upload foto sebelum dan sesudah",
		"log_id":     newLog.Ccid,
		"waktu_scan": newLog.WaktuScan,
	})
}

func UploadBeforePhotoHandler(c *gin.Context) {
	logID := c.PostForm("log_id")
	if logID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "log_id dibutuhkan"})
		return
	}

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var clog models.CleaningService
	err := models.DB.Preload("PengerjaanTugas").Where("ccid = ?", logID).First(&clog).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log cleaning tidak ditemukan"})
		return
	}

	if clog.PengerjaanTugas.PenempatanId != currentUser.Id {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke log ini"})
		return
	}

	// Parse multipart form with max memory 32MB
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal parsing form"})
		return
	}

	// Get multiple files
	form, _ := c.MultipartForm()
	files := form.File["photos"]

	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal 1 foto dibutuhkan"})
		return
	}

	// Limit max files
	if len(files) > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maksimal 5 foto per upload"})
		return
	}

	var savedFilenames []string

	// Validate and save all files
	for i, fileHeader := range files {
		filename, err := validateAndSavePhoto(c, fileHeader, "before", logID, i)
		if err != nil {
			// Delete already saved files on error
			deleteUploadedFiles(savedFilenames)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Foto ke-%d gagal: %s", i+1, err.Error()),
			})
			return
		}
		savedFilenames = append(savedFilenames, filename)
	}

	// Get existing photos if any
	var existingPhotos []string
	if clog.FotoSebelum != "" {
		json.Unmarshal([]byte(clog.FotoSebelum), &existingPhotos)
	}

	// Append new photos
	existingPhotos = append(existingPhotos, savedFilenames...)

	// Convert to JSON string
	photosJSON, _ := json.Marshal(existingPhotos)

	if err := models.DB.Model(&clog).Select("foto_sebelum").Updates(map[string]interface{}{
        "foto_sebelum": string(photosJSON),
    }).Error; err != nil {
		deleteUploadedFiles(savedFilenames)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data foto"})
		return
	}

	log.Printf("Upload berhasil: %d foto sebelum cleaning", len(savedFilenames))

	c.JSON(http.StatusOK, gin.H{
		"message":        "Foto sebelum cleaning berhasil diupload",
		"foto_sebelum":   existingPhotos,
		"total_uploaded": len(savedFilenames),
	})
}

func UploadAfterPhotoHandler(c *gin.Context) {
	logID := c.PostForm("log_id")
	if logID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "log_id dibutuhkan"})
		return
	}

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var clog models.CleaningService
	err := models.DB.Preload("PengerjaanTugas").Where("ccid = ?", logID).First(&clog).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log cleaning tidak ditemukan"})
		return	
	}

	if clog.PengerjaanTugas.PenempatanId != currentUser.Id {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke log ini"})
		return
	}

	if clog.FotoSebelum == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Foto sebelum harus diupload terlebih dahulu"})
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal parsing form"})
		return
	}

	form, _ := c.MultipartForm()
	files := form.File["photos"]

	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal 1 foto dibutuhkan"})
		return
	}

	if len(files) > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maksimal 5 foto per upload"})
		return
	}

	var savedFilenames []string

	for i, fileHeader := range files {
		filename, err := validateAndSavePhoto(c, fileHeader, "after", logID, i)
		if err != nil {
			deleteUploadedFiles(savedFilenames)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Foto ke-%d gagal: %s", i+1, err.Error()),
			})
			return
		}
		savedFilenames = append(savedFilenames, filename)
	}

	var existingPhotos []string
	if clog.FotoSesudah != "" {
		json.Unmarshal([]byte(clog.FotoSesudah), &existingPhotos)
	}

	existingPhotos = append(existingPhotos, savedFilenames...)
	photosJSON, _ := json.Marshal(existingPhotos)

	if err := models.DB.Model(&clog).Select("foto_sesudah").Updates(map[string]interface{}{
        "foto_sesudah": string(photosJSON),
    }).Error; err != nil {
		deleteUploadedFiles(savedFilenames)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data foto"})
		return
	}

	log.Printf("Upload berhasil: %d foto sesudah cleaning", len(savedFilenames))

	c.JSON(http.StatusOK, gin.H{
		"message":        "Foto sesudah cleaning berhasil diupload",
		"foto_sesudah":   existingPhotos,
		"total_uploaded": len(savedFilenames),
	})
}

type EndCleaningPayload struct {
	CleaningID int64 `json:"cleaning_id" binding:"required"`
}

func EndCleaningHandler(c *gin.Context) {
	var payload EndCleaningPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input cleaning_id dibutuhkan"})
		return
	}

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var cleaning models.PengerjaanTugas
	err := models.DB.Where(
		"ptid = ? AND penempatan_id = ? AND status = ?",
		payload.CleaningID, currentUser.Id, "berlangsung",
	).First(&cleaning).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tidak ada sesi cleaning aktif yang cocok untuk diakhiri"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencari sesi cleaning"})
		return
	}

	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")
	nowString := tanggalHariIni + " " + jamSaatIni

	result := models.DB.Model(&cleaning).Updates(models.PengerjaanTugas{
		WaktuSelesai: &nowString,
		Status:       "selesai",
	})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyelesaikan sesi cleaning"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sesi cleaning berhasil diselesaikan"})
}
