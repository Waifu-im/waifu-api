package image

import (
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/Waifu-im/waifu-api/serializers"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Controller struct {
	Globals utils.Globals
}

// RouteSelector Since most of the code is the same for /fav and /search I wrap the handler inside a function
// Then the handler use the 'parent' function favRoot boolean to know if it should add or not the user id to constants methods
// The parent function is itself a method of Root so that it can access api package globals variables (api.Globals struct)

func (controller Controller) RouteSelector(favRoute bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		var isNsfw = constants.False
		var includedTags []string
		var excludedTags []string
		var includedFiles []string
		var excludedFiles []string
		var gif = constants.Null
		var orderBy = constants.Random
		var orientation = constants.Random
		var many = false
		var full = false
		var warning string
		var userId uint
		// We check if the user provided user_id
		userIdInterface := c.Get("target_user_id")
		if userIdInterface == nil || !favRoute {
			// if the user_id was not provided, or it is not a favorite route
			if favRoute {
				// if it's a favorites route it means user_id was not provided, so we set the user_id as the token's owner
				claims := middlewares.GetUserClaims(c)
				userId = claims.UserId
			} else {
				userId = 0
			}
		} else {
			// user_id was provided
			userId = userIdInterface.(uint)
		}
		if favRoute {
			orderBy = constants.LikedAt
			isNsfw = constants.Null
		}

		if err := QueryParamsBinder(
			favRoute,
			c,
			&isNsfw,
			&includedTags,
			&excludedTags,
			&includedFiles,
			&excludedFiles,
			&gif,
			&orderBy,
			&orientation,
			&many,
			&full,
		); err != nil {
			return err
		}
		rows, err := controller.Globals.Database.FetchImages(
			isNsfw,
			includedTags,
			excludedTags,
			includedFiles,
			excludedFiles,
			gif,
			orderBy,
			orientation,
			many,
			full,
			userId,
		)
		if err != nil {
			return err
		}
		if gif == constants.True {
			warning = constants.GifWarning
		}
		jsonResponse := serializers.JsonLike(rows)
		if len(jsonResponse.Images) == 0 {
			return c.JSON(http.StatusNotFound, serializers.JSONError{Detail: constants.NotFoundMessage + warning})
		}
		return c.JSON(http.StatusOK, jsonResponse)
	}
}
