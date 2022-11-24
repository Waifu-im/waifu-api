package tag

import "github.com/Waifu-im/waifu-api/api"

type Route api.Globals

type TagsJsonResponse struct {
	Versatile []interface{} `json:"versatile"`
	Nsfw      []interface{} `json:"nsfw"`
}
