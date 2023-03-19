package routes

import (
	"github.com/Waifu-im/waifu-api/api/controllers/report"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/labstack/echo/v4"
)

func AddReportRouter(globals utils.Globals, app *echo.Echo) error {
	controller := report.Controller{Globals: globals}
	app.POST(
		"/report",
		controller.Report,
		middlewares.TokenVerification(globals, nil),
		middlewares.PermissionsVerification(globals, []string{"report"}, middlewares.SkipOrSetUser(true)),
	)
	return nil
}
