package routes

import (
	"github.com/Waifu-im/waifu-api/api/controllers/fav_management"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/labstack/echo/v4"
)

func AddFavManagementRouter(globals utils.Globals, app *echo.Echo) error {
	controller := fav_management.Controller{Globals: globals}
	app.POST(
		"/fav/insert",
		controller.Insert,
		middlewares.TokenVerification(globals, nil),
		middlewares.PermissionsVerification(globals, []string{"manage_favorites"}, middlewares.SkipOrSetUser(true)),
	)
	app.DELETE(
		"/fav/delete",
		controller.Delete,
		middlewares.TokenVerification(globals, nil),
		middlewares.PermissionsVerification(globals, []string{"manage_favorites"}, middlewares.SkipOrSetUser(true)),
	)
	app.POST(
		"/fav/toggle",
		controller.Toggle,
		middlewares.TokenVerification(globals, nil),
		middlewares.PermissionsVerification(globals, []string{"manage_favorites"}, middlewares.SkipOrSetUser(true)),
	)
	return nil
}
