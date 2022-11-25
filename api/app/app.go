package app

import (
	"context"
	"fmt"
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/routes/fav"
	"github.com/Waifu-im/waifu-api/api/routes/image"
	"github.com/Waifu-im/waifu-api/api/routes/report"
	"github.com/Waifu-im/waifu-api/api/routes/tag"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"os"
	"os/signal"
	"time"
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
				if claims := middlewares.GetUserClaims(c); claims != nil {
					userId = claims.UserId
				}
				go globals.Database.LogRequest(v.RemoteIP, "https://"+v.Host+v.URI, v.UserAgent, userId)
			}
			return nil
		},
	}))
	_ = image.AddRouter(globals, app)
	_ = tag.AddRouter(globals, app)
	_ = fav.AddRouter(globals, app)
	_ = report.AddRouter(globals, app)
	go func() {
		if err := app.Start(":" + globals.Configuration.Port); err != nil && err != http.ErrServerClosed {
			app.Logger.Fatal("Shutting Down...")
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	fmt.Println("\nGracefully shutting Down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.Shutdown(ctx); err != nil {
		app.Logger.Fatal(err)
	}
	fmt.Println("Shutdown complete")
}
