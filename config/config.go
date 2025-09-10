package config

import (
	"github.com/golang-jwt/jwt/v4"
	"github.com/joho/godotenv"
	"log"
	"os"
)


var JWT_KEY []byte
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	key := os.Getenv("JWT_KEY")
	if key == "" {
		log.Fatal("JWT_KEY must be set in .env file")
	}

	// 2. ISI NILAI di dalam fungsi (Mengisi botol yang sudah ada)
	// Kita tidak menggunakan 'var' atau ':=' lagi di sini.
	JWT_KEY = []byte(key)
}
type JWTClaims struct {
	Username string
	jwt.RegisteredClaims
}
