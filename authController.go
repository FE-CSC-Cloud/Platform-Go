package main

import (
	"fmt"
	"github.com/go-ldap/ldap/v3"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

func login(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	ldapURL := "ldap://" + getEnvVar("LDAP_HOST") + ":389"
	ldapConn, err := ldap.DialURL(ldapURL)
	if err != nil {
		fmt.Println(err)
		return c.String(http.StatusInternalServerError, "Failed to connect to LDAP server")
	}
	defer ldapConn.Close()

	log.Println()

	// Bind with provided username and password
	// err = ldapConn.Bind(username+"@"+getEnvVar("LDAP_READ_DOMAIN"), password)
	// TODO: line below is not tested, above works ^
	err = ldapConn.Bind(username+"@"+getEnvVar("LDAP_READ_DOMAIN"), password)
	if err != nil {
		fmt.Println(err, "Bind failed")
		return c.String(http.StatusInternalServerError, "Failed to bind to LDAP server")
	}

	return c.String(http.StatusOK, "Logged in")
}
