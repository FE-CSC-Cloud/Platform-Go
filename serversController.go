package main

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"strconv"
	"time"
)

type DBServers struct {
	ID              int
	UsersId         int
	VcenterId       string
	Name            string
	Description     string
	EndDate         string
	OperatingSystem string
	Storage         int
	Memory          int
	IP              string
}

type vCenterServers struct {
	Memory_size_MiB int    `json:"memory_size_MiB"`
	Vm              string `json:"vm"`
	Name            string `json:"name"`
	Power_state     string `json:"power_state"`
	Cpu_count       int    `json:"cpu_count"`
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
	// checkIfvCenterSessionIsExpired is pretty slow, might not be needed every time; rest is ~1ms
	var session string = getVCenterSession()

	var serversFromVCenter = getPowerStatusFromvCenter(session, "")

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

	var rowsArr []PowerStatusReturn
	for rows.Next() {
		var s PowerStatusReturn

		err = rows.Scan(&s.ID, &s.Users_id, &s.Vcenter_id, &s.Name, &s.Description, &s.End_date, &s.Operating_system, &s.Storage, &s.Memory, &s.IP)
		if err != nil {
			log.Fatal("Error scanning row: ", err)
		}

		s.Power_status = getVCenterPowerState(s.Vcenter_id, serversFromVCenter)

		rowsArr = append(rowsArr, s)
	}
	// return the result as a json object
	return c.JSON(http.StatusOK, rowsArr)
}

func getVCenterPowerState(DBId string, VCenterServers []vCenterServers) string {
	for _, server := range VCenterServers {
		if server.Vm == DBId {
			return server.Power_state
		}
	}

	return "UNKNOWN"
}

func createServer(c echo.Context) error {
	type jsonBody struct {
		Name            string `json:"name"`
		Description     string `json:"description"`
		OperatingSystem string `json:"operating_system"`
		EndDate         string `json:"end_date"`
		Storage         int    `json:"storage"`
		Memory          int    `json:"memory"`
	}

	session := getVCenterSession()
	db, err := connectToDB()
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	json := new(jsonBody)
	if err := c.Bind(&json); err != nil {
		return err
	}

	// check if the date is in the correct format (YYYY-MM-DD)
	var endDate, errDate = time.Parse("2006-01-02", json.EndDate)
	if errDate != nil {
		return c.JSON(http.StatusBadRequest, "Invalid date format, please use YYYY-MM-DD")
	}

	// check if the OS exist
	templates := getTemplatesFromVCenter(session)
	if checkIfItemIsKeyOfArray(json.OperatingSystem, templates) == false {
		return c.JSON(http.StatusBadRequest, "Invalid operating system")
	}

	var User_id int64 = 1

	var UserIdStr = strconv.FormatInt(User_id, 10)

	var vCenterID = createvCenterVM(session, UserIdStr, json.Name, json.OperatingSystem)

	// Insert the new server into the database
	stmt, err := db.Prepare("INSERT INTO virtual_machines(users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal("Error preparing statement: ", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(User_id, vCenterID, json.Name, json.Description, endDate, json.OperatingSystem, json.Storage, json.Memory, "")
	if err != nil {
		log.Fatal("Error executing statement: ", err)
	}

	return c.JSON(http.StatusCreated, "heuye")
}
