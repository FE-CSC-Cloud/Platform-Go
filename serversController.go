package main

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"strings"
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

type jsonBody struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	OperatingSystem string `json:"operating_system"`
	EndDate         string `json:"end_date"`
	Storage         int    `json:"storage"`
	Memory          int    `json:"memory"`
}

func getServers(c echo.Context) error {
	id := c.Param("id")
	UserId, admin, _, _ := getUserAssociatedWithJWT(c)
	session := getVCenterSession()
	serversFromVCenter := getPowerStatusFromvCenter(session, "")

	db, err := connectToDB()
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}
	defer db.Close()

	rows, err := getServersFromSQL(db, id, UserId, admin)
	if err != nil {
		log.Fatal("Error executing query: ", err)
	}
	defer rows.Close()

	ip := findEmptyIp()
	if ip == "" {
		return c.JSON(http.StatusBadRequest, "No IP addresses available")
	}

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
	_, _, name, StudentID := getUserAssociatedWithJWT(c)

	nameWithoutSpace := strings.ReplaceAll(name, " ", "-")

	parseAndSetIpListForSophos()
	err := createIPHostInSopohos("145.89.192.231", nameWithoutSpace, StudentID)
	if err != nil {
		log.Println("Error creating IP host: ", err)
		return c.JSON(http.StatusInternalServerError, "Kon server niet aan firewall toevoegen")
	}

	err = createSophosFirewallRules(StudentID, nameWithoutSpace)
	if err != nil {
		log.Println("Error creating IP host: ", err)
		return c.JSON(http.StatusInternalServerError, "Kon server niet aan firewall toevoegen")
	}

	err = updateFirewallRuleGroupInSophos(StudentID, nameWithoutSpace)
	if err != nil {
		log.Println("Error creating IP host: ", err)
		return c.JSON(http.StatusInternalServerError, "Kon server niet aan firewall toevoegen")
	}

	/*	json := new(jsonBody)
		err := c.Bind(&json)
		if err != nil {
			log.Println("Error binding JSON: ", err)
			return c.JSON(http.StatusBadRequest, "Invalid JSON")
		}

		db, err := connectToDB()
		if err != nil {
			log.Fatal("Error connecting to database: ", err)
		}

		session := getVCenterSession()

		valid, errMessage, endDate := validateServerCreation(json, session)
		if !valid {
			return c.JSON(http.StatusBadRequest, errMessage)
		}

		UserId, _ := getUserAssociatedWithJWT(c)

		serverAlreadyExists := checkIfUserAlreadyHasServerWithName(json.name, UserId, db)
		if serverAlreadyExists {
			return c.JSON(http.StatusOK, "Deze naam bestaat al voor jouw!")
		}

		err = createServerInDB(UserId, json, endDate, session, db)
		if err != nil {
			log.Fatal("Error creating server: ", err)
		}*/

	return c.JSON(http.StatusCreated, "Server gemaakt!")
}

func createServerInDB(UserId string, json *jsonBody, endDate time.Time, session string, db *sql.DB) error {
	var vCenterID = createvCenterVM(session, UserId, json.Name, json.OperatingSystem)

	// Insert the new server into the database
	stmt, err := db.Prepare("INSERT INTO virtual_machines(users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(UserId, vCenterID, json.Name, json.Description, endDate, json.OperatingSystem, json.Storage, json.Memory, "")
	if err != nil {
		return err
	}

	return nil
}

func validateServerCreation(json *jsonBody, session string) (bool, string, time.Time) {
	// check if the date is in the correct format (YYYY-MM-DD)
	var endDate, errDate = time.Parse("2006-01-02", json.EndDate)
	if errDate != nil {
		return false, "Invalid date format, please use YYYY-MM-DD", time.Time{}
	}

	// check if the end date is in the future
	if endDate.Before(time.Now()) {
		return false, "End date in the past", time.Time{}
	}

	// check if the OS exist
	templates := getTemplatesFromVCenter(session)
	if !checkIfItemIsKeyOfArray(json.OperatingSystem, templates) {
		return false, "Invalid operating system", time.Time{}
	}

	return true, "", endDate
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

func checkIfUserAlreadyHasServerWithName(name, userID string, db *sql.DB) bool {
	rows, err := db.Query("SELECT name FROM virtual_machines WHERE name = ? AND users_id = ?", name, userID)
	if err != nil {
		log.Println("Could not check if the nane already exists err: ", err)
		return true
	}

	if !rows.Next() {
		return false
	}

	return true
}
