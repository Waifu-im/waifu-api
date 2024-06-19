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
	Signature        string  `field:"signature" json:"signature"`
	Extension        string  `field:"extension" json:"extension"`
	ImageId          int64   `field:"image_id" json:"image_id"`
	Favorites        int64   `field:"favorites" json:"favorites"`
	DominantColor    string  `field:"dominant_color" json:"dominant_color"`
	Source           *string `field:"source" json:"source"`
	UploadedAt       string  `field:"uploaded_at" json:"uploaded_at"`
	LikedAt          *string `field:"liked_at" json:"liked_at"`
	IsNsfw           bool    `field:"is_nsfw" json:"is_nsfw"`
	Width            int64   `field:"width" json:"width"`
	Height           int64   `field:"height" json:"height"`
	ByteSize         int64   `field:"byte_size" json:"byte_size"`
	TagId            int64   `field:"tag_id" json:"tag_id"`
	TagName          string  `field:"tag_name" json:"tag_name"`
	TagDescription   string  `field:"tag_description" json:"tag_description"`
	TagIsNsfw        bool    `field:"tag_is_nsfw" json:"tag_is_nsfw"`
	ArtistId         *int64  `field:"artist_id" json:"artist_id"`
	ArtistName       *string `field:"artist_name" json:"artist_name"`
	ArtistPatreon    *string `field:"artist_patreon" json:"artist_patreon"`
	ArtistTwitter    *string `field:"artist_twitter" json:"artist_twitter"`
	ArtistDeviantArt *string `field:"artist_deviant_art" json:"artist_deviant_art"`
	ArtistPixiv      *string `field:"artist_pixiv" json:"artist_pixiv"`
}

// ImageRows Represent multiple rows
type ImageRows struct {
	Configuration config.Configuration
	Rows          []ImageRow
}

func (ir ImageRows) GetImage(ImageId int64) models.Image {
	var image models.Image
	for _, im := range ir.Rows {
		if im.ImageId == ImageId {
			if image.ImageId == 0 {
				image = models.Image{
					Signature:     im.Signature,
					Extension:     im.Extension,
					ImageId:       im.ImageId,
					Favorites:     im.Favorites,
					DominantColor: im.DominantColor,
					Source:        im.Source,
					UploadedAt:    im.UploadedAt,
					LikedAt:       im.LikedAt,
					IsNsfw:        im.IsNsfw,
					Width:         im.Width,
					Height:        im.Height,
					ByteSize:      im.ByteSize,
					Url:           ir.Configuration.CDNUrl + "/" + fmt.Sprintf("%v%v", im.ImageId, im.Extension),
					PreviewUrl:    ir.Configuration.WebSiteUrl + "/preview/" + fmt.Sprintf("%v", im.ImageId) + "/",
					Tags:          []models.Tag{},
				}
			}
			if im.ArtistId != nil {
				image.Artist = &models.Artist{
					ArtistId:   *im.ArtistId,
					Name:       *im.ArtistName,
					Patreon:    im.ArtistPatreon,
					Twitter:    im.ArtistTwitter,
					DeviantArt: im.ArtistDeviantArt,
					Pixiv:      im.ArtistPixiv,
				}
			}
			image.Tags = append(
				image.Tags,
				models.Tag{
					TagId:       im.TagId,
					Name:        im.TagName,
					Description: im.TagDescription,
					IsNsfw:      im.TagIsNsfw,
				},
			)
		}
	}
	return image
}

// PermissionsInformation a struct used to retrieve and a row from the database
type PermissionsInformation struct {
	UserId   int64
	TargetId *int64
	Position int
	Name     string
}

type ReportRes struct {
	ImageId     int64   `json:"image_id" field:"image_id" `
	AuthorId    string  `json:"author_id" field:"author_id"`
	Description *string `json:"description" field:"description"`
	Existed     bool    `json:"existed" field:"existed"`
}
