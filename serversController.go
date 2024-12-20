package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
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

type serverCreationJsonBody struct {
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	OperatingSystem string    `json:"operating_system"`
	EndDate         string    `json:"end_date"`
	Storage         int       `json:"storage"`
	Memory          int       `json:"memory"`
	HomeIPs         *[]string `json:"home_ips"`
	SubDomain       *string   `json:"sub_domain"`
	DomainZone      *string   `json:"domain_zone"`
}

type startScript struct {
	User             string `json:"user"`
	Password         string `json:"password"`
	ScriptLocation   string `json:"scriptLocation"`
	ScriptExecutable string `json:"scriptExecutable"`
}

func GetServers(c echo.Context) error {
	id := c.Param("id")
	UserId, isAdmin, _, _ := getUserAssociatedWithJWT(c)
	session := getVCenterSession()
	serversFromVCenter := getPowerStatusFromvCenter(session, "")

	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
	}
	defer db.Close()

	rows, err := getServersFromSQL(db, id, UserId, isAdmin)
	if err != nil {
		log.Println("Error executing query: ", err)
	}
	defer rows.Close()

	RowsArr, err := getPowerStatusRows(rows, serversFromVCenter)
	if err != nil {
		log.Println("Error scanning row: ", err)
	}

	if id != "" {
		if len(RowsArr) > 0 {
			return c.JSON(http.StatusOK, RowsArr[0])
		} else {
			return c.JSON(http.StatusNotFound, "No servers found for the given ID")
		}
	}

	return c.JSON(http.StatusOK, RowsArr)
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

func DeleteServer(c echo.Context) error {
	id := c.Param("id")
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
	}

	idInt, err := strconv.Atoi(id)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Error converting ID to int")
	}

	// delete all the DNS records for the server
	err = deleteDNSRecordsForServer(idInt, db)
	if err != nil {
		log.Println("Error deleting DNS records for server: ", err)
		return c.JSON(http.StatusBadRequest, "Error deleting DNS records for server")
	}

	// get the vCenter ID from the database
	var (
		vCenterID  string
		serverName string
	)

	userID, isAdmin, _, studentID := getUserAssociatedWithJWT(c)

	if isAdmin {
		err = db.QueryRow("SELECT vcenter_id, name FROM virtual_machines WHERE id = ?", id).Scan(&vCenterID, &serverName)
	} else {
		err = db.QueryRow("SELECT vcenter_id, name FROM virtual_machines WHERE id = ? and users_id = ?", id, userID).Scan(&vCenterID, &serverName)
	}
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Can't find server with that ID")
	}

	// Prepare statement for deleting data
	stmt, err := db.Prepare("DELETE FROM virtual_machines WHERE id = ?")
	if err != nil {
		log.Println("Error preparing statement: ", err)
	}

	_, err = stmt.Exec(id)
	if err != nil {
		log.Println("Error executing statement: ", err)
		return c.JSON(http.StatusBadRequest, "Error deleting server from database")
	}

	err = unassignIPfromVM(vCenterID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Error unassigning IP from VM")
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

	return c.JSON(http.StatusCreated, "Server deleted!")
}

func PowerServer(c echo.Context) error {
	id := c.Param("id")
	status := c.Param("status")
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return c.JSON(http.StatusInternalServerError, "error in the program, please try again later")
	}

	userId, isAdmin, _, _ := getUserAssociatedWithJWT(c)

	// get the vCenter ID from the database
	var vCenterID string

	if isAdmin {
		err = db.QueryRow("SELECT vcenter_id FROM virtual_machines WHERE id = ?", id).Scan(&vCenterID)
	} else {
		err = db.QueryRow("SELECT vcenter_id FROM virtual_machines WHERE id = ? and users_id = ?", id, userId).Scan(&vCenterID)
	}
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Can't find server with that ID")
	}

	log.Println("vCenterID: ", vCenterID)

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
	case "RESET":
		{
			success := reset(session, vCenterID)
			if !success {
				return c.JSON(http.StatusBadRequest, "Error resetting server")
			}
		}
	default:
		return c.JSON(http.StatusBadRequest, "Invalid status")
	}

	return c.JSON(http.StatusCreated, "Server powered "+status)

}

