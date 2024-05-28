package main

import (
	"fmt"
	"github.com/go-ldap/ldap/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"strings"
	"time"
)

func login(c echo.Context) error {
	// make a type for the request body
	type LoginResponse struct {
		Token string `json:"token"`
		User  struct {
			Name    string `json:"name"`
			IsAdmin bool   `json:"is_admin"`
		}
	}

	username := c.FormValue("username")
	password := c.FormValue("password")

	ldapConn, err := connectAndBind(username, password)
	if err != nil {
		return c.String(http.StatusUnauthorized, err.Error())
	}
	defer ldapConn.Close()

	groups, ye, err := fetchGroupsAndDisplayNames(ldapConn, username)
	log.Println(ye)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch groups")
	}

	isAdmin := checkIfAdmin(groups)

	// Create a JWT token
	token, err := generateToken(username, isAdmin)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to create token")
	}

	// Save token in database
	err = saveTokenInDB(token, username)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to save token in database")
	}

	response := LoginResponse{
		Token: token,
		User: struct {
			Name    string `json:"name"`
			IsAdmin bool   `json:"is_admin"`
		}{
			Name:    username,
			IsAdmin: isAdmin,
		},
	}

	return c.JSON(http.StatusOK, response)
}

func connectAndBind(username string, password string) (*ldap.Conn, error) {
	ldapURL := "ldap://" + getEnvVar("LDAP_HOST") + ":389"
	ldapConn, err := ldap.DialURL(ldapURL)
	if err != nil {
		fmt.Println(err)
		return nil, fmt.Errorf("Failed to connect to LDAP server")
	}

	// Bind with provided username and password to validate the user
	err = ldapConn.Bind(username+"@"+getEnvVar("LDAP_READ_DOMAIN"), password)
	if err != nil {
		fmt.Println(err, "Bind failed")
		return nil, fmt.Errorf("Email or password is incorrect")
	}

	return ldapConn, nil
}

// TODO: also fetch display name
func fetchGroupsAndDisplayNames(ldapConn *ldap.Conn, username string) ([]string, string, error) {
	searchRequest := ldap.NewSearchRequest(
		"DC=OICLOUD,DC=LOCAL",
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", username),
		[]string{"memberOf"},
		nil,
	)

	sr, err := ldapConn.Search(searchRequest)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to search LDAP server")
	}

	var groups []string

	for _, entry := range sr.Entries {
		for _, attr := range entry.Attributes {
			for _, value := range attr.Values {
				group := value[:strings.Index(value, ",")]
				group = group[3:]
				groups = append(groups, group)
			}
		}
	}

	return groups, "", err
}

func checkIfAdmin(groups []string) bool {
	for _, group := range groups {
		if group == "Administrators" {
			return true
		}
	}
	return false
}

func generateToken(name string, admin bool) (string, error) {
	// Create JWT token
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["name"] = name
	claims["admin"] = admin
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()

	// Generate encoded token and send it as response.
	t, err := token.SignedString([]byte(getEnvVar("JWT_SECRET")))
	if err != nil {
		return "", err
	}

	return t, nil

}

func saveTokenInDB(token string, username string) error {
	// Save token in database
	db, err := connectToDB()
	if err != nil {
		return err
	}

	// Convert Unix timestamp to MySQL datetime string, expires in 3 days
	expiresAt := time.Unix(time.Now().Add(time.Hour*72).Unix(), 0).Format("2006-01-02 15:04:05")

	// Insert the token into the database
	_, err = db.Exec("INSERT INTO user_tokens (token, expires_at, belongs_to) VALUES (?, ?, ?)", token, expiresAt, username)
	if err != nil {
		log.Println(err)
		return err
	}

	// Close the database connection
	err = db.Close()

	return nil
}

func removeOldTokensFromDB(c echo.Context) error {
	// Remove tokens that have expired
	db, err := connectToDB()
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	_, err = db.Exec("DELETE FROM user_tokens WHERE expires_at < NOW()")
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, "Failed to delete old tokens from database")
	}

	err = db.Close()
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, "Failed to close database connection")
	}

	return c.JSON(http.StatusOK, "Old tokens have been removed")
}
