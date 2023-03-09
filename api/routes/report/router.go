package report

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

func AddRouter(globals api.Globals, app *echo.Echo) error {
	route := Route(globals)
	app.POST(
		"/report",
		route.Report,
		echojwt.WithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"report"}, middlewares.SkipOrSetUser(true)),
	)
	return nil
}
