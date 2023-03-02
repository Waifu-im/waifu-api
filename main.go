package main

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/app"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/configuration"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/Waifu-im/waifu-api/ipc"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"log"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatalln("Error loading .env file")
	}
	config := configuration.Load()
	globals := api.Globals{
		Configuration: config,
		Database:      database.InitDatabase(config),
		Ipc:           ipc.IPC{BaseUrl: config.IPCUrl},
		JWTConfig: middleware.JWTConfig{
			Claims:     &middlewares.JwtCustomClaims{},
			SigningKey: []byte(config.SecretKey),
			Skipper: func(c echo.Context) bool {
				var full bool
				_ = echo.QueryParamsBinder(c).Bool("full", &full)
				return c.Request().URL.Path == "/search" && !full
			},
		},
	}
	app.CreateApp(globals)
}
