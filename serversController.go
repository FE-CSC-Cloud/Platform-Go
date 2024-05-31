package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"sync"
	"time"
)

type DBServers struct {
	ID              int
	UsersId         string
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
	ID              int
	UsersId         string
	VcenterId       string
	Name            string
	Description     string
	EndDate         string
	OperatingSystem string
	Storage         int
	Memory          int
	IP              string
	PowerStatus     string
}

func getServers(c echo.Context) error {
	id := c.Param("id")
	SID, admin := getUserAssociatedWithJWT(c)
	session := getVCenterSession()
	serversFromVCenter := getPowerStatusFromvCenter(session, "")

	db, err := connectToDB()
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}
	defer db.Close()

	rows, err := getServersFromSQL(db, id, SID, admin)
	if err != nil {
		log.Fatal("Error executing query: ", err)
	}
	defer rows.Close()

	rowsArr, err := getPowerStatusRows(rows, serversFromVCenter)
	if err != nil {
		log.Fatal("Error scanning row: ", err)
	}

	if id != "" {
		if len(rowsArr) > 0 {
			return c.JSON(http.StatusOK, rowsArr[0])
		} else {
			return c.JSON(http.StatusNotFound, "No servers found for the given ID")
		}
	}

	return c.JSON(http.StatusOK, rowsArr)
}

func getServersFromSQL(db *sql.DB, id string, user string, admin bool) (*sql.Rows, error) {
	if id != "" && !admin {
		return db.Query("SELECT id, users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip FROM virtual_machines WHERE id = ? and users_id = ?", id, user)
	} else if id != "" && admin {
		return db.Query("SELECT id, users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip FROM virtual_machines WHERE id = ?", id)
	} else if admin {
		return db.Query("SELECT id, users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip FROM virtual_machines")
	} else {
		return db.Query("SELECT id, users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip FROM virtual_machines WHERE users_id = ?", user)
	}
}

func getPowerStatusRows(rows *sql.Rows, serversFromVCenter []vCenterServers) ([]PowerStatusReturn, error) {
	var rowsArr []PowerStatusReturn
	var wg sync.WaitGroup
	rowChan := make(chan PowerStatusReturn)

	for rows.Next() {
		var s PowerStatusReturn
		err := rows.Scan(&s.ID, &s.UsersId, &s.VcenterId, &s.Name, &s.Description, &s.EndDate, &s.OperatingSystem, &s.Storage, &s.Memory, &s.IP)
		if err != nil {
			return nil, err
		}

		wg.Add(1)
		go func(s PowerStatusReturn) {
			defer wg.Done()
			s.PowerStatus = getVCenterPowerState(s.VcenterId, serversFromVCenter)
			rowChan <- s
		}(s)
	}

	go func() {
		wg.Wait()
		close(rowChan)
	}()

	for row := range rowChan {
		rowsArr = append(rowsArr, row)
	}

	return rowsArr, nil
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

	UserId, _ := getUserAssociatedWithJWT(c)

	var vCenterID = createvCenterVM(session, UserId, json.Name, json.OperatingSystem)

	// Insert the new server into the database
	stmt, err := db.Prepare("INSERT INTO virtual_machines(users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal("Error preparing statement: ", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(UserId, vCenterID, json.Name, json.Description, endDate, json.OperatingSystem, json.Storage, json.Memory, "")
	if err != nil {
		log.Fatal("Error executing statement: ", err)
	}

	return c.JSON(http.StatusCreated, "Server gemaakt!")
}

func deleteServer(c echo.Context) error {
	id := c.Param("id")
	db, err := connectToDB()
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	// get the vCenter ID from the database
	var vCenterID string
	err = db.QueryRow("SELECT vcenter_id FROM virtual_machines WHERE id = ?", id).Scan(&vCenterID)
	if err != nil {
		log.Fatal("Error getting vCenter ID: ", err)
	}

	// delete the server from vCenter
	session := getVCenterSession()
	status := deletevCenterVM(session, vCenterID)

	if !status {
		return c.JSON(http.StatusBadRequest, "Error deleting server from vCenter")
	}

	// Prepare statement for deleting data
	stmt, err := db.Prepare("DELETE FROM virtual_machines WHERE id = ?")
	if err != nil {
		log.Fatal("Error preparing statement: ", err)
	}

	_, err = stmt.Exec(id)
	if err != nil {
		log.Fatal("Error executing statement: ", err)
	}

	return c.JSON(http.StatusCreated, "Server deleted!")
}
