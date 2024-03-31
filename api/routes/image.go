package routes

import (
	"github.com/Waifu-im/waifu-api/api/controllers/image"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/labstack/echo/v4"
)

func AddImageRouter(globals utils.Globals, app *echo.Echo) error {
	controller := image.Controller{Globals: globals}
	//using a group for all the commons middleware would have been better but there is some issue with groups.
	//404 instead of 405 when using wrong method, parent group middlewares not triggered on children etc...

	app.GET(
		"/search",
		controller.Search(),
		middlewares.TokenVerification(
			globals,
			func(c echo.Context) (bool, error) {
				var full bool
				var limit int
				_ = echo.QueryParamsBinder(c).Bool("full", &full)
				_ = echo.QueryParamsBinder(c).Int("limit", &limit)
				// when new release will be out this middleware will be on the whole group with that condition in addition
				// c.Request().URL.Path == "/search"
				skip := !full && limit <= constants.MaxLimit
				return skip, nil
			}),
		middlewares.PermissionsVerification(
			globals, []string{"admin"},
			middlewares.BoolParamsSkipper("full", "", true),
		),
		middlewares.PermissionsVerification(
			globals, []string{"admin"},
			middlewares.LimitParamsSkipper("limit", "", true),
		),
	)
	app.GET(
		"/fav",
		controller.Fav(),
		middlewares.TokenVerification(globals, nil),
		middlewares.PermissionsVerification(globals, []string{"view_favorites"}, middlewares.Int64ParamsSkipper("user_id", "target_user_id", true)),
	)
	return nil
}
