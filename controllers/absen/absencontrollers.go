package absencontroller

import (
	"SITEKAD/helper"
	"SITEKAD/models"
	"net/http"
	"time"
	"fmt"
	"os"
	"golang.org/x/crypto/bcrypt"
	"github.com/golang-jwt/jwt/v4"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Histori Absensi
func GetAllAbsen(c *gin.Context) {
	var absensi []models.Absensi

	if err := models.DB.Order("created_at desc").Find(&absensi).Limit(10).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"Absen": absensi})
}

type ScanPayload struct {
	Jenis		string `json:"jenis"`
	KodeQr     string `json:"kodeqr" binding:"required"`
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	AndroidID    string `json:"android_id" binding:"required"`
}

// func Absensi(c *gin.Context) {
// 	var payload ScanPayload

// 	if err := c.ShouldBindJSON(&payload); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
// 		return
// 	}

// 	now := time.Now()
// 	tanggalHariIni := now.Format("2006-01-02") //YYYY-MM-DD
// 	jamSaatIni := now.Format("15:04:05")       //HH:MM:SS

// 	// 3. Cari apakah sudah ada record absensi untuk user ini hari ini
// 	var absensi models.Absensi
// 	err := models.DB.Where("penempatan_id = ? AND tgl_absen = ?", payload.PenempatanID, tanggalHariIni).First(&absensi).Error

// 	// 4. Logika Percabangan: Check-In atau Check-Out?
// 	switch err {
// 	case gorm.ErrRecordNotFound:
// 		// --- KASUS CHECK-IN (Belum ada record hari ini) ---

// 		// Buat record baru
// 		newAbsen := models.Absensi{
// 			Penempatan_id: payload.PenempatanID,
// 			Tgl_absen:     tanggalHariIni,
// 			Jam_masuk:     jamSaatIni,
// 			Kordmasuk:     payload.Koordinat,
// 			Andid_masuk:   payload.AndroidID,
// 			Check: 	   tanggalHariIni + " " + jamSaatIni,
// 			// Field 'keluar' akan kosong secara default
// 		}

// 		// Simpan record baru ke database
// 		if err := models.DB.Create(&newAbsen).Error; err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan absensi masuk"})
// 			return
// 		}

// 		c.JSON(http.StatusCreated, gin.H{"message": "Check-in berhasil pada jam " + jamSaatIni})

// 	case nil:
// 		// --- KASUS CHECK-OUT (Sudah ada record hari ini) ---

// 		// Pertama, pastikan user belum pernah check-out
// 		if absensi.Jam_keluar != nil {
// 			c.JSON(http.StatusConflict, gin.H{"error": "Anda sudah melakukan check-out hari ini"})
// 			return
// 		}

// 		// Update record yang sudah ada dengan data check-out
// 		result := models.DB.Model(&absensi).Updates(models.Absensi{
// 			Tgl_keluar:   &tanggalHariIni,
// 			Jam_keluar:   &jamSaatIni,
// 			Kordkeluar:   &payload.Koordinat,
// 			Andid_keluar: &payload.AndroidID,
// 			Check: 	   tanggalHariIni + " " + jamSaatIni,
// 		})

// 		if result.Error != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan absensi keluar"})
// 			return
// 		}

// 		c.JSON(http.StatusOK, gin.H{"message": "Check-out berhasil pada jam " + jamSaatIni})

// 	default:
// 		// Handle jika ada error lain dari database saat pencarian
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Terjadi masalah pada server: " + err.Error()})
// 	}
// }

// 1. Buat struct khusus untuk payload login
type LoginPayload struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required"`
}

