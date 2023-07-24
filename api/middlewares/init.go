package middlewares

import (
	"encoding/json"
	"github.com/Waifu-im/waifu-api/api/utils"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func Init(globals utils.Globals, e *echo.Echo) {
	e.Pre(middleware.RemoveTrailingSlash())
	/*
		Already set with nginx

		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowCredentials: true,
			AllowOrigins:     []string{"*"},
			AllowMethods: []string{
				http.MethodGet,
				http.MethodHead,
				http.MethodPut,
				http.MethodPatch,
				http.MethodPost,
				http.MethodDelete,
				http.MethodOptions,
			},
			AllowHeaders: []string{"Accept-Version", "Authorization", "Content-Type"},
		}))
	*/
	// Using default logger
	e.Use(sentryecho.New(sentryecho.Options{}))
	e.Use(middleware.Logger())
	e.Use(middleware.BodyDumpWithConfig(middleware.BodyDumpConfig{
		Handler: func(c echo.Context, reqBody, resBody []byte) {
			var execTime int64
			var version string
			if execTimeInterface := c.Get("search_query_exec_time"); execTimeInterface != nil {
				execTime = execTimeInterface.(int64)
			}

			userId := utils.GetUser(c).Id

			rawVersion := c.Request().Header.Get("Version")

			if rawVersion != "" {
				max := len(rawVersion)
				// Check if string length is > than 20 if yes we set 20 to the max
				if len(rawVersion) > 20 {
					max = 20
				}
				version = rawVersion[0:max]
			}

			stringResBody := string(resBody)
			headersJSON, _ := json.Marshal(c.Request().Header)

			go globals.Database.LogRequest(
				c.RealIP(),
				"https://"+c.Request().Host+c.Request().RequestURI,
				c.Request().UserAgent(),
				userId,
				version,
				execTime,
				string(headersJSON),
				stringResBody,
				c.Response().Status,
			)
		}}))
}