func CreateServer(c echo.Context) error {
	jsonBody := new(serverCreationJsonBody)
	err := c.Bind(&jsonBody)
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

	if jsonBody.SubDomain != nil && jsonBody.DomainZone != nil {
		// parse the subdomain and domain zone to regular strings
		zone := *jsonBody.DomainZone
		subdomain := *jsonBody.SubDomain + "." + getEnvVar("SUBDOMAIN_PREFIX")

		// strip any dots from the subdomain to make sure it's a parent
		subdomain = strings.TrimSuffix(subdomain, ".")

		if subDomainInUse(zone, subdomain, db) {
			return c.JSON(http.StatusBadRequest, "This subdomain is already in use!")
		}
	}

	valid, errMessage, endDate := validateServerCreation(jsonBody, session)
	if !valid {
		return c.JSON(http.StatusBadRequest, errMessage)
	}

	UserId, isAdmin, fullName, studentID := getUserAssociatedWithJWT(c)

	globalLimit := getEnvVar("GLOBAL_SERVER_LIMIT")
	intGlobalLimit, _ := strconv.Atoi(globalLimit)

	if countServersByUser(UserId, db) >= intGlobalLimit && !isAdmin {
		return c.JSON(http.StatusBadRequest, "You already have 2 servers, you can't create more")
	}

	serverAlreadyExists := checkIfUserAlreadyHasServerWithName(jsonBody.Name, UserId, db)
	if serverAlreadyExists {
		return c.JSON(http.StatusConflict, "You're already using this name!")
	}

	ip := findEmptyIp()
	if ip == "" {
		return c.JSON(http.StatusBadRequest, "No IP addresses available")
	}

	err = claimIp(ip)
	if err != nil {
		return c.JSON(http.StatusBadRequest, "Error claiming IP")
	}

	err = createServerInDB(UserId, jsonBody, endDate, db)
	if err != nil {
		log.Println("Error creating server: ", err)
	}
	serverCreationStep = "made in db"

	go func() {
		var vCenterID, err = createvCenterVM(session, studentID, jsonBody.Name, jsonBody.OperatingSystem, jsonBody.Storage, jsonBody.Memory)
		err = updateServerWithVCenterID(vCenterID, jsonBody.Name, UserId, ip, db)
		if err != nil {
			logErrorInDB(err)
			handleFailedCreation(jsonBody.Name, UserId, studentID, "", serverCreationStep, ip, db)
			log.Println("Error updating or making server: ", err)
			log.Println("vCenter ID: ", vCenterID)
			return
		}
		serverCreationStep = "made in vCenter"

		err = createFirewallRuleForServerCreation(ip, studentID, jsonBody.Name)
		if err != nil {
			logErrorInDB(err)
			handleFailedCreation(jsonBody.Name, UserId, studentID, vCenterID, serverCreationStep, ip, db)
			log.Println("Error creating firewall rules: ", err)
			return
		}

		serverCreationStep = "made in sophos"

		err = assignIPToVM(ip, vCenterID)
		if err != nil {
			logErrorInDB(err)
			handleFailedCreation(jsonBody.Name, UserId, studentID, vCenterID, serverCreationStep, ip, db)
			log.Println("Error assigning IP to VM: ", err)
			return
		}

		serverCreationStep = "made in ip"

		err = addUsersToFirewall(studentID, *jsonBody)
		if err != nil {
			logErrorInDB(err)
			handleFailedCreation(jsonBody.Name, UserId, studentID, vCenterID, serverCreationStep, ip, db)
			log.Println("Error adding ips to firewall: ", err)
			return
		}

		powerOn(session, vCenterID)

		startScript, err := readStartScript(jsonBody.OperatingSystem)
		if err != nil {
			log.Println("Error running script in VM: ", err)
		}

		time.Sleep(30 * time.Second)

		// Get only the first name of the user
		firstName := strings.Split(fullName, " ")[0]

		err = runStartScript(session, startScript, firstName, studentID, vCenterID, ip, jsonBody.Name)
		if err != nil {
			logErrorInDB(err)
			handleFailedCreation(jsonBody.Name, UserId, studentID, vCenterID, serverCreationStep, ip, db)
			log.Println("Error running script in VM: ", err)
			return
		}

		_, _, _, studentEmail, err := fetchUserInfoWithSID(UserId)
		if err != nil {
			log.Println("Error fetching user info: ", err)
		}

		if jsonBody.SubDomain != nil && jsonBody.DomainZone != nil {
			// convert the subdomain and domain zone to regular strings
			zone := *jsonBody.DomainZone
			subdomain := *jsonBody.SubDomain + "." + getEnvVar("SUBDOMAIN_PREFIX")

			// strip any dots from the subdomain to make sure it's a parent
			subdomain = strings.TrimSuffix(subdomain, ".")

			// get the id of the server we just created
			var serverID string
			err = db.QueryRow("SELECT id FROM virtual_machines WHERE name = ? AND users_id = ? and vcenter_id = ?", jsonBody.Name, UserId, vCenterID).Scan(&serverID)
			if err != nil {
				log.Println("Error getting server ID: ", err)
				handleFailedCreation(jsonBody.Name, UserId, studentID, vCenterID, serverCreationStep, ip, db)
			}

			// create the DNS record
			err = createDNSRecord(db, subdomain, zone, ip, strconv.Itoa(3306), "A", serverID)
		}

		serverCreationSuccessTitle := "Server is gemaakt"
		serverCreationSuccessBody := "Je server(" + jsonBody.Name + ") is gemaakt met het ip: " + ip + " je gebruikersnaam is: " + firstName + " en je wachtwoord is: " + firstName + " verander dit aub zo snel mogelijk! Als je een subdomein hebt aangevraagd dan is deze ook aangemaakt kan je deze nu ook gebruiken."
		// check if the email is not empty
		createNotificationForUser(db, UserId, serverCreationSuccessTitle, serverCreationSuccessBody)
		if studentEmail != "" {
			sendEmailNotification(studentEmail, serverCreationSuccessTitle, serverCreationSuccessBody)
		}
	}()

	return c.JSON(http.StatusCreated, "Server is being made!")
}

