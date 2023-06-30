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

// Tags godoc
// @Summary      Get tags
// @Description  Get a list of tags
// @Tags         Tags
// @Accept       json
// @Produce      json
// @Param        full     query     boolean  false  "Returns more information about the tags, such as a description."  default(false)  example(true)
// @Success      200      {object}  serializers.TagsJsonResponse
// @Failure      default  {object}  serializers.JSONError
// @Router       /tags [get]
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
