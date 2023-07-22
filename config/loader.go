package config

import (
	"fmt"
	"log"
	"os"
)

type DatabaseConfiguration struct {
	IP       string
	Port     string
	User     string
	Password string
	Name     string
}

type Configuration struct {
	Port       string
	Database   DatabaseConfiguration
	Domain     string
	WebSiteUrl string
	IPCUrl     string
	CDNUrl     string
	SecretKey  string
	Dsn        string
}

func getEnvironmentVariable(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalln(fmt.Sprintf("Missing environment variable : %s", key))
	}
	return value
}

func Load() Configuration {
	return Configuration{
		Port: getEnvironmentVariable("PORT"),
		Database: DatabaseConfiguration{
			IP:       getEnvironmentVariable("DATABASE_IP"),
			Port:     getEnvironmentVariable("DATABASE_PORT"),
			User:     getEnvironmentVariable("DATABASE_USER"),
			Password: getEnvironmentVariable("DATABASE_PASSWORD"),
			Name:     getEnvironmentVariable("DATABASE_NAME"),
		},
		Domain:     getEnvironmentVariable("DOMAIN"),
		WebSiteUrl: getEnvironmentVariable("WEBSITE_URL"),
		IPCUrl:     getEnvironmentVariable("IPC_URL"),
		CDNUrl:     getEnvironmentVariable("CDN_URL"),
		SecretKey:  getEnvironmentVariable("SECRET_KEY"),
		Dsn:        getEnvironmentVariable("DSN"),
	}
}
