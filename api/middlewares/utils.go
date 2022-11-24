package middlewares

import (
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
)

func GetUserClaims(c echo.Context) *JwtCustomClaims {
	user, ok := c.Get("user").(*jwt.Token)
	if !ok {
		return nil
	}
	claims := user.Claims.(*JwtCustomClaims)
	return claims
}

type JwtCustomClaims struct {
	UserId     uint   `json:"user_id"`
	UserSecret string `json:"user_secret"`
	jwt.StandardClaims
}

type User struct {
	Id uint `json:"user_id"`
}
