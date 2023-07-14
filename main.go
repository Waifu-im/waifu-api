package main

import (
	"github.com/Waifu-im/waifu-api/api/routes"
	"github.com/Waifu-im/waifu-api/api/utils"
	_ "github.com/Waifu-im/waifu-api/docs"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const shutdownTimeout = 10 * time.Second

// @title    Waifu.im
// @version  5.2.3
// @description.markdown
// @termsOfService  https://www.waifu.im/terms-of-service

// @contact.name   Contact
// @contact.url    http://www.waifu.im/contact
// @contact.email  contact@waifu.im

// @license.name  MPL 2.0
// @license.url   https://www.mozilla.org/en-US/MPL/2.0/

// @host      api.waifu.im
// @BasePath  /

// @securityDefinitions.apikey  ApiKeyAuth
// @in                          header
// @name                        Authorization
// @tokenUrl                    https://www.waifu.im/dashboard
func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	globals := utils.InitGlobals()
	e := echo.New()
	e.HTTPErrorHandler = utils.CustomHTTPErrorHandler
	e.Pre(middleware.RemoveTrailingSlash())
	/*
		Already set with nginx

		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowCredentials: true,
			AllowOrigins:     []string{"*"},
			AllowMethods: []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPut,
				http.MethodPatch,
				http.MethodPost,
				http.MethodDelete,
				http.MethodOptions,
			},
			AllowHeaders: []string{"Accept-Version", "Authorization", "Content-Type"},
		}))
	*/
	// Using default logger
	e.Use(middleware.Logger())
	// Adding a custom one for the database
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogHost:      true,
		LogURI:       true,
		LogUserAgent: true,
		LogRemoteIP:  true,
		LogHeaders:   []string{"Version"},
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			if v.Status == http.StatusOK || v.Status == http.StatusNoContent {
				var execTime int64
				var version string
				userId := utils.GetUser(c).Id
				if execTimeInterface := c.Get("search_query_exec_time"); execTimeInterface != nil {
					execTime = execTimeInterface.(int64)
				}
				if len(v.Headers["Version"]) > 0 {
					max := len(v.Headers["Version"][0])
					// Check if string length is > than 20 if yes we set 20 to the max
					if len(v.Headers["Version"][0]) > 20 {
						max = 20
					}
					version = v.Headers["Version"][0][0:max]
				}
				go globals.Database.LogRequest(v.RemoteIP, "https://"+v.Host+v.URI, v.UserAgent, userId, version, execTime)
			}
			return nil
		},
	}))
	//jwtRoutes := e.Group("", middlewares.TokenVerification(globals, theSkipper))
	// The bug regarding group will probably be fixed in the next echo versions (fix has been merged https://github.com/labstack/echo/issues/1981)
	_ = routes.AddImageRouter(globals, e)
	_ = routes.AddFavManagementRouter(globals, e)
	_ = routes.AddReportRouter(globals, e)
	_ = routes.AddTagRouter(globals, e)
	go func() {
		if err := e.Start(":" + globals.Config.Port); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal(err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}
