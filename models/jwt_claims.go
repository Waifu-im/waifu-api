package models

import "github.com/golang-jwt/jwt/v4"

type JwtCustomClaims struct {
	UserId     int64  `json:"user_id"`
	UserSecret string `json:"user_secret"`
	jwt.RegisteredClaims
}
