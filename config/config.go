package config

import "github.com/golang-jwt/jwt/v4"

var JWT_KEY = []byte("qwertyuiop1234567890")

type JWTClaims struct {
	Username string
	jwt.RegisteredClaims
}
