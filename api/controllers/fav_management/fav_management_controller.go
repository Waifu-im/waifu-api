package fav_management

import (
	"database/sql"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/Waifu-im/waifu-api/serializers"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"net/http"
)

type Controller struct {
	Globals utils.Globals
}

func (controller Controller) Insert(c echo.Context) error {
	var image Image
	var userId int64
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: constants.ImageIdMissing})
	}
	userIdInterface := c.Get("target_user_id")
	if userIdInterface == nil {
		claims := middlewares.GetUserClaims(c)
		userId = claims.UserId
	} else {
		userId = userIdInterface.(int64)
		user, status, err := controller.Globals.Ipc.GetUser(userId)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: constants.UserIdNotFoundError})
		}
		if status == http.StatusInternalServerError {
			return c.JSON(http.StatusInternalServerError, serializers.JSONError{Detail: constants.IPCError})
		}
		err = controller.Globals.Database.InsertUser(user)
		if err != nil {
			return err
		}
	}
	if err := controller.Globals.Database.InsertImageToFav(userId, image.Id); err != nil {
		if pqe, ok := err.(*pq.Error); ok {
			if pqe.Code == "23503" {
				return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: "The image you provided does not exist."})
			}
			if pqe.Code == "23505" {
				return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: "The image you provided is already in the user favorites, consider using /fav/toggle instead."})
			}
		}
		return err
	}
	return c.JSON(
		200,
		struct {
			State string `json:"state"`
		}{"INSERTED"},
	)
}

func (controller Controller) Delete(c echo.Context) error {
	var image Image
	var userId int64
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: constants.ImageIdMissing})
	}
	userIdInterface := c.Get("target_user_id")
	if userIdInterface == nil {
		claims := middlewares.GetUserClaims(c)
		userId = claims.UserId
	} else {
		userId = userIdInterface.(int64)
	}
	if err := controller.Globals.Database.DeleteImageFromFav(userId, image.Id); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: "The image you provided do not exist or it is not in the user favorites."})
		}
		return err
	}
	return c.JSON(
		200,
		struct {
			State string `json:"state"`
		}{"DELETED"},
	)
}

func (controller Controller) Toggle(c echo.Context) error {
	var image Image
	var userId int64
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: constants.ImageIdMissing})
	}
	userIdInterface := c.Get("target_user_id")
	if userIdInterface == nil {
		claims := middlewares.GetUserClaims(c)
		userId = claims.UserId
	} else {
		userId = userIdInterface.(int64)
		user, status, err := controller.Globals.Ipc.GetUser(userId)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: constants.UserIdNotFoundError})
		}
		if status == http.StatusInternalServerError {
			return c.JSON(http.StatusInternalServerError, serializers.JSONError{Detail: constants.IPCError})
		}
		if err := controller.Globals.Database.InsertUser(user); err != nil {
			return err
		}
	}
	state, err := controller.Globals.Database.ToggleImageInFav(userId, image.Id)
	if err != nil {
		if pqe, ok := err.(*pq.Error); ok {
			if pqe.Code == "23503" {
				return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: "The image you provided does not exist."})
			}
		}
		return err
	}
	return c.JSON(
		200,
		struct {
			State string `json:"state"`
		}{state},
	)
}
