package models

// Image Represent an image to a json-like format
type Image struct {
	Signature     string  `json:"signature" example:"58e6f0372364abda"`
	Extension     string  `json:"extension" example:".png"`
	ImageId       int64   `json:"image_id" example:"8108"`
	Favorites     int64   `json:"favorites" example:"1"`
	DominantColor string  `json:"dominant_color" example:"#bbb7b2"`
	Source        *string `json:"source" example:"https://www.patreon.com/posts/persephone-78224476"`
	Artist        *Artist `json:"artist"`
	UploadedAt    string  `json:"uploaded_at" example:"2023-05-03T18:40:04.381354+02:00"`
	Uploader      *int64  `json:"uploader" example:"953746972518275160"`
	LikedAt       *string `json:"liked_at"`
	IsNsfw        bool    `json:"is_nsfw" example:"false"`
	Width         int64   `json:"width" example:"1536"`
	Height        int64   `json:"height" example:"2304"`
	ByteSize      int64   `json:"byte_size" example:"3299586"`
	Url           string  `json:"url" example:"https://cdn.waifu.im/8108.png"`
	PreviewUrl    string  `json:"preview_url" example:"https://www.waifu.im/preview/8108/"`
	Tags          []Tag   `json:"tags"`
}
