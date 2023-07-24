package main

import (
	"errors"
	"fmt"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/routes"
	"github.com/Waifu-im/waifu-api/api/utils"
	_ "github.com/Waifu-im/waifu-api/docs"
	"github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/context"
	"log"
	"net"
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

	middlewares.Init(globals, e)

	//jwtRoutes := e.Group("", middlewares.TokenVerification(globals, theSkipper))
	// The bug regarding group will probably be fixed in the next echo versions (fix has been merged https://github.com/labstack/echo/issues/1981)
	// edit: well the new version has been released, but it's still not working.
	// It also creates new bugs on the tags route if you use the wrong method so this will stay as is for now
	_ = routes.AddImageRouter(globals, e)
	_ = routes.AddFavManagementRouter(globals, e)
	_ = routes.AddReportRouter(globals, e)
	_ = routes.AddTagRouter(globals, e)

	go func() {
		if err := e.Start(":" + globals.Config.Port); err != nil && !errors.Is(err, http.ErrServerClosed) {
			e.Logger.Fatal(err)
		}
	}()

	defer sentry.Flush(2 * time.Second)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}
