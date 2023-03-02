package database

import (
	"database/sql"
	"fmt"
	"github.com/Waifu-im/waifu-api/configuration"
	_ "github.com/lib/pq"
	"log"
)

func InitDatabase(config configuration.Configuration) Database {
	info := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Database.IP,
		config.Database.Port,
		config.Database.User,
		config.Database.Password,
		config.Database.Name,
	)
	db, err := sql.Open("postgres", info)
	if err != nil {
		log.Panicln(err)
	}
	return Database{
		Db:            db,
		configuration: config,
	}
}
