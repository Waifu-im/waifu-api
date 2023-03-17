package serializers

import (
	"github.com/Waifu-im/waifu-api/database"
	"github.com/Waifu-im/waifu-api/models"
	"golang.org/x/exp/slices"
)

func JsonLike(rows database.ImageRows) ImagesJsonResponse {
	var processedIds []int64
	var imageRes ImagesJsonResponse

	for _, im := range rows.Rows {
		if !slices.Contains(processedIds, im.ImageId) {
			imageRes.Images = append(imageRes.Images, rows.GetImage(im.ImageId))
			processedIds = append(processedIds, im.ImageId)
		}
	}
	return imageRes
}

// ImagesJsonResponse Represent A JSON Response Content just before it is Serialized
type ImagesJsonResponse struct {
	Images []models.Image `json:"images"`
}
