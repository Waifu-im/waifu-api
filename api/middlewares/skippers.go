package middlewares

import (
	"bytes"
	"encoding/json"
	"github.com/Waifu-im/waifu-api/constants"
	"github.com/Waifu-im/waifu-api/serializers"
	"github.com/labstack/echo/v4"
	"io"
)

func defaultSkipper(_ echo.Context) (bool, error) {
	return false, nil
}

// BoolParamsSkipper return true when skipAfter and no param provided (only for bool param).
// If a param is provided it can assign it to a context 'variable' if contextKey is passed
func BoolParamsSkipper(sourceParam string, contextKey string, skipAfter bool) func(echo.Context) (bool, error) {
	return func(c echo.Context) (bool, error) {
		var param bool
		if err := echo.QueryParamsBinder(c).Bool(sourceParam, &param).BindError(); err != nil {
			return false, err
		}
		if contextKey != "" {
			(c).Set(contextKey, param)
		}
		return !param && skipAfter, nil
	}
}

// Int64ParamsSkipper return true when skipAfter and no param provided (only for int64 param).
// If a param is provided it can assign it to a context 'variable' if contextKey is passed
func Int64ParamsSkipper(sourceParam string, contextKey string, skipAfter bool) func(echo.Context) (bool, error) {
	return func(c echo.Context) (bool, error) {
		var param int64
		if err := echo.QueryParamsBinder(c).Int64(sourceParam, &param).BindError(); err != nil {
			return false, err
		}
		if param == 0 && skipAfter {
			return true, nil
		}
		if contextKey != "" {
			(c).Set(contextKey, param)
		}
		return false, nil
	}
}

// LimitParamsSkipper return true when skipAfter and the param provided is not superior to MaxLimit (only for int param).
// If a param is provided it can assign it to a context 'variable' if contextKey is passed
func LimitParamsSkipper(sourceParam string, contextKey string, skipAfter bool) func(echo.Context) (bool, error) {
	return func(c echo.Context) (bool, error) {
		var param int
		if err := echo.QueryParamsBinder(c).Int(sourceParam, &param).BindError(); err != nil {
			return false, err
		}
		if param <= constants.MaxLimit && skipAfter {
			return true, nil
		}
		if contextKey != "" {
			(c).Set(contextKey, param)
		}
		return false, nil
	}
}

// SkipOrSetUser I use this 'skipper' has a function to set the user id present in the body so that permissions can check for it later
func SkipOrSetUser(isSkipper bool) func(echo.Context) (bool, error) {
	return func(c echo.Context) (bool, error) {
		var bodyBytes []byte
		bodyBytes, _ = io.ReadAll((c).Request().Body)
		(c).Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		user := serializers.User{}
		_ = json.Unmarshal(bodyBytes, &user)
		if user.Id == 0 {
			return isSkipper, nil
		}
		(c).Set("target_user_id", user.Id)
		return false, nil
	}
}
