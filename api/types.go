package api

import (
	"github.com/Waifu-im/waifu-api/configuration"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/Waifu-im/waifu-api/ipc"
	"github.com/labstack/echo/v4/middleware"
)

type Globals struct {
	Database      database.Database
	Configuration configuration.Configuration
	Ipc           ipc.IPC
	JWTConfig     middleware.JWTConfig
}

type JSONError struct {
	Detail string `json:"detail"`
}
