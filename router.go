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

	// s.PATCH("/:id", UpdateServer)

	s.POST("/power/:id/:status", PowerServer)

	s.GET("/:id", GetServers)

	s.DELETE("/:id", DeleteServer)

	d := e.Group("/dns")
	d.Use(checkIfLoggedIn)

	d.GET("", GetDnsZones)
	d.POST(":serverId", CreateDnsRecord)

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

	a := e.Group("/auth")

	a.POST("/login", Login)
	a.POST("/resetRequest", ResetRequest)
	a.POST("/resetPassword", ResetPassword)
	e.GET("checkIfLoginTokenIsValid", CheckIfLoginTokenIsValid)

	n := e.Group("/notifications")
	n.Use(checkIfLoggedIn)

	n.GET("", GetNotifications)
	n.PATCH("/:id", MarkNotificationAsRead)

	e.Logger.Fatal(e.Start(":" + getEnvVar("APP_PORT")))
}
