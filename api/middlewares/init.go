package middlewares

import (
	"encoding/json"
	"github.com/getsentry/sentry-go"
	sentryecho "github.com/getsentry/sentry-go/echo"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"strconv"
	"strings"
)

func Init(e *echo.Echo) {
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

			rawVersion := c.Request().Header.Get("Version")

			if rawVersion != "" {
				max := len(rawVersion)
				// Check if string length is > than 20 if yes we set 20 to the max
				if len(rawVersion) > 20 {
					max = 20
				}
				version = rawVersion[0:max]
			}

			rawBody, err := json.Marshal(string(resBody))
			cleanBody := string(rawBody)

			if err == nil {
				cleanBody = strings.Replace(string(rawBody), `\n`, ``, -1)
				cleanBody = strings.Replace(cleanBody, `\`, ``, -1)
				cleanBody = strings.Replace(cleanBody, `"`, `'`, -1)
			}

			if hub := sentryecho.GetHubFromContext(c); hub != nil {
				hub.WithScope(func(scope *sentry.Scope) {
					scope.SetRequest(c.Request())
					scope.SetFingerprint([]string{c.Request().Header.Get("X-Request-Id")})
					scope.SetLevel(sentry.LevelInfo)
					scope.SetTag("level", string(sentry.LevelInfo))
					scope.SetTag("version", version)
					scope.SetTag("status_code", strconv.Itoa(c.Response().Status))
					scope.SetTag("ip_address", c.RealIP())
					scope.SetTag("user_agent", c.Request().Header.Get("User-Agent"))
					scope.SetTag("type", "req")
					scope.SetTag("path", c.Request().URL.Path)
					scope.SetTag("request_id", c.Request().Header.Get("X-Request-Id"))

					scope.SetContext("Response", map[string]interface{}{
						"status_code": c.Response().Status,
						"body":        cleanBody,
					})

					if execTime != 0 {
						scope.SetContext("Database", map[string]interface{}{
							"query_execution_time": strconv.FormatInt(execTime, 10) + "ms",
						})
					}

					hub.CaptureMessage("REQ - " + c.Request().URL.Path)
				})
			}
		}}))
}
