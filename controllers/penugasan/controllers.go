package penugasancontrollers

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

// Helper function for photo validation and saving
func validateAndSavePatrolPhoto(c *gin.Context, fileHeader *multipart.FileHeader, logID string, index int) (string, error) {
	log.Printf("File ditemukan: %s (%d bytes)", fileHeader.Filename, fileHeader.Size)
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("gagal membuka file")
	}
	defer file.Close()

	ext := filepath.Ext(fileHeader.Filename)
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true}
	if !allowedExts[ext] {
		return "", fmt.Errorf("format file tidak diizinkan. Hanya file jpg, jpeg, png yang diizinkan")
	}

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("gagal membaca file untuk validasi")
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return "", fmt.Errorf("gagal mereset file reader")
	}

	mimeType := http.DetectContentType(buffer)
	allowedMimes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
	}

	if !allowedMimes[mimeType] {
		return "", fmt.Errorf("tipe konten file tidak valid: %s", mimeType)
	}

	maxSize := int64(5 * 1024 * 1024) // 5MB
	if fileHeader.Size > maxSize {
		return "", fmt.Errorf("ukuran file terlalu besar. Maksimal 5MB per foto")
	}

	extension := filepath.Ext(fileHeader.Filename)
	stringToHash := fmt.Sprintf("%d-patrol-%s-%d", time.Now().UnixNano(), logID, index)
	hasher := sha256.New()
	hasher.Write([]byte(stringToHash))
	hashedFilename := hex.EncodeToString(hasher.Sum(nil)) + extension

	uploadDir := os.Getenv("PATROL_PATH")
	destinationPath := filepath.Join(uploadDir, hashedFilename)

	if err := c.SaveUploadedFile(fileHeader, destinationPath); err != nil {
		return "", fmt.Errorf("gagal menyimpan file")
	}

	return hashedFilename, nil
}

func deleteUploadedPatrolFiles(filenames []string) {
	uploadDir := os.Getenv("PATROL_PATH")
	for _, filename := range filenames {
		os.Remove(filepath.Join(uploadDir, filename))
	}
}

func StartPatrolHandler(c *gin.Context) {
	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var activePengerjaan models.PengerjaanTugas
	batasWaktu := time.Now().Add(-8 * time.Hour)

	err := models.DB.Where(
		"penempatan_id = ? AND status = ? AND waktu_mulai > ?",
		currentUser.Id,
		"berlangsung",
		batasWaktu,
	).First(&activePengerjaan).Error

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":      "Anda sudah memiliki sesi patroli yang sedang berlangsung",
			"patroli_id": activePengerjaan.Ptid,
		})
		return
	}

	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memeriksa sesi patroli"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")

	newPengerjaan := models.PengerjaanTugas{
		PenempatanId: currentUser.Id,
		WaktuMulai:   tanggalHariIni + " " + jamSaatIni,
		Status:       "berlangsung",
	}

	if err := models.DB.Create(&newPengerjaan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai sesi patroli"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Sesi patroli berhasil dimulai",
		"patroli_id": newPengerjaan.Ptid,
	})
}

type ScanCheckpointPayload struct {
	PatroliID      int64  `json:"patroli_id" binding:"required"`
	CheckpointKode string `json:"checkpoint_kode" binding:"required"`
}

func ScanCheckpointHandler(c *gin.Context) {
	var payload ScanCheckpointPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var patroli models.PengerjaanTugas
	err := models.DB.Where(
		"ptid = ? AND penempatan_id = ? AND status = ?",
		payload.PatroliID, currentUser.Id, "berlangsung",
	).First(&patroli).Error

	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Sesi patroli tidak valid atau bukan milik Anda"})
		return
	}

	// Check for incomplete location (location without photos)
	var incompleteLog models.CekTugas
	err = models.DB.Where(
		"ptid = ? AND (foto = '' OR foto IS NULL)",
		patroli.Ptid,
	).First(&incompleteLog).Error

	if err == nil {
		var checkpoint models.Checkpoint
		models.DB.Where("cid = ?", incompleteLog.Cid).First(&checkpoint)
		c.JSON(http.StatusConflict, gin.H{
			"error":             "Anda harus upload foto untuk lokasi '" + checkpoint.NamaLokasi + "' terlebih dahulu!",
			"incomplete_log_id": incompleteLog.Ctid,
			"nama_lokasi":       checkpoint.NamaLokasi,
		})
		return
	}

	var checkpoint models.Checkpoint
	if err := models.DB.Where("kode_qr = ?", payload.CheckpointKode).First(&checkpoint).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kode QR checkpoint tidak terdaftar"})
		return
	}

	var existingLog models.CekTugas
	err = models.DB.Where("ptid = ? AND cid = ?", patroli.Ptid, checkpoint.Cid).First(&existingLog).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Checkpoint ini sudah pernah Anda scan sebelumnya dalam sesi ini"})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")

	newLog := models.CekTugas{
		Ptid:      patroli.Ptid,
		Cid:       checkpoint.Cid,
		WaktuScan: tanggalHariIni + " " + jamSaatIni,
	}

	if err := models.DB.Create(&newLog).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencatat checkpoint"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Checkpoint '" + checkpoint.NamaLokasi + "' berhasil dicatat. Silakan upload foto sebagai bukti kehadiran",
		"log_id":      newLog.Ctid,
		"waktu_scan":  newLog.WaktuScan,
		"nama_lokasi": checkpoint.NamaLokasi,
	})
}

