package fav_management

import (
	"database/sql"
	"errors"
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

// Insert inserts an image to the user's favorites.
// It expects the image data in the request body.
// Returns a JSON response indicating the state of the operation.
// @Summary      Insert an image to favorites
// @Description  Inserts an image to the user's favorites.
// @Tags         Favorites
// @Accept       json
// @Produce      json
// @Param        image    body  serializers.Image  true   "Image data"
// @Param        user_id  body  serializers.User   false  "User ID"
// @Security     ApiKeyAuth
// @Success      200      {object}  serializers.FavState  "INSERTED"
// @Failure      default  {object}  serializers.JSONError
// @Router       /fav/insert [post]
func (controller Controller) Insert(c echo.Context) error {
	var image serializers.Image
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: constants.ImageIdMissing})
	}
	userId := utils.GetUser(c).Id
	userIdInterface := c.Get("target_user_id")
	if userIdInterface != nil {
		userId = userIdInterface.(int64)
		user, status, err := controller.Globals.Ipc.GetUser(userId)
		if err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return c.JSON(http.StatusNotFound, serializers.JSONError{Detail: constants.UserIdNotFoundError})
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
		var pqe *pq.Error
		if errors.As(err, &pqe) {
			if pqe.Code == "23503" {
				return c.JSON(http.StatusNotFound, serializers.JSONError{Detail: "The image you provided does not exist."})
			}
			if pqe.Code == "23505" {
				return c.JSON(http.StatusConflict, serializers.JSONError{Detail: "The image you provided is already in the user favorites, consider using /fav/toggle instead."})
			}
		}
		return err
	}
	return c.JSON(
		200,
		serializers.FavState{State: "INSERTED"},
	)
}

// Delete removes an image from the user's favorites.
// It expects the image data in the request body.
// Returns a JSON response indicating the state of the operation.
// @Summary      Delete an image from favorites
// @Description  Removes an image from the user's favorites.
// @Tags         Favorites
// @Accept       json
// @Produce      json
// @Param        image    body  serializers.Image  true   "Image data"
// @Param        user_id  body  serializers.User   false  "User ID"
// @Security     ApiKeyAuth
// @Success      200      {object}  serializers.FavState  "DELETED"
// @Failure      default  {object}  serializers.JSONError
// @Router       /fav/delete [delete]
func (controller Controller) Delete(c echo.Context) error {
	var image serializers.Image
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: constants.ImageIdMissing})
	}
	userId := utils.GetUser(c).Id
	userIdInterface := c.Get("target_user_id")
	if userIdInterface != nil {
		userId = userIdInterface.(int64)
	}
	if err := controller.Globals.Database.DeleteImageFromFav(userId, image.Id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.JSON(http.StatusNotFound, serializers.JSONError{Detail: "The image you provided do not exist or it is not in the user favorites."})
		}
		return err
	}
	return c.JSON(
		200,
		serializers.FavState{State: "DELETED"},
	)
}

// Toggle toggles an image in the user's favorites.
// It expects the image data in the request body.
// Returns a JSON response indicating the state of the operation.
// @Summary      Toggle an image in favorites
// @Description  Toggles an image in the user's favorites.
// @Tags         Favorites
// @Accept       json
// @Produce      json
// @Param        image    body  serializers.Image  true   "Image data"
// @Param        user_id  body  serializers.User   false  "User ID"
// @Security     ApiKeyAuth
// @Success      200      {object}  serializers.FavState
// @Failure      default  {object}  serializers.JSONError
// @Router       /fav/toggle [post]
func (controller Controller) Toggle(c echo.Context) error {
	var image serializers.Image
	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: constants.ImageIdMissing})
	}
	userId := utils.GetUser(c).Id
	userIdInterface := c.Get("target_user_id")
	if userIdInterface != nil {
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
		var pqe *pq.Error
		if errors.As(err, &pqe) {
			if pqe.Code == "23503" {
				return c.JSON(http.StatusNotFound, serializers.JSONError{Detail: "The image you provided does not exist."})
			}
		}
		return err
	}
	return c.JSON(
		200,
		serializers.FavState{State: state},
	)
}
