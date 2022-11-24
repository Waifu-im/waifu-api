package tag

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

func (r Route) Tags(c echo.Context) error {
	var full bool
	mapping := make(map[bool][]interface{})
	err := echo.QueryParamsBinder(c).Bool("full", &full).BindError()
	if err != nil {
		return err
	}
	tags, err := r.Database.GetTags()
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
	return c.JSON(http.StatusOK, TagsJsonResponse{Versatile: mapping[false], Nsfw: mapping[true]})
}
