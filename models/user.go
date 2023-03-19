package models

type User struct {
	Id            int64   `field:"id"`
	Name          string  `field:"name"`
	Token         *string `field:"token"`
	IsBlacklisted bool    `field:"is_blacklisted"`
}
