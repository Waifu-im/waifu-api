package middlewares

import (
	"errors"
	"fmt"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"net/http"
	"strings"
)

// TokenVerification it will check if the token is present and its authenticity then add its user information to a context variable
// there is a skipper to skip if not needed
func TokenVerification(globals utils.Globals, skipper func(c echo.Context) (bool, error)) echo.MiddlewareFunc {
	if skipper == nil {
		skipper = defaultSkipper
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			skip, err := skipper(c)
			if err != nil {
				return err
			}
			if skip {
				return next(c)
			}
			authorization := c.Request().Header.Get("Authorization")
			splitToken := strings.Split(authorization, "Bearer ")
			if len(splitToken) != 2 {
				return &echo.HTTPError{
					Code:    http.StatusUnauthorized,
					Message: "Missing or malformed token",
				}
			}
			token := splitToken[1]

			//check if token is associated to a user in database (permanent token)
			user, err := globals.Database.GetUserInformationFromToken(token)

			if err != nil {
				return err
			}
			if user.Id == 0 {
				// if it is not check if it is a temporary one created on login in the website
				userId, err := globals.Redis.GetUserIdFromToken(token)
				if err == nil {
					// if there is no error, then the key exist,
					// so we extract the user information from the user id associated to the token
					user, err = globals.Database.GetUserInformationFromId(userId)
					if err != nil {
						return err
					}
				} else if !errors.Is(err, redis.Nil) {
					// if there was an error, and it is not the error that means no key were found
					return err
				}
				// finally check if the user information were found, if no it means the token was incorrect
				if user.Id == 0 {
					return &echo.HTTPError{
						Code:     http.StatusUnauthorized,
						Message:  fmt.Sprintf("Your token is incorrect or has expired, please check your current token at %v", globals.Config.WebSiteUrl+"/dashboard/"),
						Internal: err,
					}
				}
			}
			c.Set("user", user)
			return next(c)
		}
	}
}

// PermissionsVerification it will check if the user as the provided permission, there is a skipper to skip if not needed
func PermissionsVerification(globals utils.Globals, permissions []string, skipper func(c echo.Context) (bool, error)) echo.MiddlewareFunc {
	if skipper == nil {
		skipper = defaultSkipper
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			skip, err := skipper(c)
			if err != nil {
				return err
			}
			if skip {
				return next(c)
			}
			var targetUserId int64
			if targetUserIdInterface := c.Get("target_user_id"); targetUserIdInterface != nil {
				targetUserId = targetUserIdInterface.(int64)
			}
			user := utils.GetUser(c)
			missing, err := globals.Database.GetMissingPermissions(user.Id, targetUserId, permissions)

			if err != nil {
				return err
			}
			if len(missing) > 0 {
				return &echo.HTTPError{
					Code:     http.StatusForbidden,
					Message:  fmt.Sprintf("You are missing the following permissions : %v", strings.Join(missing, ", ")),
					Internal: err,
				}
			}
			return next(c)
		}
	}
}
