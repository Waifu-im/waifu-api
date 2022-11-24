package fav

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func AddRouter(globals api.Globals, app *echo.Echo) error {
	route := Route(globals)
	// No need to add the JWT and the token Verification middleware since it has been added previously in image
	app.POST(
		"/fav/insert",
		route.Insert,
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favourites"}, middlewares.SkipOrSetUser(false)),
	)
	app.DELETE(
		"/fav/delete",
		route.Delete,
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favourites"}, middlewares.SkipOrSetUser(false)),
	)
	app.POST(
		"/fav/toggle",
		route.Toggle,
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favourites"}, middlewares.SkipOrSetUser(false)),
	)
	return nil
}
