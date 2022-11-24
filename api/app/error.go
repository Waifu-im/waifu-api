package app

import (
	"fmt"
	"github.com/Waifu-im/waifu-api/api"
	"github.com/Waifu-im/waifu-api/database"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/lib/pq"
	"net/http"
)

func customHTTPErrorHandler(err error, c echo.Context) {
	if err == database.BlacklistedError {
		_ = c.JSON(http.StatusForbidden, api.JSONError{Detail: fmt.Sprintf("%v", err)})
		return
	}
	if err == middleware.ErrJWTMissing {
		_ = c.JSON(http.StatusUnauthorized, api.JSONError{Detail: "Missing or malformed token"})
		return
	}
	httpError, ok := err.(*echo.HTTPError)
	if ok {
		// echo JWT middleware package doesn't directly return the error they defined so the error cannot be directly compared
		if httpError.Message == middleware.ErrJWTInvalid.Message {
			_ = c.JSON(http.StatusUnauthorized, api.JSONError{Detail: "Invalid token (note that if you are using a token generated before the golang version of the api, the token encryption has changed, please check your new token at https://waifu.im/dashboard/"})
			return
		}
		_ = c.JSON(httpError.Code, api.JSONError{Detail: fmt.Sprintf("%v", httpError.Message)})
		return
	}
	be, ok := err.(*echo.BindingError)
	if ok {
		_ = c.JSON(http.StatusBadRequest, api.JSONError{Detail: fmt.Sprintf("Bad Request, error on %v, %v", be.Field, be.Message)})
		return
	}
	fmt.Println(err)
	pqe, ok := err.(*pq.Error)
	if ok && (pqe.Code == "53300" || pqe.Code == "53400") {
		_ = c.JSON(http.StatusServiceUnavailable, api.JSONError{Detail: fmt.Sprintf("Service Unavailable, there is currently too many connections to the database. Postgresql error code: %v", pqe.Code)})
		return
	}
	_ = c.JSON(http.StatusInternalServerError, api.JSONError{Detail: "Internal Server Error, Seems like there is a problem in the application, please retry later."})
	return
}
