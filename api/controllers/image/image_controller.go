package image

import (
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/Waifu-im/waifu-api/serializers"
	"github.com/labstack/echo/v4"
	"net/http"
	"time"
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
		var width string
		var height string
		var byteSize string
		var warning string
		var userId int64

		if favRoute {
			orderBy = constants.LikedAt
			isNsfw = constants.Null
			userId = utils.GetUser(c).Id
			userIdInterface := c.Get("target_user_id")
			if userIdInterface != nil {
				userId = userIdInterface.(int64)
			}
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
			&width,
			&height,
			&byteSize,
		); err != nil {
			return err
		}
		rows, execTime, err := controller.Globals.Database.FetchImages(
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
			width,
			height,
			byteSize,
			userId,
		)
		(c).Set("search_query_exec_time", int64(execTime/time.Millisecond))
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
