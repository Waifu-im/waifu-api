package utils

import (
	"github.com/Waifu-im/waifu-api/config"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/Waifu-im/waifu-api/ipc"
	"github.com/Waifu-im/waifu-api/redis"
)

type Globals struct {
	Database database.Database
	Redis    redis.Redis
	Config   config.Configuration
	Ipc      ipc.IPC
}

func InitGlobals() Globals {
	configuration := config.Load()
	globals := Globals{
		Database: database.InitDatabase(configuration),
		Redis:    redis.InitRedis(configuration),
		Config:   configuration,
		Ipc:      ipc.IPC{BaseUrl: configuration.IPCUrl},
	}
	return globals
}
