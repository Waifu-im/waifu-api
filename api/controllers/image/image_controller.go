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

// Search handles the route selection based on the specified parameters.
// It retrieves images randomly or by tag.
// If no tag filtering is requested, the default tag "waifu" will be included.
//
// @Summary      Search Images
// @Description  Retrieves images randomly or by tag based on the specified search criteria.
// @Tags         Get Images
// @Accept       json
// @Produce      json
// @Param        included_tags   query  []string  false  "Force the API to return images with at least all the provided tags"
// @Param        excluded_tags   query  []string  false  "Force the API to return images without any of the provided tags"
// @Param        included_files  query  []string  false  "Force the API to provide only the specified file IDs or signatures"                                          example(58e6f0372364abda)
// @Param        excluded_files  query  []string  false  "Force the API to not list the specified file IDs or signatures"                                              example(8108)
// @Param        is_nsfw         query  string    false  "Force or exclude lewd files (only works if included_tags only contain versatile tags and no nsfw only tag).  You  can  provide  'null'  to  make  it  be  random." Enums(null, true, false)  default(false)
// @Param        gif             query  bool      false  "Force or prevent the API to return .gif files"
// @Param        order_by        query  string    false  "Ordering criteria"                    Enums(FAVORITES, UPLOADED_AT, LIKED_AT, RANDOM)  default(RANDOM)
// @Param        orientation     query  string    false  "Image orientation"                    Enums(LANDSCAPE, PORTRAIT, RANDOM)               default(RANDOM)
// @Param        many            query  bool      false  "Return an array of 30 files if true"  default(false)
// @Param        full            query  bool      false  "Returns the full result without any limit (admins only)" default(false)
// @Param        width           query  string    false  "Filter images by width (in pixels).                                   Accepted  operators:  <=,  >=,  >,  <,  !=,  =" example(>=2000)
// @Param        height          query  string    false  "Filter images by height (in pixels).                                  Accepted  operators:  <=,  >=,  >,  <,  !=,  =" example(>=2000)
// @Param        byte_size       query  string    false  "Filter images by byte size. Accepted operators: <=, >=, >, <, !=, ="  example(>=2000)
// @Security     ApiKeyAuth
// @Success      200      {object}  serializers.ImagesJsonResponse
// @Failure      default  {object}  serializers.JSONError
// @Router       /search [get]
func (controller Controller) Search() echo.HandlerFunc {
	return controller.RouteSelector(false)
}

// Fav handles the route selection for fetching favorite images.
// It retrieves images based on the user's favorites.
//
// @Summary      Fetch Favorite Images
// @Description  Retrieves images based on the user's favorites.
// @Tags         Favorites
// @Accept       json
// @Produce      json
// @Param        included_tags   query  []string  false  "Force the API to return images with at least all the provided tags"
// @Param        excluded_tags   query  []string  false  "Force the API to return images without any of the provided tags"
// @Param        included_files  query  []string  false  "Force the API to provide only the specified file IDs or signatures"                                          example(58e6f0372364abda)
// @Param        excluded_files  query  []string  false  "Force the API to not list the specified file IDs or signatures"                                              example(8108)
// @Param        is_nsfw         query  string    false  "Force or exclude lewd files (only works if included_tags only contain versatile tags and no nsfw only tag).  You  can  provide  'null'  to  make  it  be  random." Enums(null, true, false)  default(false)
// @Param        gif             query  bool      false  "Force or prevent the API to return .gif files"
// @Param        order_by        query  string    false  "Ordering criteria"                    Enums(FAVORITES, UPLOADED_AT, LIKED_AT, RANDOM)  default(RANDOM)
// @Param        orientation     query  string    false  "Image orientation"                    Enums(LANDSCAPE, PORTRAIT, RANDOM)               default(RANDOM)
// @Param        many            query  bool      false  "Return an array of 30 files if true"  default(false)
// @Param        full            query  bool      false  "Returns the full result without any limit (admins only)" default(false)
// @Param        width           query  string    false  "Filter images by width (in pixels).                                   Accepted  operators:  <=,  >=,  >,  <,  !=,  =" example(>=2000)
// @Param        height          query  string    false  "Filter images by height (in pixels).                                  Accepted  operators:  <=,  >=,  >,  <,  !=,  =" example(>=2000)
// @Param        byte_size       query  string    false  "Filter images by byte size. Accepted operators: <=, >=, >, <, !=, ="  example(>=2000)
// @Param        user_id         query  int64     false  "User ID"
// @Security     ApiKeyAuth
// @Success      200      {object}  serializers.ImagesJsonResponse
// @Failure      default  {object}  serializers.JSONError
// @Router       /fav [get]
func (controller Controller) Fav() echo.HandlerFunc {
	return controller.RouteSelector(true)
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
