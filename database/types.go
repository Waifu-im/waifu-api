package database

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/Waifu-im/waifu-api/configuration"
	"golang.org/x/exp/slices"
)

// ImageRow Represent a row when retrieving images (likedAt may not be present)
type ImageRow struct {
	Signature     string  `field:"signature" json:"signature"`
	Extension     string  `field:"extension" json:"extension"`
	ImageId       uint    `field:"image_id" json:"image_id"`
	Favorites     uint    `field:"favorites" json:"favorites"`
	DominantColor string  `field:"dominant_color" json:"dominant_color"`
	Source        *string `field:"source" json:"source"`
	UploadedAt    string  `field:"uploaded_at" json:"uploaded_at"`
	LikedAt       *string `field:"liked_at" json:"liked_at"`
	IsNsfw        bool    `field:"is_nsfw" json:"is_nsfw"`
	Width         uint    `field:"width" json:"width"`
	Height        uint    `field:"height" json:"height"`
	TagId         uint    `field:"tag_id" json:"tag_id"`
	Name          string  `field:"name" json:"name"`
	Description   string  `field:"description" json:"description"`
	TagIsNsfw     bool    `field:"tag_is_nsfw" json:"tag_is_nsfw"`
}

// ImageRows Represent multiple rows
type ImageRows struct {
	Configuration configuration.Configuration
	Rows          []ImageRow
}

// Tag Represent a tag to a json-like format
type Tag struct {
	TagId       uint   `field:"tag_id" json:"tag_id"`
	Name        string `field:"name" json:"name"`
	Description string `field:"description" json:"description"`
	IsNsfw      bool   `field:"is_nsfw" json:"is_nsfw"`
}

// Image Represent an image to a json-like format
type Image struct {
	Signature     string  `json:"signature"`
	Extension     string  `json:"extension"`
	ImageId       uint    `json:"image_id"`
	Favorites     uint    `json:"favorites"`
	DominantColor string  `json:"dominant_color"`
	Source        *string `json:"source"`
	UploadedAt    string  `json:"uploaded_at"`
	LikedAt       *string `json:"liked_at"`
	IsNsfw        bool    `json:"is_nsfw"`
	Width         uint    `json:"width"`
	Height        uint    `json:"height"`
	Url           string  `json:"url"`
	PreviewUrl    string  `json:"preview_url"`
	Tags          []Tag   `json:"tags"`
}

func (ir ImageRows) GetImage(ImageId uint) Image {
	var image Image
	for _, im := range ir.Rows {
		if im.ImageId == ImageId {
			if image.ImageId == 0 {
				image = Image{
					im.Signature,
					im.Extension,
					im.ImageId,
					im.Favorites,
					im.DominantColor,
					im.Source,
					im.UploadedAt,
					im.LikedAt,
					im.IsNsfw,
					im.Width,
					im.Height,
					ir.Configuration.CDNUrl + "/" + fmt.Sprintf("%v%v", im.ImageId, im.Extension),
					ir.Configuration.WebSiteUrl + "/preview/" + fmt.Sprintf("%v", im.ImageId) + "/",
					[]Tag{},
				}
			}
			image.Tags = append(image.Tags, Tag{im.TagId, im.Name, im.Description, im.TagIsNsfw})
		}
	}
	return image
}

// ImagesJsonResponse Represent A JSON Response Content just before it is Serialized
type ImagesJsonResponse struct {
	Images []Image `json:"images"`
}

func (ir ImageRows) JsonLike() ImagesJsonResponse {
	var processedIds []uint
	var imageRes ImagesJsonResponse

	for _, im := range ir.Rows {
		if !slices.Contains(processedIds, im.ImageId) {
			imageRes.Images = append(imageRes.Images, ir.GetImage(im.ImageId))
			processedIds = append(processedIds, im.ImageId)
		}
	}
	return imageRes
}

// PermissionsInformation a struct used to retrieve and a row from the database
type PermissionsInformation struct {
	UserId   uint
	TargetId *uint
	Position int
	Name     string
}

type ReportRes struct {
	ImageId     uint    `json:"image_id" field:"image_id" `
	AuthorId    uint    `json:"author_id" field:"author_id"`
	Description *string `json:"description" field:"description"`
	Existed     bool    `json:"existed" field:"existed"`
}

type Database struct {
	Db            *sql.DB
	configuration configuration.Configuration
}

var BlacklistedError = errors.New("you are not allowed to do this action, you have been blacklisted")
