package middlewares

import (
	"bytes"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"io/ioutil"
)

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
func SkipOrSetUser(isSkipper bool) func(*echo.Context) (bool, error) {
	// I use this 'skipper' has a function to set the user id present in the body so that permissions can check for it later
	return func(c *echo.Context) (bool, error) {
		var bodyBytes []byte
		bodyBytes, _ = ioutil.ReadAll((*c).Request().Body)
		(*c).Request().Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		user := struct {
			Id uint `json:"user_id"`
		}{}
		_ = json.Unmarshal(bodyBytes, &user)
		if user.Id == 0 {
			return isSkipper, nil
		}
		(*c).Set("target_user_id", user.Id)
		return false, nil
	}
}
