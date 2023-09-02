package redis

import (
	"github.com/Waifu-im/waifu-api/config"
	"github.com/redis/go-redis/v9"
)

func InitRedis(config config.Configuration) Redis {
	client := redis.NewClient(&redis.Options{
		Addr:     config.Redis.IP + ":" + config.Redis.Port,
		Password: config.Redis.Password,
		DB:       config.Redis.DB,
	})

	return Redis{
		client:        client,
		configuration: config,
	}
}
