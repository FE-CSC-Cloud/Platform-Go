package main

import (
	"database/sql"
	"fmt"
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
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	OperatingSystem string    `json:"operating_system"`
	EndDate         string    `json:"end_date"`
	Storage         int       `json:"storage"`
	Memory          int       `json:"memory"`
	HomeIPs         *[]string `json:"home_ips"`
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

func deleteServer(c echo.Context) error {
	id := c.Param("id")
	db, err := connectToDB()
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}

	// get the vCenter ID from the database
	var (
		vCenterID  string
		userID     string
		serverName string
	)

	err = db.QueryRow("SELECT vcenter_id, users_id, name FROM virtual_machines WHERE id = ?", id).Scan(&vCenterID, &userID, &serverName)
	if err != nil {
		log.Fatal("Error getting vCenter ID: ", err)
	}

	_, studentID, _, err := fetchUserInfoWithSID(userID)
	if err != nil {
		log.Fatal("Error fetching user info: ", err)
		return err
	}

	// delete the server from sophos
	err = removeFirewallFromServerInSophos(studentID, serverName)
	if err != nil {
		log.Println("Error removing firewall from sophos: ", err)
		return c.JSON(http.StatusBadRequest, "Error deleting server from sophos")
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

func powerServer(c echo.Context) error {
	id := c.Param("id")
	status := c.Param("status")
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

	session := getVCenterSession()
	status = strings.ToUpper(status)

	switch status {
	case "ON":
		{
			success := powerOn(session, vCenterID)
			if !success {
				return c.JSON(http.StatusBadRequest, "Error powering on server")
			}
		}
	case "OFF":
		{
			success := powerOff(session, vCenterID)
			if !success {
				return c.JSON(http.StatusBadRequest, "Error powering off server")
			}
		}
	case "FORCE_OFF":
		{
			success := forcePowerOff(session, vCenterID)
			if !success {
				return c.JSON(http.StatusBadRequest, "Error powering off server")
			}
		}
	default:
		return c.JSON(http.StatusBadRequest, "Invalid status")
	}

	return c.JSON(http.StatusCreated, "Server powered "+status)

}

func createServer(c echo.Context) error {
	json := new(jsonBody)
	err := c.Bind(&json)
	if err != nil {
		log.Println("Error binding JSON: ", err)
		return c.JSON(http.StatusBadRequest, "Invalid JSON")
	}

	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
	}

	session := getVCenterSession()
	serverCreationStep := ""

	valid, errMessage, endDate := validateServerCreation(json, session)
	if !valid {
		return c.JSON(http.StatusBadRequest, errMessage)
	}

	UserId, _, _, studentID := getUserAssociatedWithJWT(c)

	serverAlreadyExists := checkIfUserAlreadyHasServerWithName(json.Name, UserId, db)
	if serverAlreadyExists {
		return c.JSON(http.StatusOK, "You're already using this name!")
	}

	ip := findEmptyIp()
	if ip == "" {
		return c.JSON(http.StatusBadRequest, "No IP addresses available")
	}

	err = createServerInDB(UserId, json, endDate, session, db)
	if err != nil {
		log.Println("Error creating server: ", err)
	}
	serverCreationStep = "made in db"

	go func() {
		var vCenterID, err = createvCenterVM(session, UserId, json.Name, json.OperatingSystem)
		err = updateServerWithVCenterID(vCenterID, json.Name, UserId, db)
		if err != nil {
			logErrorInDB(err)
			handleFailedCreation(json.Name, UserId, studentID, "", serverCreationStep, db)
			log.Println("Error updating or making server: ", err)
			log.Println("vCenter ID: ", vCenterID)
			return
		}
		serverCreationStep = "made in vCenter"

		err = createFirewallRuleForServerCreation(ip, studentID, json.Name)
		if err != nil {
			logErrorInDB(err)
			handleFailedCreation(json.Name, UserId, studentID, vCenterID, serverCreationStep, db)
			log.Println("Error creating firewall rules: ", err)
			return
		}

		serverCreationStep = "made in sophos"

		err = addUsersToFirewall(studentID, *json)
		if err != nil {
			logErrorInDB(err)
			handleFailedCreation(json.Name, UserId, studentID, vCenterID, serverCreationStep, db)
			log.Println("Error adding ips to firewall: ", err)
			return
		}

		powerOn(session, vCenterID)
	}()

	return c.JSON(http.StatusCreated, "Server is being made!")
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

	// remove spaces from the name
	json.Name = strings.ReplaceAll(json.Name, " ", "")

	return true, "", endDate
}

func createServerInDB(UserId string, json *jsonBody, endDate time.Time, session string, db *sql.DB) error {
	// Insert the new server into the database
	stmt, err := db.Prepare("INSERT INTO virtual_machines(users_id, vcenter_id, name, description, end_date, operating_system, storage, memory, ip) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(UserId, "", json.Name, json.Description, endDate, json.OperatingSystem, json.Storage, json.Memory, "")
	if err != nil {
		return err
	}

	return nil
}

func deleteServerFromDB(serverName, userId string, db *sql.DB) {
	// Prepare statement for deleting data
	stmt, err := db.Prepare("DELETE FROM virtual_machines WHERE name = ? and users_id = ?")
	if err != nil {
		log.Println("Error preparing statement: ", err)
	}

	_, err = stmt.Exec(serverName, userId)
	if err != nil {
		log.Println("Error executing statement: ", err)
	}

}

func updateServerWithVCenterID(vCenterID, name, userID string, db *sql.DB) error {
	// Update the vCenter ID in the database
	stmt, err := db.Prepare("UPDATE virtual_machines SET vcenter_id = ? WHERE name = ? and users_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(vCenterID, name, userID)
	if err != nil {
		return err
	}

	return nil
}

func createFirewallRuleForServerCreation(ip, studentID, serverName string) error {
	defer timeTrack(time.Now(), "createFirewallRuleForServerCreation")
	parseAndSetIpListForSophos()
	err := createIPHostInSopohos(ip, studentID, serverName)
	if err != nil {
		log.Println("Error creating IP host: ", err)
		return err
	}
	err = createSophosFirewallRules(studentID, serverName)
	if err != nil {
		removeIPHostInSophos(studentID, serverName)

		log.Println("Error creating firewall rules: ", err)
		return err
	}

	err = updateFirewallRuleGroupInSophos(studentID, serverName)
	if err != nil {
		log.Println("Error updating rule group: ", err)

		removeIPHostInSophos(studentID, serverName)
		removeInBoundRuleInSophos(studentID, serverName)
		removeOutBoundRuleInSophos(studentID, serverName)

		return err
	}

	return nil
}

func addUsersToFirewall(studentID string, json jsonBody) error {
	ipHost, err := getSophosIpHost()
	if err != nil {
		return err
	}

	// Check how many IP's are already in sophos belonging to the student
	userHasIPsWhitelisted := strings.Count(ipHost, studentID)

	// Check if the IP already exists in sophos
	for _, ip := range *json.HomeIPs {
		if strings.Contains(ipHost, ip) {
			return err
		}
	}

	for _, ip := range *json.HomeIPs {
		err := addIpToSophos(studentID, ip, userHasIPsWhitelisted)
		if err != nil {
			return err
		}
	}

	return nil
}

func handleFailedCreation(serverName, userId, studentId, vCenterId, serverCreationStep string, db *sql.DB) {
	logErrorInDB(fmt.Errorf("Server creation failed for server: " + serverName + " got stuck at: " + serverCreationStep))
	if serverCreationStep == "made in db" {
		deleteServerFromDB(serverName, studentId, db)
	}

	if serverCreationStep == "made in vCenter" {
		deleteServerFromDB(serverName, studentId, db)
		deletevCenterVM(getVCenterSession(), vCenterId)
	}

	if serverCreationStep == "made in sophos" {
		removeIPHostInSophos(studentId, serverName)
		removeInBoundRuleInSophos(studentId, serverName)
		removeOutBoundRuleInSophos(studentId, serverName)
	}

	createNotificationForUser(db, userId, "Server creation failed", "Server creation failed for server: "+serverName)
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
