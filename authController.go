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
	token, err := generateLoginToken(SID, fullName, studentId, isAdmin)
	if err != nil {
		log.Println(err)
		return c.String(http.StatusInternalServerError, "Login failed")
	}

	// Save token in database
	err = saveLoginTokenInDB(token, username)
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

func ResetRequest(c echo.Context) error {
	email := c.FormValue("email")

	_, _, _, userName, _, err := fetchUserInfoWithEmail(email)
	if err != nil {
		log.Println(err)
		return c.String(http.StatusNotFound, "Email not found")
	}

	if userName == "" {
		return c.String(http.StatusNotFound, "Email not found")
	}

	err = removeTokensFromUser(userName)
	if err != nil {
		log.Println(err)
		return c.String(http.StatusInternalServerError, "Failed to remove old tokens from user")
	}

	token := jwt.New(jwt.SigningMethodHS256)
	claims := token.Claims.(jwt.MapClaims)
	claims["email"] = email
	claims["exp"] = time.Now().Add(time.Hour * 1).Unix()

	t, err := token.SignedString([]byte(getEnvVar("JWT_SECRET")))
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to generate token")
	}

	// Send email with reset instructions in the background
	go func() {
		sendEmailNotification(email, "Password reset", fmt.Sprintf("Click the following link to reset your password: %s/auth/resetPassword?token=%s", getEnvVar("FRONTEND_URL"), t))
	}()

	return c.String(http.StatusOK, "Email sent with reset instructions")
}

func ResetPassword(c echo.Context) error {
	token := c.FormValue("token")
	password := c.FormValue("password")

	// get the JWT secret from the environment
	jwtSecret := getEnvVar("JWT_SECRET")

	// parse the token
	t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return c.String(http.StatusUnauthorized, "Token is invalid")
	}

	// check if the token is valid
	if !t.Valid {
		return c.String(http.StatusUnauthorized, "Token is invalid")
	}

	// check if the token is expired
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return c.String(http.StatusUnauthorized, "Token is invalid")
	}

	if claims["email"] == nil {
		return c.String(http.StatusUnauthorized, "Token is invalid")
	}

	email := claims["email"].(string)

	// get the username associated with the email
	_, _, sid, _, _, err := fetchUserInfoWithEmail(email)
	if err != nil {
		return c.String(http.StatusNotFound, "Email not found")
	}

	err = resetPasswordOfSidUser(sid, password)
	if err != nil {
		return err
	}

	return c.String(http.StatusOK, "Password has been reset")
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

func generateLoginToken(SID string, fullName string, studentId string, admin bool) (string, error) {
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

func saveLoginTokenInDB(token string, username string) error {
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

func removeTokensFromUser(userName string) error {
	db, err := connectToDB()
	if err != nil {
		return err
	}

	_, err = db.Exec("DELETE FROM user_tokens WHERE belongs_to = ?", userName)
	if err != nil {
		return err
	}

	err = db.Close()
	if err != nil {
		return err
	}

	return nil
}

func RemoveOldTokensFromDB(c echo.Context) error {
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

func CheckIfLoginTokenIsValid(c echo.Context) error {
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
