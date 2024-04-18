package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/robaho/leveldb"
	"log"
)

// opens a connection to the MySQL database
func connectToDB() (*sql.DB, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file") // Log an error and stop the program if the .env file can't be loaded
	}

	dbUser := getEnvVar("DB_USER")
	dbPass := getEnvVar("DB_PASS")
	dbHost := getEnvVar("DB_HOST")
	dbPort := getEnvVar("DB_PORT")
	dbName := getEnvVar("DB_NAME")

	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName))
	if err != nil {
		return nil, err
	}

	// ping the database to check if the connection is successful
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

// opens a connection to the KeyDB database
func connectToKeyDB() *leveldb.Database {
	// Connect to KeyDB
	db, err := leveldb.Open("keydb", leveldb.Options{})

	if err != nil {
		log.Fatal("unable to create database", err)
	}

	return db
}
