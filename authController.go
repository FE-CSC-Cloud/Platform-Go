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
		log.Println(err)
		return c.String(http.StatusUnauthorized, "Username or password incorrect")
	}
	defer ldapConn.Close()

	groups, fullName, studentId, SID, err := fetchUserInfo(ldapConn, username)
	if err != nil {
		log.Println(err)
		return c.String(http.StatusInternalServerError, "Login failed")
	}

	isAdmin := checkIfAdmin(groups)

	// Create a JWT token
	token, err := generateToken(SID, fullName, studentId, isAdmin)
	if err != nil {
		log.Println(err)
		return c.String(http.StatusInternalServerError, "Login failed")
	}

	// Save token in database
	err = saveTokenInDB(token, username)
	if err != nil {
		log.Println(err)
		return c.String(http.StatusInternalServerError, "Login failed")
	}

	response := LoginResponse{
		Token: token,
		User: struct {
			Name    string `json:"name"`
			IsAdmin bool   `json:"is_admin"`
		}{
			Name:    fullName,
			IsAdmin: isAdmin,
		},
	}

	return c.JSON(http.StatusOK, response)
}

func connectAndBind(username string, password string) (*ldap.Conn, error) {
	ldapURL := "ldap://" + getEnvVar("LDAP_HOST") + ":389"
	ldapConn, err := ldap.DialURL(ldapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP server")
	}

	// Bind with provided username and password to validate the user
	err = ldapConn.Bind(username+"@"+getEnvVar("LDAP_READ_DOMAIN"), password)
	if err != nil {
		return nil, fmt.Errorf("email or password is incorrect")
	}

	return ldapConn, nil
}

func fetchUserInfo(ldapConn *ldap.Conn, username string) ([]string, string, string, string, error) {
	searchRequest := ldap.NewSearchRequest(
		getEnvVar("LDAP_BASE_DN"),
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", username),
		[]string{"memberOf", "givenName", "description", "sn", "objectSid"},
		nil,
	)

	sr, err := ldapConn.Search(searchRequest)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to search LDAP server: %v", err)
	}

	if len(sr.Entries) == 0 {
		return nil, "", "", "", fmt.Errorf("no entries found for user %s", username)
	}

	entry := sr.Entries[0]

	memberOf := entry.GetAttributeValues("memberOf")

	firstName := entry.GetAttributeValue("givenName")

	description := entry.GetAttributeValue("description")
	// last name is an array for some reason so we have to check if it exists
	var lastName string
	if len(entry.GetAttributeValues("sn")) >= 1 {
		lastName = entry.GetAttributeValues("sn")[0]
	}

	fullName := firstName + " " + lastName

	var groups []string

	// Get only the CN= and not the OU=
	for _, dn := range memberOf {
		// Extract the part of the DN starting with "CN=" and ending before the next comma
		start := strings.Index(dn, "CN=")
		if start != -1 {
			start += 3 // Skip past "CN="
			end := strings.Index(dn[start:], ",")
			if end != -1 {
				groups = append(groups, dn[start:start+end])
			} else {
				groups = append(groups, dn[start:])
			}
		}
	}

	// Extract the SID
	objectSid := entry.GetRawAttributeValue("objectSid")
	sidString := sidToString(objectSid)

	return groups, fullName, description, sidString, err
}

func sidToString(sid []byte) string {
	// Convert the SID from byte format to a readable string format
	if len(sid) < 8 {
		return ""
	}
	// Version is the first byte
	revision := sid[0]
	// Sub-authority count is the second byte
	subAuthorityCount := sid[1]
	// Authority is the next 6 bytes
	authority := uint64(sid[2])<<40 | uint64(sid[3])<<32 | uint64(sid[4])<<24 | uint64(sid[5])<<16 | uint64(sid[6])<<8 | uint64(sid[7])
	sidString := fmt.Sprintf("S-%d-%d", revision, authority)
	// Sub-authorities are the rest of the bytes
	for i := 0; i < int(subAuthorityCount); i++ {
		subAuth := uint32(sid[8+4*i]) | uint32(sid[8+4*i+1])<<8 | uint32(sid[8+4*i+2])<<16 | uint32(sid[8+4*i+3])<<24
		sidString += fmt.Sprintf("-%d", subAuth)
	}
	return sidString
}

func checkIfAdmin(groups []string) bool {
	for _, group := range groups {
		if group == "Gilde Members" {
			return true
		}
	}
	return false
}

func generateToken(SID string, fullName string, studentId string, admin bool) (string, error) {
	// Create JWT token
	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["sid"] = SID
	claims["givenName"] = fullName
	claims["studentId"] = studentId
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
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	_, err = db.Exec("DELETE FROM user_tokens WHERE expires_at < NOW()")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to delete old tokens from database")
	}

	err = db.Close()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to close database connection")
	}

	return c.JSON(http.StatusOK, "Old tokens have been removed")
}

func getUserAssociatedWithJWT(c echo.Context) (string, bool, string, string) {
	token := formatJWTfromBearer(c)

	// get the JWT secret from the environment
	jwtSecret := getEnvVar("JWT_SECRET")

	// parse the token
	t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return "", false, "", ""
	}

	// check if the token is valid
	if !t.Valid {
		return "", false, "", ""
	}

	// check if the token is expired
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return "", false, "", ""
	}

	if claims["sid"] == nil {
		return "", false, "", ""
	}

	if claims["admin"] == nil {
		return "", false, "", ""
	}

	if claims["givenName"] == nil {
		return "", false, "", ""
	}

	if claims["studentId"] == nil {
		return "", false, "", ""
	}

	return claims["sid"].(string), claims["admin"].(bool), claims["givenName"].(string), claims["studentId"].(string)
}

func fetchUserInfoWithSID(sid string) (string, string, string, error) {
	// Connect to LDAP
	ldapConn, err := connectAndBind(getEnvVar("LDAP_READ_USER"), getEnvVar("LDAP_READ_PASS"))

	// Search for the user with the given SID
	searchRequest := ldap.NewSearchRequest(
		getEnvVar("LDAP_BASE_DN"),
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(objectSid=%s))", sid),
		[]string{"memberOf", "givenName", "description", "sn"},
		nil,
	)

	sr, err := ldapConn.Search(searchRequest)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to search LDAP server: %v", err)
	}

	if len(sr.Entries) == 0 {
		return "", "", "", fmt.Errorf("no entries found for user with SID %s", sid)
	}

	entry := sr.Entries[0]

	memberOf := entry.GetAttributeValues("memberOf")

	firstName := entry.GetAttributeValue("givenName")

	description := entry.GetAttributeValue("description")
	// last name is an array for some reason so we have to check if it exists
	var lastName string
	if len(entry.GetAttributeValues("sn")) >= 1 {
		lastName = entry.GetAttributeValues("sn")[0]
	}

	fullName := firstName + " " + lastName

	var groups []string

	// Get only the CN= and not the OU=
	for _, dn := range memberOf {
		// Extract the part of the DN starting with "CN=" and ending before the next comma
		start := strings.Index(dn, "CN=")
		if start != -1 {
			start += 3 // Skip past "CN="
			end := strings.Index(dn[start:], ",")
			if end != -1 {
				groups = append(groups, dn[start:start+end])
			} else {
				groups = append(groups, dn[start:])
			}
		}
	}

	return fullName, description, sid, nil
}
