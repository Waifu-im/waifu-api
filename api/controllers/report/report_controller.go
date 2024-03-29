package report

import (
	"github.com/Waifu-im/waifu-api/api/utils"
	"github.com/Waifu-im/waifu-api/serializers"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"net/http"
)

type Controller struct {
	Globals utils.Globals
}

// Report
// @Summary      Report an image
// @Description  Report an image with a given image ID and description
// @Tags         Report
// @Accept       json
// @Produce      json
// @Param        image  body  serializers.ReportImage  true  "Image Data"
// @Security     ApiKeyAuth
// @Success      200      {object}  database.ReportRes  "Returns the report information"
// @Failure      default  {object}  serializers.JSONError
// @Router       /report [post]
func (controller Controller) Report(c echo.Context) error {
	var image serializers.ReportImage

	if err := c.Bind(&image); err != nil {
		return err
	}
	if image.Id == 0 {
		return c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: "Bad Request, image_id is a required parameter, please format it in a json like format inside the request body."})
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
