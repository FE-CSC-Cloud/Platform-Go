package main

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"time"
)

func Login(c echo.Context) error {
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

func checkIfTokenIsValid(c echo.Context) error {
	token := formatJWTfromBearer(c)

	// check if the token exists in the database
	db, err := connectToDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	var tokenFromDB string
	err = db.QueryRow("SELECT token FROM user_tokens WHERE token = ?", token).Scan(&tokenFromDB)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, "Token is invalid")
	}

	db.Close()

	return c.JSON(http.StatusOK, "Token is valid")
}
