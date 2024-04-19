package main

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

type Servers struct {
	ID               int
	Users_id         int
	Name             string
	Description      string
	End_date         string
	Operating_system string
	Storage          int
	Memory           int
	IP               string
}

func getServers(c echo.Context) error {
	getSession()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	db, err := connectToDB()
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	// Prepare statement for reading data
	rows, err := db.Query("SELECT id, users_id, name, description, end_date, operating_system, storage, memory, ip FROM virtual_machines")
	if err != nil {
		log.Fatal("Error executing query: ", err)
	}
	defer rows.Close()

	var rowsArr []Servers
	for rows.Next() {
		var s Servers

		err = rows.Scan(&s.ID, &s.Users_id, &s.Name, &s.Description, &s.End_date, &s.Operating_system, &s.Storage, &s.Memory, &s.IP)
		if err != nil {
			log.Fatal("Error scanning row: ", err)
		}

		// Print out the server struct
		log.Println(s)

		rowsArr = append(rowsArr, s)
	}
	// return the result as a json object
	return c.JSON(http.StatusOK, rowsArr)
}
