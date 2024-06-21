package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"log"
)

var (
	dbUser string
	dbPass string
	dbHost string
	dbPort string
	dbName string
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
	}

	dbUser = getEnvVar("DB_USER")
	dbPass = getEnvVar("DB_PASS")
	dbHost = getEnvVar("DB_HOST")
	dbPort = getEnvVar("DB_PORT")
	dbName = getEnvVar("DB_NAME")
}

// opens a connection to the MySQL database
func connectToDB() (*sql.DB, error) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName))
	if err != nil {
		return nil, err
	}

	// ping the database to check if the connection is successful
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	// Set maximum number of connections
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(20)

	return db, nil
}
