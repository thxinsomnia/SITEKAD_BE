package lemburcontrollers

import (
	"SITEKAD/models"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func StartOvertimeHandler(c *gin.Context) {

	//get data dari token
	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	//cek udah checkin atau belum maks 12 jam
	var existingLembur models.Lembur
	twelveHoursAgo := time.Now().Add(-12 * time.Hour)
	err := models.DB.Where(
		"penempatan_id = ? AND jam_keluar IS NULL AND created_at > ?",
		currentUser.Id,
		twelveHoursAgo,
	).First(&existingLembur).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Anda sudah memiliki sesi lembur yang aktif. Harap selesaikan (check-out) sesi tersebut terlebih dahulu."})
		return
	}

	//console log
	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memverifikasi sesi lembur"})
		return
	}

	//cek file
	file, err := c.FormFile("spl_file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File SPL wajib diunggah!"})
		return
	}

	

	//ambil form data
	koordinat := c.PostForm("koordinat")
	androidID := c.PostForm("android_id")

	//hash
	extension := filepath.Ext(file.Filename)
	stringToHash := fmt.Sprintf("%d-%s", time.Now().UnixNano(), file.Filename)
	hasher := sha256.New()
	hasher.Write([]byte(stringToHash))
	hashedFilename := hex.EncodeToString(hasher.Sum(nil)) + extension

	//simpan file di server
	destinationPath := filepath.Join("/var/www/html/presensi/uploads/spl", hashedFilename)
	if err := c.SaveUploadedFile(file, destinationPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file"})
		return
	}

	// input ke db
	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")

	newLembur := models.Lembur{
		Penempatan_id: currentUser.Id,
		Tad_id:        currentUser.Pkwt.TadId,
		Cabang_id:     currentUser.Cabang_id,
		Lokasi_id:     currentUser.Lokasi_kerja_id,
		Jabatan_id:    currentUser.Jabatan_id,
		Spl:           hashedFilename,
		Tgl_absen:     tanggalHariIni,
		Jam_masuk:     jamSaatIni,
		Kordmasuk:     koordinat,
		Andid_masuk:   androidID,
		Check:         tanggalHariIni + " " + jamSaatIni,
	}

	if err := models.DB.Create(&newLembur).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data lembur"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":       "Sesi lembur berhasil dimulai",
		"file_disimpan": hashedFilename,
	})
}

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
