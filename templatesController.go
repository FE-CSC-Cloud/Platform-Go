package main

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

func getTemplates(c echo.Context) error {
	session := getVCenterSession()
	fetchTemplatesFromVCenter(session)

	return c.String(http.StatusOK, "Templates fetched")
}
