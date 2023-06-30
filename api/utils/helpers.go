package utils

import (
	"github.com/Waifu-im/waifu-api/models"
	"github.com/labstack/echo/v4"
)

func GetUser(c echo.Context) models.User {
	user, ok := c.Get("user").(models.User)
	// We don't return an error because it means the token was not provided, and it was allowed to skip the verification
	// Would have failed earlier if token was required
	if !ok {
		return models.User{}
	}
	return user
}
