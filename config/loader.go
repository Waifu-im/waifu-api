package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

type DatabaseConfiguration struct {
	IP       string
	Port     string
	User     string
	Password string
	Name     string
}

type RedisConfiguration struct {
	IP       string
	Port     string
	Password string
	DB       int
}

type Configuration struct {
	Port                string
	Database            DatabaseConfiguration
	Redis               RedisConfiguration
	Domain              string
	WebSiteUrl          string
	IPCUrl              string
	CDNUrl              string
	SecretKey           string
	Dsn                 string
	TrustedIP           string
	TrustedIPMask       string
	TempTokensKeyPrefix string
}

func getEnvironmentVariable(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalln(fmt.Sprintf("Missing environment variable : %s", key))
	}
	return value
}

func Load() Configuration {
	redisDB, err := strconv.Atoi(getEnvironmentVariable("REDIS_DB"))
	if err != nil {
		panic(err)
	}
	return Configuration{
		Port: getEnvironmentVariable("PORT"),
		Database: DatabaseConfiguration{
			IP:       getEnvironmentVariable("DATABASE_IP"),
			Port:     getEnvironmentVariable("DATABASE_PORT"),
			User:     getEnvironmentVariable("DATABASE_USER"),
			Password: getEnvironmentVariable("DATABASE_PASSWORD"),
			Name:     getEnvironmentVariable("DATABASE_NAME"),
		},
		Redis: RedisConfiguration{
			IP:       getEnvironmentVariable("REDIS_IP"),
			Port:     getEnvironmentVariable("REDIS_PORT"),
			Password: getEnvironmentVariable("REDIS_PASSWORD"),
			DB:       redisDB,
		},
		Domain:              getEnvironmentVariable("DOMAIN"),
		WebSiteUrl:          getEnvironmentVariable("WEBSITE_URL"),
		IPCUrl:              getEnvironmentVariable("IPC_URL"),
		CDNUrl:              getEnvironmentVariable("CDN_URL"),
		SecretKey:           getEnvironmentVariable("SECRET_KEY"),
		Dsn:                 getEnvironmentVariable("DSN"),
		TrustedIP:           getEnvironmentVariable("TRUSTED_IP"),
		TrustedIPMask:       getEnvironmentVariable("TRUSTED_IP_MASK"),
		TempTokensKeyPrefix: getEnvironmentVariable("TEMP_TOKENS_KEY_PREFIX"),
	}
}
