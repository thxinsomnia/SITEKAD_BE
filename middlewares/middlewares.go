package middlewares

import (
	"net/http"
	"strings"
	"SITEKAD/config"
	"os"
	"SITEKAD/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

func JWTVerif() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := c.Cookie("token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"Message": "Tidak Terverifikasi!! Harap Login Terlebih dahulu!!"})
			c.Abort()
			return
		}

		claims := &config.JWTClaims{}

		//parsing token
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return config.JWT_KEY, nil
		})

		if err != nil {
			v, _ := err.(*jwt.ValidationError)
			switch v.Errors {
			case jwt.ValidationErrorSignatureInvalid:
				c.JSON(http.StatusUnauthorized, gin.H{"Message": "Tidak Terverifikasi!! Harap Login Terlebih dahulu!!"})
				c.Abort()
				return

			case jwt.ValidationErrorExpired:
				c.JSON(http.StatusUnauthorized, gin.H{"Message": "Silahkan Login Ulang Sesi Sudah Kadaluarsa!!"})
				c.Abort()
				return

			default:
				c.JSON(http.StatusUnauthorized, gin.H{"Message": "Tidak Terverifikasi!! Harap Login Terlebih dahulu!!"})
				c.Abort()
				return
			}
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"Message": "Tidak Terverifikasi!! Harap Login Terlebih dahulu!!"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Ambil token dari Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"Peringatan": "Silahkan Login Terlebih Dahulu!"})
			return
		}

		// Header format: "Bearer {token}"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"Peringatan": "Silahkan Login Terlebih Dahulu!"})
			return
		}

		tokenString := parts[1]
		secretKey := os.Getenv("JWT_KEY")

		// 2. Parse dan validasi token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// Pastikan algoritma sesuai
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.NewValidationError("Metode Signing Tidak Valid", jwt.ValidationErrorSignatureInvalid)
			}
			return []byte(secretKey), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"Error":"Token Tidak Valid atau Sudah Kedaluwarsa!"})
			return
		}

		// 3. Ekstrak ID pengguna dari claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"Error": "Gagal Memproses Token!"})
			return
		}

		// Ambil 'id' yang kita simpan saat login
		penempatanID := int64(claims["id"].(float64))

		// 4. Ambil data lengkap pengguna DENGAN PRELOAD
		var penempatan models.Penempatan
		err = models.DB.
			Preload("Lokasi").
			Preload("Cabang").
			Preload("Pkwt").
			Preload("Pkwt.Jabatan").
			Preload("Pkwt.Tad").
			First(&penempatan, penempatanID).Error

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"Error": "Pengguna Tidak Ditemukan!"})
			return
		}
		// 5. Simpan objek PENGGUNA YANG LENGKAP ke dalam context Gin
		c.Set("currentUser", penempatan)

		// Lanjutkan ke handler berikutnya
		c.Next()
	}
}
