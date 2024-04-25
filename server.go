package main

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.GET("/", func(c echo.Context) error { return c.String(http.StatusTeapot, "I'm a teapot") })

	// TODO: check if the user is Admin or not and give users only the servers they have access to
	e.GET("/servers", getServers)
	// TODO: server aan de requesting user toevoegen
	e.POST("/servers", createServer)

	e.DELETE("/servers/:id", deleteServer)

	// TODO: array met template IDs cachen (fetchTemplateLibraryIdsFromVCenter)
	// TODO: JSON het zelfde maken als de Laravel JSON
	/*
	   {
	       "UBUNTU TEMPLATE": {
	           "storage": 20,
	           "memory": 1,
	           "os": "UBUNTU_64"
	       },
	       "OICT-AUTO-Template": {
	           "storage": 20,
	           "memory": 1,
	           "os": "UBUNTU_64"
	       },
	       "OICT-AUTO-DEBIAN": {
	           "storage": 20,
	           "memory": 1,
	           "os": "UBUNTU_64"
	       }
	   }
	*/
	e.GET("/templates", getTemplates)

	// force the templates to be recached
	// TODO: zorgen dat alleen Admins dit kunnen doen
	e.GET("/templates/refresh", refreshTemplates)

	// TODO: zorgen dat alleen Admins dit kunnen doen
	e.GET("/dataStores/refresh", refreshDataStores)

	e.Logger.Fatal(e.Start(":1323"))
}
