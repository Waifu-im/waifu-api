package tag

import (
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/Waifu-im/waifu-api/serializers"
	"github.com/labstack/echo/v4"
	"net/http"
)

type Controller struct {
	Globals utils.Globals
}

func (controller Controller) Tags(c echo.Context) error {
	var full bool
	mapping := make(map[bool][]interface{})
	if err := echo.QueryParamsBinder(c).Bool("full", &full).BindError(); err != nil {
		return err
	}
	tags, err := controller.Globals.Database.GetTags()
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if full {
			mapping[tag.IsNsfw] = append(mapping[tag.IsNsfw], tag)

		} else {
			mapping[tag.IsNsfw] = append(mapping[tag.IsNsfw], tag.Name)
		}
	}
	return c.JSON(http.StatusOK, serializers.TagsJsonResponse{Versatile: mapping[false], Nsfw: mapping[true]})
}
