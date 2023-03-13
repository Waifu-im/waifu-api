package middlewares

import (
	"bytes"
	"encoding/json"
	"github.com/Waifu-im/waifu-api/models"
	"github.com/labstack/echo/v4"
	"io"
)

// BoolParamsSkipper return true when skipAfter and no param provided (only for bool param).
// If a param is provided it can assign it to a context 'variable' if contextKey is passed
func BoolParamsSkipper(sourceParam string, contextKey string, skipAfter bool) func(*echo.Context) (bool, error) {
	return func(c *echo.Context) (bool, error) {
		var param bool
		if err := echo.QueryParamsBinder(*c).Bool(sourceParam, &param).BindError(); err != nil {
			return false, err
		}
		if contextKey != "" {
			(*c).Set(contextKey, param)
		}
		return !param && skipAfter, nil
	}
}

// UIntParamsSkipper return true when skipAfter and no param provided (only for uint param).
// If a param is provided it can assign it to a context 'variable' if contextKey is passed
func UIntParamsSkipper(sourceParam string, contextKey string, skipAfter bool) func(ctx *echo.Context) (bool, error) {
	return func(c *echo.Context) (bool, error) {
		var param uint
		if err := echo.QueryParamsBinder(*c).Uint(sourceParam, &param).BindError(); err != nil {
			return false, err
		}
		if param == 0 && skipAfter {
			return true, nil
		}
		if contextKey != "" {
			(*c).Set(contextKey, param)
		}
		return false, nil
	}
}

// SkipOrSetUser I use this 'skipper' has a function to set the user id present in the body so that permissions can check for it later
func SkipOrSetUser(isSkipper bool) func(*echo.Context) (bool, error) {
	return func(c *echo.Context) (bool, error) {
		var bodyBytes []byte
		bodyBytes, _ = io.ReadAll((*c).Request().Body)
		(*c).Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		user := models.User{}
		_ = json.Unmarshal(bodyBytes, &user)
		if user.Id == 0 {
			return isSkipper, nil
		}
		(*c).Set("target_user_id", user.Id)
		return false, nil
	}
}