func validateServerCreation(json *serverCreationJsonBody, session string) (bool, string, time.Time) {
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

	if json.Memory > 2 || json.Storage > 20 {
		return false, "NUH UH", time.Time{}
	}

	// remove spaces from the name
	json.Name = strings.ReplaceAll(json.Name, " ", "")

	return true, "", endDate
}

func createServerInDB(UserId string, json *serverCreationJsonBody, endDate time.Time, db *sql.DB) error {
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

func updateServerWithVCenterID(vCenterID, name, userID, ip string, db *sql.DB) error {
	// Update the vCenter ID in the database
	stmt, err := db.Prepare("UPDATE virtual_machines SET vcenter_id = ?, ip = ? WHERE name = ? and users_id = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(vCenterID, ip, name, userID)
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

func addUsersToFirewall(studentID string, json serverCreationJsonBody) error {
	ipHost, err := getSophosIpHost()
	if err != nil {
		return err
	}

	// Check how many IP's are already in sophos belonging to the student
	userHasIPsWhitelisted := strings.Count(ipHost, studentID)

	if json.HomeIPs == nil {
		return nil
	}

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

func readStartScript(templateName string) (startScript, error) {
	workingDir, err := os.Getwd()
	// check if the file exists
	file, err := os.Open(workingDir + "/startScripts/" + templateName + ".json")
	if err != nil {
		return startScript{}, err
	}

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return startScript{}, err
	}

	var script startScript
	err = json.Unmarshal(fileBytes, &script)
	if err != nil {
		return startScript{}, err
	}

	return script, nil
}

func handleFailedCreation(serverName, userId, studentId, vCenterId, serverCreationStep, ip string, db *sql.DB) {
	logErrorInDB(fmt.Errorf("Server creation failed for server: " + serverName + " got stuck at: " + serverCreationStep))
	err := unassignIPfromVM(vCenterId)
	if err != nil {
		log.Println("Error unassigning IP from VM: ", err)
	}

	if serverCreationStep == "made in db" {
		deleteServerFromDB(serverName, userId, db)
	}

	if serverCreationStep == "made in vCenter" {
		deleteServerFromDB(serverName, userId, db)
		deletevCenterVM(getVCenterSession(), vCenterId)
	}

	if serverCreationStep == "made in sophos" {
		deleteServerFromDB(serverName, userId, db)
		deletevCenterVM(getVCenterSession(), vCenterId)

		err := removeIPHostInSophos(studentId, serverName)
		if err != nil {
			log.Println("Error removing IP host in Sophos: ", err)
		}
		err = removeInBoundRuleInSophos(studentId, serverName)
		if err != nil {
			log.Println("Error removing inbound rule in Sophos: ", err)
		}
		err = removeOutBoundRuleInSophos(studentId, serverName)
		if err != nil {
			log.Println("Error removing outbound rule in Sophos: ", err)
		}
	}

	if serverCreationStep == "made in ip" {
		deleteServerFromDB(serverName, userId, db)
		deletevCenterVM(getVCenterSession(), vCenterId)

		err := removeIPHostInSophos(studentId, serverName)
		if err != nil {
			log.Println("Error removing IP host in Sophos: ", err)
		}
		err = removeInBoundRuleInSophos(studentId, serverName)
		if err != nil {
			log.Println("Error removing inbound rule in Sophos: ", err)
		}
		err = removeOutBoundRuleInSophos(studentId, serverName)
		if err != nil {
			log.Println("Error removing outbound rule in Sophos: ", err)
		}
	}

	userErrorTitle := "Error bij server maken"
	userErrorBody := "Server: " + serverName + " kon niet gemaakt worden probeer dit aub opnieuw met een andere naam. \n Als dit probleem zich blijft voordoen neem dan contact op met de beheerder"

	createNotificationForUser(db, userId, userErrorTitle, userErrorBody)
	_, _, _, studentEmail, _ := fetchUserInfoWithSID(userId)
	if studentEmail != "" {
		sendEmailNotification(studentEmail, userErrorTitle, userErrorBody)
	}
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

func checkIfServerExistsInDB(id string, db *sql.DB) bool {
	rows, err := db.Query("SELECT id FROM virtual_machines WHERE id = ?", id)
	if err != nil {
		log.Println("Could not check if the server exists err: ", err)
		return true
	}

	if !rows.Next() {
		return false
	}

	return true
}

func checkIfServerBelongsToUser(serverID, userID string, db *sql.DB) bool {
	rows, err := db.Query("SELECT id FROM virtual_machines WHERE id = ? AND users_id = ?", serverID, userID)
	if err != nil {
		log.Println("Could not check if the server belongs to the user err: ", err)
		return false
	}

	if !rows.Next() {
		return false
	}

	return true
}

func countServersByUser(userID string, db *sql.DB) int {
	rows, err := db.Query("SELECT id FROM virtual_machines WHERE users_id = ?", userID)
	if err != nil {
		log.Println("Could not count the servers by user err: ", err)
		return 0
	}

	var count int
	for rows.Next() {
		count++
	}

	return count
}
