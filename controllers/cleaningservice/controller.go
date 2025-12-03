package cccontrollers

import (
	"SITEKAD/models"
	"fmt"
	"net/http"
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
		c.JSON(http.StatusConflict, gin.H{"error": "Lokasi ini sudah pernah Anda scan sebelumnya dalam sesi ini"})
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
		"message":    "Lokasi '" + checkpoint.NamaLokasi + "' berhasil dicatat. Silakan upload foto sebelum dan sesudah cleaning",
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

	var log models.CleaningService
	err := models.DB.Preload("PengerjaanTugas").Where("ccid = ?", logID).First(&log).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log cleaning tidak ditemukan"})
		return
	}

	if log.PengerjaanTugas.PenempatanId != currentUser.Id {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke log ini"})
		return
	}

	if log.FotoSebelum == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Foto sebelum harus diupload terlebih dahulu"})
		return
	}

	if log.FotoSesudah != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Foto sesudah sudah pernah diupload untuk lokasi ini"})
		return
	}

	file, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File foto dibutuhkan"})
		return
	}

	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("before_%s_%d%s", logID, time.Now().Unix(), ext)
	filepath := "./uploads/cleaning/" + filename

	if err := c.SaveUploadedFile(file, filepath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan foto"})
		return
	}

	if err := models.DB.Model(&log).Update("foto_sebelum", filename).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data foto"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Foto sebelum cleaning berhasil diupload",
		"foto_sebelum": filename,
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

	var log models.CleaningService
	err := models.DB.Preload("PengerjaanTugas").Where("ctid = ?", logID).First(&log).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log cleaning tidak ditemukan"})
		return
	}

	if log.PengerjaanTugas.PenempatanId != currentUser.Id {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke log ini"})
		return
	}

	file, err := c.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File foto dibutuhkan"})
		return
	}

	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("after_%s_%d%s", logID, time.Now().Unix(), ext)
	filepath := "./uploads/cleaning/" + filename

	if err := c.SaveUploadedFile(file, filepath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan foto"})
		return
	}

	if err := models.DB.Model(&log).Update("foto_sesudah", filename).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data foto"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Foto sesudah cleaning berhasil diupload",
		"foto_sesudah": filename,
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
