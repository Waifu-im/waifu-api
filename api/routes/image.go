package routes

import (
	"github.com/Waifu-im/waifu-api/api/controllers/image"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

func AddImageRouter(globals utils.Globals, app *echo.Echo) error {
	controller := image.Controller{Globals: globals}
	//using a group for all the commons middleware would have been better but there is some issue with groups.
	//404 instead of 405 when using wrong method, parent group middlewares not triggered on children etc...

	app.GET(
		"/search",
		controller.RouteSelector(false),
		echojwt.WithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"admin"}, middlewares.BoolParamsSkipper("full", "", true)),
	)

	app.GET(
		"/fav",
		controller.RouteSelector(true),
		echojwt.WithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"view_favorites"}, middlewares.Int64ParamsSkipper("user_id", "target_user_id", true)),
	)
	return nil
}
