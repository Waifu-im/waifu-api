package app

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/routes/fav"
	"github.com/Waifu-im/waifu-api/api/routes/image"
	"github.com/Waifu-im/waifu-api/api/routes/report"
	"github.com/Waifu-im/waifu-api/api/routes/tag"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
)

// CreateApp Create an echo application, add routes and start it
func CreateApp(globals api.Globals) {
	app := echo.New()
	app.HTTPErrorHandler = customHTTPErrorHandler
	app.Pre(middleware.RemoveTrailingSlash())
	app.Use(middleware.CORS())
	// Using default logger
	app.Use(middleware.Logger())
	// Adding a custom one for the database
	app.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogHost:      true,
		LogURI:       true,
		LogUserAgent: true,
		LogRemoteIP:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Status == http.StatusOK || v.Status == http.StatusNoContent {
				var userId uint
				claims := middlewares.GetUserClaims(c)
				if claims != nil {
					userId = claims.UserId
				}
				go globals.Database.LogRequest(v.RemoteIP, "https://"+v.Host+v.URI, v.UserAgent, userId)
			}
			return nil
		}}))
	_ = image.AddRouter(globals, app)
	_ = tag.AddRouter(globals, app)
	_ = fav.AddRouter(globals, app)
	_ = report.AddRouter(globals, app)
	app.Logger.Fatal(app.Start(":" + globals.Configuration.Port))
}
