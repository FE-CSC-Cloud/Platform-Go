package main

import (
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
	"time"
)

func checkIfLoggedIn(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		token := formatJWTfromBearer(c)

		// check if the token is valid
		if token == "" {
			return echo.ErrUnauthorized
		}

		valid, expired, err := checkJWT(token)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, "Token Invalid")
		}
		if !valid {
			return c.JSON(http.StatusUnauthorized, "Token Invalid")
		}
		if expired {
			return c.JSON(http.StatusUnauthorized, "Token is expired")
		}

		tokenValid := checkTokenAgainstDB(token)
		if !tokenValid {
			return echo.ErrUnauthorized
		}

		return next(c)
	}
}

func formatJWTfromBearer(c echo.Context) string {
	// get the token from the request as bearer token
	token := c.Request().Header.Get("Authorization")

	// strip the "bearer " part of the token
	token = strings.TrimPrefix(token, "Bearer ")

	return token
}

// valid, expired, error
func checkJWT(token string) (bool, bool, error) {
	// get the JWT secret from the environment
	jwtSecret := getEnvVar("JWT_SECRET")

	// parse the token
	t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return false, false, err
	}

	// check if the token is valid
	if !t.Valid {
		return false, false, fmt.Errorf("token is invalid")
	}

	// check if the token is expired
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return false, false, fmt.Errorf("invalid claims")
	}

	if claims["exp"] == nil {
		return false, false, fmt.Errorf("no expiration time")
	}

	if int64(claims["exp"].(float64)) < time.Now().Unix() {
		return true, true, nil
	}

	return true, false, nil
}

// bool 1: token is valid
// bool 2: token is expired
func checkTokenAgainstDB(token string) bool {
	// connect to the database
	db, err := connectToDB()
	if err != nil {
		return false
	}

	defer db.Close()

	// query the database for the token
	rows, err := db.Query("SELECT token FROM user_tokens WHERE token = ?", token)
	if err != nil {
		return false
	}

	// check if the token is in the database
	if !rows.Next() {
		return false
	}

	return true
}
