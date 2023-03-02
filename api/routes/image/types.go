package image

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/labstack/echo/v4"
)

type Route struct {
	Globals api.Globals
}

type ImagesContext struct {
	echo.Context
	favRoute bool
}

const NotFoundMessage = "No image found matching the criteria given."
const GifWarning = " Please note that there is not much gif in the database, therefore, the more you filter the search the less gifs you will find."
