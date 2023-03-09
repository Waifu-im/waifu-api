package api

import (
	"github.com/Waifu-im/waifu-api/configuration"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/Waifu-im/waifu-api/ipc"
	"github.com/labstack/echo-jwt/v4"
)

type Globals struct {
	Database      database.Database
	Configuration configuration.Configuration
	Ipc           ipc.IPC
	JWTConfig     echojwt.Config
}

type JSONError struct {
	Detail string `json:"detail"`
}
