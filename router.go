package main

import (
	"github.com/labstack/echo/v4/middleware"
	"net/http"

	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	e.GET("/", func(c echo.Context) error { return c.String(http.StatusTeapot, "I'm a teapot") })

	s := e.Group("/servers")
	s.Use(checkIfLoggedIn)

	s.GET("", getServers)
	s.POST("", createServer)

	// TODO: check if the user is Admin or not and give users only the servers they have access to
	s.POST("/power/:id/:status", powerServer)

	s.GET("/:id", getServers)

	// TODO: check if the user is Admin or not and give users only the servers they have access to
	s.DELETE("/:id", deleteServer)

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

	g := e.Group("/admin")

	g.Use(checkIfLoggedInAsAdmin)

	g.POST("/ipAdresses", createIpAdress)

	// force the templates to be re-cached
	g.GET("/templates/refresh", refreshTemplates)
	g.GET("/dataStores/refresh", refreshDataStores)

	e.POST("/auth/login", login)

	e.Logger.Fatal(e.Start(":1323"))
}
