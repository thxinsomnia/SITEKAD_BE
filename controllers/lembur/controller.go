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
)

func StartOvertimeHandler(c *gin.Context) {
	// 1. Ambil data pengguna yang sudah disiapkan oleh middleware
	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	// 2. Ambil file dari request multipart
	file, err := c.FormFile("spl_file") // "spl_file" adalah nama field untuk file di form
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File SPL wajib diunggah"})
		return
	}

	// 3. Ambil data teks lainnya dari request multipart
	koordinat := c.PostForm("koordinat")
	androidID := c.PostForm("android_id")

	// 4. Buat nama file yang di-hash
	//    (timestamp + nama asli file untuk keunikan)
	extension := filepath.Ext(file.Filename)
	stringToHash := fmt.Sprintf("%d-%s", time.Now().UnixNano(), file.Filename)
	hasher := sha256.New()
	hasher.Write([]byte(stringToHash))
	hashedFilename := hex.EncodeToString(hasher.Sum(nil)) + extension

	// 5. Simpan file ke server
	//    Pastikan Anda sudah membuat folder "uploads/spl" di proyek Anda
	destinationPath := filepath.Join("uploads/spl", hashedFilename)
	if err := c.SaveUploadedFile(file, destinationPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan file"})
		return
	}

	// 6. Siapkan dan simpan data ke database
	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")

	newLembur := models.Lembur{
		Penempatan_id: currentUser.Id,
		Tad_id:        currentUser.Pkwt.TadId,
		Cabang_id:     currentUser.Cabang_id,
		Lokasi_id:     currentUser.Lokasi_kerja_id,
		Jabatan_id:    currentUser.Jabatan_id,
		Spl:           hashedFilename, // Simpan nama file yang sudah di-hash
		Tgl_absen:     tanggalHariIni,
		Jam_masuk:     jamSaatIni,
		Kordmasuk:     koordinat,
		Andid_masuk:   androidID,
	}

	if err := models.DB.Create(&newLembur).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan data lembur"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Sesi lembur berhasil dimulai",
		"file_disimpan": hashedFilename,
	})
}