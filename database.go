package main

import (
	"database/sql"
)

func connectToDB() {
	dbUser := getEnvVar("DB_USER")
	dbPass := getEnvVar("DB_PASS")

	db, err := sql.Open("mysql", dbUser+":"+dbPass+"@/Login")
	if err != nil {
		panic(err)
	}
	defer db.Close()
}
