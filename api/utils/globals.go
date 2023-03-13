package utils

import (
	"github.com/Waifu-im/waifu-api/config"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/Waifu-im/waifu-api/ipc"
	"github.com/Waifu-im/waifu-api/models"
	"github.com/golang-jwt/jwt/v4"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

type Globals struct {
	Database  database.Database
	Config    config.Configuration
	Ipc       ipc.IPC
	JWTConfig echojwt.Config
}

func InitGlobals() Globals {
	configuration := config.Load()
	return Globals{
		Config:   configuration,
		Database: database.InitDatabase(configuration),
		Ipc:      ipc.IPC{BaseUrl: configuration.IPCUrl},
		JWTConfig: echojwt.Config{
			NewClaimsFunc: func(c echo.Context) jwt.Claims {
				return &models.JwtCustomClaims{}
			},
			SigningKey: []byte(configuration.SecretKey),
			Skipper: func(c echo.Context) bool {
				var full bool
				_ = echo.QueryParamsBinder(c).Bool("full", &full)
				return c.Request().URL.Path == "/search" && !full
			},
		},
	}
}
