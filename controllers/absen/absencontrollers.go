package absencontroller

import (
	"SITEKAD/helper"
	"SITEKAD/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetAllAbsen(c *gin.Context) {
	var absensi []models.Absensi

	if err := models.DB.Order("created_at desc").Find(&absensi).Limit(10).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"Absen": absensi})
}

type ScanPayload struct {
	Jenis     string  `json:"jenis"`
	KodeQr    string  `json:"kodeqr" binding:"required"`
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	AndroidID string  `json:"android_id" binding:"required"`
}

func ScanAbsensiHandler(c *gin.Context) {
	var payload ScanPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan)

	var qrCode models.LokasiPresensi
	err := models.DB.Where("kodeqr = ? AND penempatan_id = ?", payload.KodeQr, currentUser.Id).First(&qrCode).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusForbidden, gin.H{"error": "QR Code Tidak Valid, atau Anda Tidak Terdaftar di Lokasi Ini!"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memvalidasi QR Code"})
		return
	}

	const radiusDiizinkanMeter = 300.0

	jarak := helper.Geolocation(payload.Latitude, payload.Longitude, qrCode.Latitude, qrCode.Longitude)

	if jarak > radiusDiizinkanMeter {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Anda berada di luar jangkauan area absensi",
		})
		return
	}

	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")

	koordinatString := fmt.Sprintf("%f, %f", payload.Latitude, payload.Longitude)

	var absensi models.Absensi

	err = models.DB.Where("penempatan_id = ? AND tgl_absen = ?",
		currentUser.Id,
		tanggalHariIni,
	).First(&absensi).Error

	switch err {
	case gorm.ErrRecordNotFound:

		newAbsen := models.Absensi{
			Penempatan_id: currentUser.Id,
			Tad_id:        currentUser.Pkwt.TadId,
			Cabang_id:     currentUser.Cabang_id,
			Lokasi_id:     currentUser.Lokasi_kerja_id,
			Jabatan_id:    currentUser.Jabatan_id,
			Tgl_absen:     tanggalHariIni,
			Jam_masuk:     jamSaatIni,
			Kordmasuk:     koordinatString,
			Andid_masuk:   payload.AndroidID,
			Check:         tanggalHariIni + " " + jamSaatIni,
		}

		if err := models.DB.Create(&newAbsen).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan absensi masuk"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Check-in berhasil pada jam " + jamSaatIni})

	case nil:
		if absensi.Jam_keluar != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Anda sudah melakukan check-out hari ini"})
			return
		}

		batasDurasi := 12 * time.Hour
		durasiSesi := time.Since(absensi.CreatedAt)
		if durasiSesi > batasDurasi {
			c.JSON(http.StatusForbidden, gin.H{"error": "Sesi kerja Anda sudah lebih dari 12 jam. Harap hubungi admin."})
			return
		}

		var tanggalKeluar string
		hour := now.Hour()
		if hour >= 0 && hour < 6 {
			tanggalKeluar = now.AddDate(0, 0, -1).Format("2006-01-02")
		} else {
			tanggalKeluar = now.Format("2006-01-02")
		}

		models.DB.Model(&absensi).Updates(models.Absensi{
			Tgl_keluar:   &tanggalKeluar,
			Jam_keluar:   &jamSaatIni,
			Kordkeluar:   &koordinatString,
			Andid_keluar: &payload.AndroidID,
			Check:        tanggalKeluar + " " + jamSaatIni,
			Jenis:        &payload.Jenis,
		})
		c.JSON(http.StatusOK, gin.H{"message": "Check-out berhasil pada jam " + jamSaatIni})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Terjadi masalah pada server: " + err.Error()})
	}
}

func GetHistoryUser(c *gin.Context) {
	userData, exists := c.Get("currentUser")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi pengguna tidak valid"})
		return
	}
	currentUser := userData.(models.Penempatan)
	var history []models.Absensi
	err := models.DB.Where("penempatan_id = ?", currentUser.Id).Order("tgl_absen DESC, jam_masuk DESC").Find(&history).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil riwayat absensi"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": history})
}

// New handler for checkout prediction
func PrediksiCheckout(c *gin.Context) {
	userData, exists := c.Get("currentUser")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sesi pengguna tidak valid"})
		return
	}
	currentUser := userData.(models.Penempatan)

	// Get today's attendance record
	var todayAbsen models.Absensi
	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")

	err := models.DB.Where("penempatan_id = ? AND tgl_absen = ?",
		currentUser.Id, tanggalHariIni).First(&todayAbsen).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Belum melakukan check-in hari ini",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data absensi",
		})
		return
	}

	// Check if already checked out
	if todayAbsen.Jam_keluar != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Sudah melakukan check-out hari ini",
			"jam_keluar": *todayAbsen.Jam_keluar,
		})
		return
	}

	// Get historical data for prediction
	history, err := helper.GetTrainingDataForUser(currentUser.Id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal mengambil data historis",
		})
		return
	}

	// Check if enough data for prediction
	if len(history) < 3 {
		c.JSON(http.StatusOK, gin.H{
			"message":               "Data historis tidak cukup untuk prediksi (minimal 3 data diperlukan)",
			"check_in":              todayAbsen.Jam_masuk,
			"prediction_available":  false,
			"historical_data_count": len(history),
		})
		return
	}

	// Make prediction
	predictedCheckout, err := helper.PredictCheckoutTime(history, todayAbsen.Jam_masuk)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Gagal membuat prediksi: " + err.Error(),
		})
		return
	}

	// Return prediction
	c.JSON(http.StatusOK, gin.H{
		"check_in":              todayAbsen.Jam_masuk,
		"Prediksi Checkout":    predictedCheckout,
		// "prediction_available":  true,
		"Jumlah Absen": len(history),
	})
}
