package main

import (
	"fmt"
	"github.com/go-ldap/ldap/v3"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

func login(c echo.Context) error {
	/*username := c.FormValue("username")
	  password := c.FormValue("password")*/

	ldapURL := "ldap://" + getEnvVar("LDAP_HOST") + ":389"
	ldapConn, err := ldap.DialURL(ldapURL)
	if err != nil {
		fmt.Println(err)
		return c.String(http.StatusInternalServerError, "Failed to connect to LDAP server")
	}
	defer ldapConn.Close()

	log.Println()

	// Bind with provided username and password
	err = ldapConn.Bind(getEnvVar("LDAP_READ_USER")+"@"+getEnvVar("LDAP_READ_DOMAIN"), getEnvVar("LDAP_READ_PASS"))
	if err != nil {
		fmt.Println(err, "Bind failed")
		return c.String(http.StatusInternalServerError, "Failed to bind to LDAP server")
	}

	searchRequest := ldap.NewSearchRequest(
		// zoeken achterstevoren dus map1/map2/map3 is ou=map3, ou=map2, ou=map1
		"dc=OICLOUD,dc=LOCAL", // The base dn to search
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=organizationalPerson)(uid=%s))", "username"), // The filter to apply
		[]string{"dn", "cn", "mail"}, // A list attributes to retrieve
		nil,
	)

	sr, err := ldapConn.Search(searchRequest)
	if err != nil {
		fmt.Println(err, "Search failed")
		return nil
	}

	if len(sr.Entries) != 1 {
		fmt.Println("User does not exist or too many entries returned")
		return nil
	}

	userdn := sr.Entries[0].DN

	// Bind as the user to verify their password
	err = ldapConn.Bind(userdn, "user-password")
	if err != nil {
		fmt.Println("Password incorrect")
		return nil
	}

	return c.String(http.StatusOK, "Logged in")
}
