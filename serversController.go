package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
)

func getServers(c echo.Context) error {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")

	db, err := sql.Open("mysql", dbUser+":"+dbPass+"@/Login")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Open doesn't open a connection. Validate DSN data:
	err = db.Ping()
	if err != nil {
		panic(err.Error()) // proper error handling instead of panic in your app
	}

	// Prepare statement for reading data
	rows, err := db.Query("SELECT * FROM users")
	defer rows.Close()

	// return the result as a json object
	return c.JSON(http.StatusOK, rows)

	// return c.String(http.StatusOK, "dbUser "+dbUser)
}
