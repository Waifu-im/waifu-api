package fav

import "github.com/Waifu-im/waifu-api/api"

type Route api.Globals

const UserIdNotFoundError = "Please Provide an existing user id"
const IPCError = "Something went wrong with the IPC request"
const ImageIdMissing = "Bad Request, image_id is a required parameter, please format it in a json like format inside the request body."

type Image struct {
	Id uint `json:"image_id"`
}
