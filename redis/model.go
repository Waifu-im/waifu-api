package redis

import (
	"github.com/Waifu-im/waifu-api/config"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client        *redis.Client
	configuration config.Configuration
}
