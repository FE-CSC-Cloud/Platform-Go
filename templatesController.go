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

func refreshTemplates(c echo.Context) error {
	// drop the last updated key from redis so the templates will be updated
	deleteFromRedis("templates_last_updated")
	getTemplates(c)

	return c.JSON(http.StatusOK, "Template Cache refreshed.")
}
