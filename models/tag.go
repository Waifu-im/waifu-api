package models

// Tag Represent a tag to a json-like format
type Tag struct {
	TagId       int64  `field:"tag_id" json:"tag_id" example:"12"`
	Name        string `field:"name" json:"name" example:"waifu"`
	Description string `field:"description" json:"description" example:"A female anime/manga character."`
	IsNsfw      bool   `field:"is_nsfw" json:"is_nsfw" example:"false"`
}
