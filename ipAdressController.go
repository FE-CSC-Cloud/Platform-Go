package main

import (
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

func createIpAdress(c echo.Context) error {
	var ipAddresses []string
	var failedIpAddresses []string

	if err := c.Bind(&ipAddresses); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	db, err := connectToDB()
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	for _, ip := range ipAddresses {
		_, err := db.Exec("INSERT INTO ip_adresses SET `ip` =?", ip)
		if err != nil {
			failedIpAddresses = append(failedIpAddresses, ip)
			log.Println("Error inserting ip address: ", err)
		}
		log.Println("Creating ip address: ", ip)
	}

	defer db.Close()

	if len(failedIpAddresses) > 0 {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "Failed to insert ip addresses", "failedIpAddresses": failedIpAddresses})
	}

	return c.JSON(http.StatusCreated, map[string]string{"message": "Ip addresses created"})
}
