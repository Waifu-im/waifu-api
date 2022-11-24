package tag

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/labstack/echo/v4"
)

func AddRouter(globals api.Globals, group *echo.Echo) error {
	route := Route(globals)
	group.GET("/tags", route.Tags)
	return nil
}
