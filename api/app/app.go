package app

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/routes/fav"
	"github.com/Waifu-im/waifu-api/api/routes/image"
	"github.com/Waifu-im/waifu-api/api/routes/report"
	"github.com/Waifu-im/waifu-api/api/routes/tag"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// CreateApp Create an echo application, add routes and start it
func CreateApp(globals api.Globals) {
	app := echo.New()
	app.HTTPErrorHandler = customHTTPErrorHandler
	app.Pre(middleware.RemoveTrailingSlash())
	_ = image.AddRouter(globals, app)
	_ = tag.AddRouter(globals, app)
	_ = fav.AddRouter(globals, app)
	_ = report.AddRouter(globals, app)
	app.Logger.Fatal(app.Start(":" + globals.Configuration.Port))
}
