package report

import (
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"net/http"
)

func (r Route) Report(c echo.Context) error {
	var image struct {
		Id          uint    `json:"image_id"`
		Description *string `json:"description"`
	}
	var userId uint
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, api.JSONError{Detail: "Bad Request, image_id is a required parameter, please format it in a json like format inside the request body."})
	}
	userIdInterface := c.Get("target_user_id")
	if userIdInterface == nil {
		claims := middlewares.GetUserClaims(c)
		userId = claims.UserId
	} else {
		userId = userIdInterface.(uint)
		user, status, err := r.Ipc.GetUser(userId)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return c.JSON(http.StatusBadRequest, api.JSONError{Detail: "Please Provide an existing user id"})
		}
		if status == http.StatusInternalServerError {
			return c.JSON(http.StatusInternalServerError, api.JSONError{Detail: "Something went wrong with the IPC request"})
		}
		if err = r.Database.InsertUser(user); err != nil {
			return err
		}
	}
	info, err := r.Database.Report(userId, image.Id, image.Description)
	if err != nil {
		pqe, ok := err.(*pq.Error)
		if ok {
			if pqe.Code == "23503" {
				return c.JSON(http.StatusBadRequest, api.JSONError{Detail: "The image you provided does not exist."})
			}
		}
		return err
	}
	return c.JSON(200, info)
}
