package utils

import (
	"errors"
	"fmt"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/Waifu-im/waifu-api/serializers"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"
	"net/http"
)

func DefaultHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	if errors.Is(err, constants.BlacklistedError) {
		_ = c.JSON(http.StatusForbidden, serializers.JSONError{Detail: fmt.Sprintf("%v", err)})
		return
	}
	var httpError *echo.HTTPError
	if errors.As(err, &httpError) {
		// echo JWT middleware package doesn't directly return the error they defined so the error cannot be directly compared
		_ = c.JSON(httpError.Code, serializers.JSONError{Detail: fmt.Sprintf("%v", httpError.Message)})
		return
	}
	var be *echo.BindingError
	if errors.As(err, &be) {
		_ = c.JSON(http.StatusBadRequest, serializers.JSONError{Detail: fmt.Sprintf("Bad Request, error on %v, %v", be.Field, be.Message)})
		return
	}
	if hub := sentryecho.GetHubFromContext(c); hub != nil {
		hub.CaptureException(err)
	}
	var pqe *pq.Error
	if errors.As(err, &pqe) && (pqe.Code == "53300" || pqe.Code == "53400") {
		_ = c.JSON(http.StatusServiceUnavailable, serializers.JSONError{Detail: fmt.Sprintf("Service Unavailable, there is currently too many connections to the database. Postgresql error code: %v", pqe.Code)})
		return
	}
	_ = c.JSON(http.StatusInternalServerError, serializers.JSONError{Detail: "Internal Server Error, Seems like there is a problem in the application, please retry later."})
	return
}
