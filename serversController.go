package main

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

type DBServers struct {
	ID               int
	Users_id         int
	Vcenter_id       string
	Name             string
	Description      string
	End_date         string
	Operating_system string
	Storage          int
	Memory           int
	IP               string
}

type vCenterServers struct {
	memory_size_MiB int
	vm              string
	name            string
	power_state     string
	cpu_count       int
}

type PowerStatusReturn struct {
	ID               int
	Users_id         int
	Vcenter_id       string
	Name             string
	Description      string
	End_date         string
	Operating_system string
	Storage          int
	Memory           int
	IP               string
	Power_status     string
}

func getServers(c echo.Context) error {
	// checkIfvCenterSessionIsExpired ~120ms, might not be needed; rest is ~1ms
	var session string = getVCenterSession()

	var serversFromVCenter []vCenterServers = getPowerStatus(session, "")

	db, err := connectToDB()
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	// Prepare statement for reading data
	rows, err := db.Query("SELECT id, users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip FROM virtual_machines")
	if err != nil {
		log.Fatal("Error executing query: ", err)
	}
	defer rows.Close()

	var rowsArr []DBServers
	for rows.Next() {
		var s DBServers

		err = rows.Scan(&s.ID, &s.Users_id, &s.Vcenter_id, &s.Name, &s.Description, &s.End_date, &s.Operating_system, &s.Storage, &s.Memory, &s.IP)
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

func checkIfInVCenter(DBId string, VCenterServers vCenterServers) bool {
	for _, server := range VCenterServers {
		if server.vm == DBId {
			return true
		}
	}

	return true
}
