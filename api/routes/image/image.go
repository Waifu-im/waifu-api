package image

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/labstack/echo/v4"
	"net/http"
)

// RouteSelector Since most of the code is the same for /fav and /search I wrap the handler inside a function
// Then the handler use the 'parent' function favRoot boolean to know if it should add or not the user id to database methods
// The parent function is itself a method of Root so that it can access api package globals variables (api.Globals struct)
func (imr Route) RouteSelector(favRoute bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		var isNsfw = database.False
		var includedTags []string
		var excludedTags []string
		var includedFiles []string
		var excludedFiles []string
		var gif = database.Null
		var orderBy = database.Random
		var orientation = database.Null
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
			orderBy = database.LikedAt
			isNsfw = database.Null
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
		ri, err := imr.Globals.Database.FetchImages(
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
		if gif == database.True {
			warning = GifWarning
		}
		jsonResponse := ri.JsonLike()
		if len(jsonResponse.Images) == 0 {
			return c.JSON(http.StatusNotFound, api.JSONError{Detail: NotFoundMessage + warning})
		}
		return c.JSON(http.StatusOK, jsonResponse)
	}
}