func LoginHandler(c *gin.Context) {
    var payload LoginPayload
    if err := c.ShouldBindJSON(&payload); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"Message": "Input username dan password dibutuhkan"})
        return
    }

    var user models.Penempatan
    // 2. Gunakan .Preload("Pkwt") untuk mengambil data dari tabel pkwt juga
    err := models.DB.Preload("Pkwt").Where("username = ?", payload.Username).First(&user).Error
    if err != nil {
        // Jika error (termasuk gorm.ErrRecordNotFound), pesannya sama agar lebih aman
        c.JSON(http.StatusUnauthorized, gin.H{"Message": "Username atau Password Tidak Sesuai"})
        return
    }

    // 3. Verifikasi password (tidak berubah)
    if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(payload.Password)); err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"Message": "Username atau Password Tidak Sesuai"})
        return
    }

    // 4. Buat token JWT dengan menyimpan ID pengguna
    claims := jwt.MapClaims{
        "id":  user.Id, // Simpan ID (primary key) dari penempatan
        "exp": time.Now().Add(time.Hour * 72).Unix(), // Token berlaku 3 hari
    }
    
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET_KEY")))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"Message": "Gagal membuat token"})
        return
    }
    
    // 5. Kirim token sebagai response JSON
    // (Menghapus SetCookie karena untuk API, token biasanya dikelola oleh client/frontend)
    c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func ScanAbsensiHandler(c *gin.Context) {
	var payload ScanPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Input tidak valid: " + err.Error()})
		return
	}

	var qrCode models.Lokasi
	err := models.DB.Where("kodeqr = ?", payload.KodeQr).First(&qrCode).Error
	if err != nil {
		// Jika tidak ditemukan atau ada error lain, tolak absensi
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusForbidden, gin.H{"error": "QR Code tidak valid atau tidak aktif"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memvalidasi QR Code"})
		return
	}


	const radiusDiizinkanMeter = 300.0 

	jarak := helper.Geolocation(payload.Latitude, payload.Longitude, qrCode.Latitude, qrCode.Longitude)

	if jarak > radiusDiizinkanMeter {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Anda berada di luar jangkauan area absensi",
		})
		return
	}
	// 2. Ambil data pengguna LENGKAP dari context (disiapkan oleh middleware)
	userData, _ := c.Get("currentUser")
	currentUser := userData.(models.Penempatan) // currentUser sekarang berisi data Penempatan & Pkwt

	now := time.Now()
	tanggalHariIni := now.Format("2006-01-02")
	jamSaatIni := now.Format("15:04:05")

	koordinatString := fmt.Sprintf("%f, %f", payload.Latitude, payload.Longitude)

	// 3. Cari record absensi menggunakan ID dari currentUser yang tepercaya
	var absensi models.Absensi
	err = models.DB.Where("penempatan_id = ? AND tgl_absen = ?", currentUser.Id, tanggalHariIni).First(&absensi).Error

	// 4. Logika Percabangan (sama seperti sebelumnya, tapi sumber datanya berbeda)
	switch err {
		case gorm.ErrRecordNotFound:
		// --- KASUS CHECK-IN ---

		// 5. Gunakan data LENGKAP dari currentUser untuk mengisi record baru
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
			Check: 	   tanggalHariIni + " " + jamSaatIni,
		}

		if err := models.DB.Create(&newAbsen).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal menyimpan absensi masuk"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"message": "Check-in berhasil pada jam " + jamSaatIni})

	case nil:
		// --- KASUS CHECK-OUT ---
		if absensi.Jam_keluar != nil { // Menggunakan pointer, cek nil
			c.JSON(http.StatusConflict, gin.H{"error": "Anda sudah melakukan check-out hari ini"})
			return
		}

		models.DB.Model(&absensi).Updates(models.Absensi{
			Tgl_keluar:   &tanggalHariIni,
			Jam_keluar:   &jamSaatIni,
			Kordkeluar:   &koordinatString,
			Andid_keluar: &payload.AndroidID,
			Check: 	   tanggalHariIni + " " + jamSaatIni,
			Jenis: &payload.Jenis,
		})
        c.JSON(http.StatusOK, gin.H{"message": "Check-out berhasil pada jam " + jamSaatIni})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Terjadi masalah pada server: " + err.Error()})
	}
}