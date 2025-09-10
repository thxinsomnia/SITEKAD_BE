package authcontroller

import (
	"net/http"
	"time"

	"SITEKAD/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"

	"SITEKAD/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Login(c *gin.Context) {
	var userInput models.Penempatan
	if err := c.ShouldBindJSON(&userInput); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"Message": err.Error()})
		return
	}

	var user models.Penempatan
	if err := models.DB.Where("username = ?", userInput.Username).First(&user).Error; err != nil {
		switch err {
		case gorm.ErrRecordNotFound:
			c.JSON(http.StatusUnauthorized, gin.H{"Message": "Username atau Password Tidak Sesuai"})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"Message": err.Error()})
			return
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(userInput.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"Message": "Username atau Password Tidak Sesuai"})
		return
	}

	expTime := time.Now().Add(time.Minute * 3600)
	claims := &config.JWTClaims{
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "go-jwt-mux",
			ExpiresAt: jwt.NewNumericDate(expTime),
		},
	}

	tokenDeklarasi := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := tokenDeklarasi.SignedString(config.JWT_KEY)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Message": err.Error()})
		return
	}

	c.SetCookie("token", token, 36000, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"Message": "Login Berhasil!", "Token": token})
}

type ActivationPayload struct {
	Nitad    string `json:"nitad" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func Aktivasi(c *gin.Context) {
	var payload ActivationPayload

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"Message": "Input tidak valid: " + err.Error()})
		return
	}

	var pkwt models.Pkwt
	err := models.DB.Where("nitad = ?", payload.Nitad).First(&pkwt).Error
	if err != nil {
		// Err Log
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"Message": "Nitad tidak terdaftar"})
			return
		}
		// Handle error
		c.JSON(http.StatusInternalServerError, gin.H{"Message": "Server error"})
		return
	}

	var existingUser models.Penempatan
	err = models.DB.Where("username = ?", payload.Username).First(&existingUser).Error

	if err == nil {
		// Jika err == nil, artinya GORM BERHASIL menemukan user. Username sudah dipakai.
		c.JSON(http.StatusConflict, gin.H{"Message": "Username sudah digunakan, silakan pilih yang lain"})
		return
	} else if err != gorm.ErrRecordNotFound {
		// Handle jika ada error database selain "tidak ditemukan"
		c.JSON(http.StatusInternalServerError, gin.H{"Message": "Gagal memvalidasi username"})
		return
	}

	hashPassword, _ := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)

	result := models.DB.Model(&models.Penempatan{}).Where("pkwt_id = ?", pkwt.Id).Updates(models.Penempatan{
		Username: payload.Username,
		Password: string(hashPassword),
	})

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Message": "Gagal mengaktifkan akun"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"Message": "Data pengguna untuk diaktifkan tidak ditemukan"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"Message": "Akun berhasil diaktifkan!"})
}

func Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"Message": "Logout Berhasil!"})
}
