package fav

import (
	"database/sql"
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/api/middlewares"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"net/http"
)

func (r Route) Insert(c echo.Context) error {
	var image Image
	var userId uint
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, api.JSONError{Detail: ImageIdMissing})
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
			return c.JSON(http.StatusBadRequest, api.JSONError{Detail: UserIdNotFoundError})
		}
		if status == http.StatusInternalServerError {
			return c.JSON(http.StatusInternalServerError, api.JSONError{Detail: IPCError})
		}
		err = r.Database.InsertUser(user)
		if err != nil {
			return err
		}
	}
	if err := r.Database.InsertImageToFav(userId, image.Id); err != nil {
		if pqe, ok := err.(*pq.Error); ok {
			if pqe.Code == "23503" {
				return c.JSON(http.StatusBadRequest, api.JSONError{Detail: "The image you provided does not exist."})
			}
			if pqe.Code == "23505" {
				return c.JSON(http.StatusBadRequest, api.JSONError{Detail: "The image you provided is already in the user favourites, consider using /fav/toggle instead."})
			}
		}
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (r Route) Delete(c echo.Context) error {
	var image Image
	var userId uint
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, api.JSONError{Detail: ImageIdMissing})
	}
	userIdInterface := c.Get("target_user_id")
	if userIdInterface == nil {
		claims := middlewares.GetUserClaims(c)
		userId = claims.UserId
	} else {
		userId = userIdInterface.(uint)
	}
	if err := r.Database.DeleteImageFromFav(userId, image.Id); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusBadRequest, api.JSONError{Detail: "The image you provided do not exist or it is not in the user favourites."})
		}
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (r Route) Toggle(c echo.Context) error {
	var image Image
	var userId uint
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, api.JSONError{Detail: ImageIdMissing})
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
			return c.JSON(http.StatusBadRequest, api.JSONError{Detail: UserIdNotFoundError})
		}
		if status == http.StatusInternalServerError {
			return c.JSON(http.StatusInternalServerError, api.JSONError{Detail: IPCError})
		}
		if err := r.Database.InsertUser(user); err != nil {
			return err
		}
	}
	status, err := r.Database.ToggleImageInFav(userId, image.Id)
	if err != nil {
		return err
	}
	return c.JSON(
		200,
		struct {
			Status string `json:"status"`
		}{status},
	)
}
