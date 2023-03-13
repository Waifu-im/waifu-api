package report

import (
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/Waifu-im/waifu-api/serializers"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"net/http"
)

type Controller struct {
	Globals utils.Globals
}

func (controller Controller) Report(c echo.Context) error {
	var image struct {
		Id          uint    `json:"image_id"`
		Description *string `json:"description"`
	}
	var userId uint
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: "Bad Request, image_id is a required parameter, please format it in a json like format inside the request body."})
	}
	userIdInterface := c.Get("target_user_id")
	if userIdInterface == nil {
		claims := middlewares.GetUserClaims(c)
		userId = claims.UserId
	} else {
		userId = userIdInterface.(uint)
		user, status, err := controller.Globals.Ipc.GetUser(userId)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: "Please Provide an existing user id"})
		}
		if status == http.StatusInternalServerError {
			return c.JSON(http.StatusInternalServerError, serializers.JSONError{Detail: "Something went wrong with the IPC request"})
		}
		if err = controller.Globals.Database.InsertUser(user); err != nil {
			return err
		}
	}
	info, err := controller.Globals.Database.Report(userId, image.Id, image.Description)
	if err != nil {
		pqe, ok := err.(*pq.Error)
		if ok {
			if pqe.Code == "23503" {
				return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: "The image you provided does not exist."})
			}
		}
		return err
	}
	return c.JSON(200, info)
}
