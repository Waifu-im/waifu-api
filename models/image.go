package models

// Image Represent an image to a json-like format
type Image struct {
	Signature     string  `json:"signature"`
	Extension     string  `json:"extension"`
	ImageId       int64   `json:"image_id"`
	Favorites     int64   `json:"favorites"`
	DominantColor string  `json:"dominant_color"`
	Source        *string `json:"source"`
	Artist        *Artist `json:"artist"`
	UploadedAt    string  `json:"uploaded_at"`
	LikedAt       *string `json:"liked_at"`
	IsNsfw        bool    `json:"is_nsfw"`
	Width         int64   `json:"width"`
	Height        int64   `json:"height"`
	ByteSize      int64   `json:"byte_size"`
	Url           string  `json:"url"`
	PreviewUrl    string  `json:"preview_url"`
	Tags          []Tag   `json:"tags"`
}
