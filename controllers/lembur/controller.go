package lemburcontrollers

import (
	"SITEKAD/models"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)



func handleFileUpload(c *gin.Context) (string, error) {
	fileHeader, err := c.FormFile("spl_file")
	if err != nil {
		return "", fmt.Errorf("file SPL wajib diunggah")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("gagal membuka file")
	}
	defer file.Close()

	// Anda bisa menambahkan validasi tipe dan ukuran file di sini
	ext := filepath.Ext(fileHeader.Filename)
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".pdf": true}
	if !allowedExts[ext] {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "Format File Tidak Sesuai! Hanya file jpg, png, pdf Yang Diizinkan!"})
		return "", fmt.Errorf("format file tidak diizinkan")
	}

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", fmt.Errorf("gagal membaca file untuk validasi")
	}

	// PENTING: Kembalikan pointer file ke awal agar bisa disimpan nanti
	_, err = file.Seek(0, 0)
	if err != nil {
		return "", fmt.Errorf("gagal mereset file reader")
	}

	// Deteksi tipe konten
	mimeType := http.DetectContentType(buffer)
	allowedMimes := map[string]bool{
		"image/jpeg":      true,
		"image/png":       true,
		"application/pdf": true,
	}

	if !allowedMimes[mimeType] {
		return "", fmt.Errorf("tipe konten file terdeteksi tidak valid: %s", mimeType)
	}

	extension := filepath.Ext(fileHeader.Filename)
	stringToHash := fmt.Sprintf("%d-%s", time.Now().UnixNano(), fileHeader.Filename)
	hasher := sha256.New()
	hasher.Write([]byte(stringToHash))
	hashedFilename := hex.EncodeToString(hasher.Sum(nil)) + extension

	// Ambil path dari environment variable agar lebih fleksibel
	uploadPath := os.Getenv("UPLOAD_PATH")
	if uploadPath == "" {
		uploadPath = "uploads/spl" // Default path jika tidak di-set
	}

	destinationPath := filepath.Join(uploadPath, hashedFilename)
	if err := c.SaveUploadedFile(fileHeader, destinationPath); err != nil {
		return "", fmt.Errorf("gagal menyimpan file")
	}

	return hashedFilename, nil
}

// Handler utama yang sekarang lebih bersih


type EndOvertimePayload struct {
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	AndroidID string  `json:"android_id" binding:"required"`
}

func EndOvertimeHandler(c *gin.Context) {

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var payload EndOvertimePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return

	}

	koordinatString := fmt.Sprintf("%f, %f", payload.Latitude, payload.Longitude)
	var lembur models.Lembur
	err := models.DB.Where("penempatan_id = ? AND jam_keluar IS NULL", currentUser.Id).First(&lembur).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Tidak ada sesi lembur aktif yang ditemukan untuk diakhiri"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mencari data lembur"})
		return
	}

	if payload.AndroidID != lembur.Andid_masuk {
		c.JSON(http.StatusForbidden, gin.H{"error": "Perangkat yang digunakan untuk check-out berbeda dengan saat check-in"})
		return
	}

	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")
	result := models.DB.Model(&lembur).Updates(models.Lembur{
		Tgl_keluar:   &tanggalHariIni,
		Jam_keluar:   &jamSaatIni,
		Kordkeluar:   &koordinatString,
		Andid_keluar: &payload.AndroidID,
	})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyelesaikan sesi lembur"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Sesi lembur berhasil diakhiri pada jam " + jamSaatIni})
}
