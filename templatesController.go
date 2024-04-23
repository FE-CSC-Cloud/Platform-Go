package main

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

func getTemplates(c echo.Context) error {
	session := getVCenterSession()
	templates := getTemplatesFromVCenter(session)
	return c.JSON(http.StatusOK, templates)
}
