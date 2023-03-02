package fav

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func AddRouter(globals api.Globals, app *echo.Echo) error {
	route := Route(globals)
	app.POST(
		"/fav/insert",
		route.Insert,
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favourites"}, middlewares.SkipOrSetUser(true)),
	)
	app.DELETE(
		"/fav/delete",
		route.Delete,
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favourites"}, middlewares.SkipOrSetUser(true)),
	)
	app.POST(
		"/fav/toggle",
		route.Toggle,
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favourites"}, middlewares.SkipOrSetUser(true)),
	)
	return nil
}
