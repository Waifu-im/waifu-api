package report

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func AddRouter(globals api.Globals, app *echo.Echo) error {
	route := Route(globals)
	app.POST(
		"/report",
		route.Report,
		middleware.JWTWithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"report"}, middlewares.SkipOrSetUser(true)),
	)
	return nil
}
