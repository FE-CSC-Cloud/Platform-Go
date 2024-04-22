package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.GET("/", func(c echo.Context) error { return c.String(http.StatusTeapot, "I'm a teapot") })

	// servers group
	// TODO: check if the user is Admin or not and give users only the servers they have access to
	e.GET("/servers", getServers)
	// TODO: server aan de requesting user toevoegen
	e.POST("/servers", createServer)

	e.GET("/templates", getTemplates)

	e.Logger.Fatal(e.Start(":1323"))
}
