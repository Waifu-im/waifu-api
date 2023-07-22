package main

import (
	"encoding/json"
	"fmt"
	"github.com/Waifu-im/waifu-api/api/routes"
	"github.com/Waifu-im/waifu-api/api/utils"
	_ "github.com/Waifu-im/waifu-api/docs"
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/net/context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	if err := sentry.Init(sentry.ClientOptions{
		Dsn: globals.Config.Dsn,
	}); err != nil {
		fmt.Printf("Sentry initialization failed: %v\n", err)
	}
	e := echo.New()
	e.HTTPErrorHandler = utils.DefaultHTTPErrorHandler
	e.IPExtractor = echo.ExtractIPFromXFFHeader(
		echo.TrustIPRange(
			&net.IPNet{
				IP:   net.IP(globals.Config.TrustedIP),
				Mask: net.IPMask(globals.Config.TrustedIPMask),
			},
		),
	)
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
	e.Use(sentryecho.New(sentryecho.Options{}))
	e.Use(middleware.Logger())
	e.Use(middleware.BodyDumpWithConfig(middleware.BodyDumpConfig{
		Handler: func(c echo.Context, reqBody, resBody []byte) {
			var execTime int64
			var version string
			if execTimeInterface := c.Get("search_query_exec_time"); execTimeInterface != nil {
				execTime = execTimeInterface.(int64)
			}
			rawVersion := c.Request().Header.Get("Version")
			if rawVersion != "" {
				max := len(rawVersion)
				// Check if string length is > than 20 if yes we set 20 to the max
				if len(rawVersion) > 20 {
					max = 20
				}
				version = rawVersion[0:max]
			}
			if hub := sentryecho.GetHubFromContext(c); hub != nil {
				hub.WithScope(func(scope *sentry.Scope) {
					scope.SetFingerprint([]string{c.Request().Header.Get("X-Request-Id")})
					scope.SetLevel(sentry.LevelInfo)
					scope.SetTag("level", string(sentry.LevelInfo))
					scope.SetTag("version", version)
					scope.SetTag("status_code", strconv.Itoa(c.Response().Status))
					scope.SetTag("ip_address", c.RealIP())
					scope.SetTag("request_id", c.Request().Header.Get("X-Request-Id"))
					if execTime != 0 {
						scope.SetContext("Database", map[string]interface{}{
							"Query Execution Time": strconv.FormatInt(execTime, 10) + "ms",
						})
					}
					rawJson, _ := json.Marshal(string(resBody))

					cleanJson := strings.Replace(string(rawJson), `\n`, ``, -1)
					cleanJson = strings.Replace(cleanJson, `\`, ``, -1)
					cleanJson = strings.Replace(cleanJson, `"`, `'`, -1)

					scope.SetContext("Response", map[string]interface{}{
						"Status code":   c.Response().Status,
						"Json Response": cleanJson,
					})
					fmt.Println(c.Request().URL.Path)
					hub.CaptureMessage("REQ - " + c.Request().URL.Path)
				})
			}
		}}))
	//jwtRoutes := e.Group("", middlewares.TokenVerification(globals, theSkipper))
	// The bug regarding group will probably be fixed in the next echo versions (fix has been merged https://github.com/labstack/echo/issues/1981)
	// edit: well the new version has been released, but it's still not working.
	// It also creates new bugs on the tags route if you use the wrong method so this will stay as is for now
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
