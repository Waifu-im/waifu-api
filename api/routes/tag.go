package routes

import (
	"github.com/Waifu-im/waifu-api/api/controllers/tag"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/labstack/echo/v4"
)

func AddTagRouter(globals utils.Globals, app *echo.Echo) error {
	controller := tag.Controller{Globals: globals}
	app.GET("/tags", controller.Tags)
	return nil
}
