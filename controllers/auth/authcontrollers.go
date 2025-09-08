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
	var userInput models.User
	if err := c.ShouldBindJSON(&userInput); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"Message": err.Error()})
		return
	}

	var user models.User
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