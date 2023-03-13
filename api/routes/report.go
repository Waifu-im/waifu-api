package routes

import (
	"github.com/Waifu-im/waifu-api/api/controllers/report"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

func AddReportRouter(globals utils.Globals, app *echo.Echo) error {
	controller := report.Controller{Globals: globals}
	app.POST(
		"/report",
		controller.Report,
		echojwt.WithConfig(globals.JWTConfig),
		middlewares.TokenVerification(globals),
		middlewares.PermissionsVerification(globals, []string{"report"}, middlewares.SkipOrSetUser(true)),
	)
	return nil
}
