package models

// Tag Represent a tag to a json-like format
type Tag struct {
	TagId       uint   `field:"tag_id" json:"tag_id"`
	Name        string `field:"name" json:"name"`
	Description string `field:"description" json:"description"`
	IsNsfw      bool   `field:"is_nsfw" json:"is_nsfw"`
}
