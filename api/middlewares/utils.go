package middlewares

import (
	"github.com/Waifu-im/waifu-api/models"
	"github.com/golang-jwt/jwt/v4"
	"github.com/labstack/echo/v4"
)

func GetUserClaims(c echo.Context) *models.JwtCustomClaims {
	user, ok := c.Get("user").(*jwt.Token)
	// We don't return an error because it means the token was not provided, and it was allowed to skip the verification
	// Would have failed earlier if token was required
	if !ok {
		return nil
	}
	claims := user.Claims.(*models.JwtCustomClaims)
	return claims
}