func UploadPatrolPhotoHandler(c *gin.Context) {
	logID := c.PostForm("log_id")
	if logID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "log_id dibutuhkan"})
		return
	}

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var cekLog models.CekTugas
	err := models.DB.Where("ctid = ?", logID).First(&cekLog).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Log checkpoint tidak ditemukan"})
		return
	}

	// Verify ownership without Preload
	var patroli models.PengerjaanTugas
	err = models.DB.Where("ptid = ? AND penempatan_id = ?", cekLog.Ptid, currentUser.Id).First(&patroli).Error
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Anda tidak memiliki akses ke log ini"})
		return
	}

	if cekLog.Foto != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "Foto sudah diupload untuk checkpoint ini"})
		return
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Gagal parsing form"})
		return
	}

	form, _ := c.MultipartForm()
	files := form.File["photos"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Minimal 1 foto dibutuhkan!"})
		return
	}
	if len(files) > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Maksimal 3 foto yang diizinkan!"})
		return
	}

	var savedFilenames []string
	for i, fileHeader := range files {
		filename, err := validateAndSavePatrolPhoto(c, fileHeader, logID, i)
		if err != nil {
			deleteUploadedPatrolFiles(savedFilenames)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Foto ke-%d gagal: %s", i+1, err.Error()),
			})
			return
		}
		savedFilenames = append(savedFilenames, filename)
	}

	photosJSON, _ := json.Marshal(savedFilenames)
	if err := models.DB.Model(&cekLog).Select("foto").Updates(map[string]interface{}{
		"foto": string(photosJSON),
	}).Error; err != nil {
		deleteUploadedPatrolFiles(savedFilenames)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data foto!"})
		return
	}

	log.Printf("Upload berhasil: %d foto patroli", len(savedFilenames))
	var checkpoint models.Checkpoint
	models.DB.Where("cid = ?", cekLog.Cid).First(&checkpoint)

	c.JSON(http.StatusOK, gin.H{
		"message":     "Foto berhasil diupload. Checkpoint ini selesai, silakan lanjut ke lokasi berikutnya!",
		"nama_lokasi": checkpoint.NamaLokasi,
		"foto":        savedFilenames,
		"jumlah_foto": len(savedFilenames),
	})
}

type EndPatrolPayload struct {
	PatroliID int64 `json:"patroli_id" binding:"required"`
}

func EndPatrolHandler(c *gin.Context) {
	var payload EndPatrolPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input patroli_id dibutuhkan"})
		return
	}

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var patroli models.PengerjaanTugas
	err := models.DB.Where(
		"ptid = ? AND penempatan_id = ? AND status = ?",
		payload.PatroliID, currentUser.Id, "berlangsung",
	).First(&patroli).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tidak ada sesi patroli aktif yang cocok untuk diakhiri"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencari sesi patroli"})
		return
	}

	// Check for incomplete checkpoints (without photos)
	var incompleteCount int64
	models.DB.Model(&models.CekTugas{}).Where(
		"ptid = ? AND (foto = '' OR foto IS NULL)",
		patroli.Ptid,
	).Count(&incompleteCount)

	if incompleteCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Masih ada %d checkpoint yang belum selesai (foto belum diupload)", incompleteCount),
		})
		return
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(loc)
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")
	nowString := tanggalHariIni + " " + jamSaatIni

	result := models.DB.Model(&patroli).Updates(models.PengerjaanTugas{
		WaktuSelesai: &nowString,
		Status:       "selesai",
	})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyelesaikan sesi patroli"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sesi patroli berhasil diselesaikan"})
}

func GetPenugasan(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Get Penugasan"})
}
