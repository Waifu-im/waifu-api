package fav

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

func AddRouter(globals api.Globals, app *echo.Echo) error {
	route := Route(globals)
	app.POST(
		"/fav/insert",
		route.Insert,
		echojwt.WithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favorites"}, middlewares.SkipOrSetUser(true)),
	)
	app.DELETE(
		"/fav/delete",
		route.Delete,
		echojwt.WithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favorites"}, middlewares.SkipOrSetUser(true)),
	)
	app.POST(
		"/fav/toggle",
		route.Toggle,
		echojwt.WithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"manage_favorites"}, middlewares.SkipOrSetUser(true)),
	)
	return nil
}
