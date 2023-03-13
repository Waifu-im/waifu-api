package database

import (
	"database/sql"
	"fmt"
	"github.com/Waifu-im/waifu-api/config"
	"github.com/Waifu-im/waifu-api/models"
)

type Database struct {
	Db            *sql.DB
	configuration config.Configuration
}

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
	Configuration config.Configuration
	Rows          []ImageRow
}

func (ir ImageRows) GetImage(ImageId uint) models.Image {
	var image models.Image
	for _, im := range ir.Rows {
		if im.ImageId == ImageId {
			if image.ImageId == 0 {
				image = models.Image{
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
					[]models.Tag{},
				}
			}
			image.Tags = append(image.Tags, models.Tag{im.TagId, im.Name, im.Description, im.TagIsNsfw})
		}
	}
	return image
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
