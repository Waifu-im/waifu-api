package image

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func AddRouter(globals api.Globals, app *echo.Echo) error {
	route := Route{Globals: globals}
	//using a group for all the commons middleware would have been better but there is some issue with groups.
	//404 instead of 405 when using wrong method, parent group middlewares not triggered on children etc...
	app.GET(
		"/search",
		route.RouteSelector(false),
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"admin"}, middlewares.BoolParamsSkipper("full", "", true)),
	)

	app.GET(
		"/fav",
		route.RouteSelector(true),
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"view_favorites"}, middlewares.UIntParamsSkipper("user_id", "target_user_id", true)),
	)
	return nil
}
