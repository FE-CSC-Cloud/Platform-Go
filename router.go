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

	s.GET("", GetServers)
	s.POST("", CreateServer)

	s.POST("/power/:id/:status", PowerServer)

	s.GET("/:id", GetServers)

	s.DELETE("/:id", DeleteServer)

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
	e.GET("/templates", GetTemplates)

	g := e.Group("/admin")

	g.Use(checkIfLoggedInAsAdmin)

	g.POST("/ipAdresses", CreateIpAdress)

	// force the templates to be re-cached
	g.GET("/templates/refresh", RefreshTemplates)
	g.GET("/dataStores/refresh", RefreshDataStores)

	e.POST("/auth/login", Login)

	e.Logger.Fatal(e.Start(":1323"))
}
