package utils

import (
	"github.com/Waifu-im/waifu-api/config"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/Waifu-im/waifu-api/ipc"
)

type Globals struct {
	Database database.Database
	Config   config.Configuration
	Ipc      ipc.IPC
}

func InitGlobals() Globals {
	configuration := config.Load()
	globals := Globals{
		Database: database.InitDatabase(configuration),
		Config:   configuration,
		Ipc:      ipc.IPC{BaseUrl: configuration.IPCUrl},
	}
	return globals
}
