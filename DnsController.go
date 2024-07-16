package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
)

type RequestBodyServerCreation struct {
	Subdomain   string `json:"subdomain"`
	Parent      string `json:"parent"`
	RecordValue string `json:"record_value"`
	Ttl         string `json:"ttl"`
	Type        string `json:"type"`
}

type Zone struct {
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Internal        bool     `json:"internal"`
	DnssecStatus    string   `json:"dnssecStatus"`
	SoaSerial       int      `json:"soaSerial"`
	LastModified    string   `json:"lastModified"`
	Disabled        bool     `json:"disabled"`
	NotifyFailed    bool     `json:"notifyFailed,omitempty"`
	NotifyFailedFor []string `json:"notifyFailedFor,omitempty"`
}

type Response struct {
	Zones []Zone `json:"zones"`
}

type DNSZonesResponse struct {
	Response Response `json:"response"`
	Status   string   `json:"status"`
}

type Record struct {
	Name string `json:"name"`
	Type string `json:"type"`
	TTL  int    `json:"ttl"`
}

type RecordResponse struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	TTL   int    `json:"ttl"`
	RData struct {
		IpAddress string `json:"ipAddress"`
	} `json:"rData"`
	Disabled     bool   `json:"disabled"`
	DnssecStatus string `json:"dnssecStatus"`
	LastUsedOn   string `json:"lastUsedOn"`
}

type DNSResponse struct {
	Zone    Zone             `json:"zone"`
	Records []RecordResponse `json:"records"`
}

type DNSZoneRecordsResponse struct {
	Response DNSResponse `json:"response"`
	Status   string      `json:"status"`
}

func GetDnsZones(c echo.Context) error {
	body, err := authenticatedDNSRequest("zones/list", [][]string{})
	if err != nil {
		log.Println("Error fetching dns zones: ", err)
		return c.JSON(500, "could not fetch dns zones")
	}

	var dnsResponse DNSZonesResponse

	err = json.Unmarshal(body, &dnsResponse)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// remove internal zones
	var zones []string
	for _, zone := range dnsResponse.Response.Zones {
		if !zone.Internal {
			zones = append(zones, zone.Name)
		}
	}

	return c.JSON(200, zones)
}
func GetDnsRecordsForServer(c echo.Context) error {
	serverId := c.Param("serverId")

	if !userIsAllowedToaccessServer(serverId, c) {
		return c.JSON(http.StatusNotFound, "Server not found")
	}

	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return c.JSON(http.StatusInternalServerError, "could not connect to database")
	}

	rows, err := getRecordsForServerFromDB(db, serverId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "could not fetch records from database")
	}

	var records []map[string]string
	for rows.Next() {
		record := make(map[string]string)
		var (
			Id, VM                                     int
			parent, subdomain, recordType, recordValue string
		)
		err = rows.Scan(&Id, &VM, &parent, &subdomain, &recordType, &recordValue)
		if err != nil {
			log.Println("Error scanning rows: ", err)
			return c.JSON(http.StatusInternalServerError, "could not scan rows")
		}

		record["id"] = strconv.Itoa(Id)
		record["parent"] = parent
		record["subdomain"] = subdomain
		record["record_type"] = recordType
		record["record_value"] = recordValue
		records = append(records, record)
	}

	if len(records) == 0 {
		return c.JSON(http.StatusOK, "no records found")
	}

	return c.JSON(http.StatusOK, records)
}

func CreateDnsRecord(c echo.Context) error {
	serverId := c.Param("serverId")
	request := new(RequestBodyServerCreation)
	if err := c.Bind(request); err != nil {
		log.Println("Error binding request: ", err)
		return c.JSON(http.StatusBadRequest, "could not bind request")
	}

	// strip any training dots of the subdomain
	request.Subdomain = strings.TrimSuffix(request.Subdomain, ".")

	subdomainWithPrefix := strings.ToLower(request.Subdomain) + "." + getEnvVar("DOMAIN_PREFIX")

	if !userIsAllowedToaccessServer(serverId, c) {
		return c.JSON(http.StatusNotFound, "Server not found")
	}

	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return c.JSON(http.StatusInternalServerError, "could not connect to database")
	}

	err = createDNSRecord(db, subdomainWithPrefix, request.Parent, request.RecordValue, request.Ttl, request.Type, serverId)
	if err != nil {
		return c.JSON(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, "record created")
}

func UpdateDnsRecord(c echo.Context) error {
	// TODO: ACTUALLY UPDATE THE RECORD IN THE DATABASE
	recordId := c.Param("recordId")
	request := new(RequestBodyServerCreation)
	if err := c.Bind(request); err != nil {
		log.Println("Error binding request: ", err)
		return c.JSON(http.StatusBadRequest, "could not bind request")
	}

	// get record from database
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return c.JSON(http.StatusInternalServerError, "could not connect to database")
	}

	var (
		VM                                         int
		parent, subdomain, recordType, recordValue string
	)

	err = db.QueryRow("SELECT virtual_machines_id, parent_domain, subDomain, record_type, record_value FROM sub_domains WHERE id = ?", recordId).Scan(&VM, &parent, &subdomain, &recordType, &recordValue)
	if err != nil {
		log.Println("Error fetching record from database: ", err)
		return c.JSON(http.StatusInternalServerError, "could not fetch record from database")
	}

	if !userIsAllowedToaccessServer(strconv.Itoa(VM), c) {
		return c.JSON(http.StatusNotFound, "Server not found")
	}

	if !checkIfUserAlreadyHasRecordInDB(db, parent, subdomain, recordType, recordValue) {
		return c.JSON(http.StatusBadRequest, "record does not exist")
	}

	err = deleteRecordInDB(request.Parent, request.Subdomain, request.Type, request.RecordValue)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "could not delete record in database")
	}

	_, err = deleteRecordInDNS(request.Parent, request.Subdomain, request.Type, request.RecordValue)
	if err != nil {
		log.Println("Error deleting record in DNS: ", err)
		return c.JSON(http.StatusInternalServerError, "could not delete record in DNS")
	}

	err = createRecordInDNS(request.Parent, request.Subdomain, request.Ttl, request.Type, request.RecordValue)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, err)
	}

	err = createRecordForSubInDB(request.Parent, request.Subdomain, strconv.Itoa(VM), request.Type, request.RecordValue)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "could not create record in database")
	}

	return c.JSON(http.StatusOK, "record updated")

}

func DeleteDnsRecord(c echo.Context) error {
	type RequestBody struct {
		Subdomain   string `json:"subdomain"`
		Parent      string `json:"parent"`
		RecordValue string `json:"record_value"`
		RecordType  string `json:"type"`
	}

	serverId := c.Param("serverId")

	request := new(RequestBody)
	if err := c.Bind(request); err != nil {
		log.Println("Error binding request: ", err)
		return c.JSON(http.StatusBadRequest, "could not bind request")
	}

	if !userIsAllowedToaccessServer(serverId, c) {
		return c.JSON(http.StatusNotFound, "Server not found")
	}

	err := deleteRecordInDB(request.Parent, request.Subdomain, request.RecordType, request.RecordValue)
	if err != nil {
		return err
	}

	_, err = deleteRecordInDNS(request.Parent, request.Subdomain, request.RecordType, request.RecordValue)
	if err != nil {
		log.Println("Error deleting record in DNS: ", err)
		return c.JSON(http.StatusInternalServerError, "could not delete record in DNS")
	}

	return c.JSON(200, "record deleted")
}

func createDNSRecord(db *sql.DB, subdomain, zone, value, ttl, recordType, serverId string) error {
	userIsAllowed, err := userIsAllowedToMakeNewDNSRecord(db, serverId, subdomain, zone, recordType, value)
	if !userIsAllowed {
		return err
	}

	err = createRecordForSubInDB(zone, subdomain, serverId, recordType, value)
	if err != nil {
		return fmt.Errorf("could not create record in database")
	}

	errDNS := createRecordInDNS(zone, subdomain, ttl, recordType, value)
	if errDNS != nil {
		return errDNS
	}

	return nil
}

func userIsAllowedToMakeNewDNSRecord(db *sql.DB, serverId, subdomain, zone, recordType, value string) (bool, error) {
	topLevelSubdomain := getTopLevelSubDomain(subdomain)

	subdomainParts := strings.Split(subdomain, ".")
	if len(subdomainParts) > 2 {
		topLevelSubdomain = strings.Join(subdomainParts[len(subdomainParts)-2:], ".")
	} else {
		topLevelSubdomain = subdomainParts[0]
	}

	domainInUse := subDomainInUse(zone, topLevelSubdomain, db)
	if domainInUse {
		if !userOwnsDomain(zone, topLevelSubdomain, serverId, db) {
			return false, fmt.Errorf("domain is already in use and you do not own it")
		}
	}

	if checkIfUserAlreadyHasRecordInDB(db, zone, subdomain, recordType, value) {
		return false, fmt.Errorf("record already exists")
	}

	userIsAllowedToMakeDomain, err := checkIfUserDoesNotHaveMoreThen2SubSubdomains(db, serverId, subdomain)
	if err != nil {
		return false, err
	}
	if !userIsAllowedToMakeDomain {
		return false, fmt.Errorf("you are not allowed to create more than 2 sub-subdomains")
	}

	return true, nil
}

func createRecordForSubInDB(parent, subdomain, VM, recordType, recordValue string) error {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return err
	}
	// create a record for the subdomain in the database
	_, err = db.Exec("INSERT INTO sub_domains (virtual_machines_id, parent_domain, subDomain, record_type, record_value) VALUES (?, ?, ?, ?, ?)", VM, parent, subdomain, recordType, recordValue)
	if err != nil {
		log.Println("Error inserting subdomain in database: ", err)
		return err
	}
	return nil
}

func getRecordsForServerFromDB(db *sql.DB, serverId string) (*sql.Rows, error) {
	rows, err := db.Query("SELECT id ,virtual_machines_id, parent_domain, subdomain, record_type, record_value FROM sub_domains WHERE virtual_machines_id = ?", serverId)
	if err != nil {
		log.Println("Error fetching records from database: ", err)
		return nil, err
	}

	return rows, nil
}

func createRecordInDNS(zone, domain, ttl, recordType, recordValue string) error {
	queries := [][]string{
		{"zone", zone},
		{"domain", domain + "." + zone},
		{"overwrite", "false"},
		{"ttl", ttl},
		{"type", recordType},
	}

	toQueries, err := appendRecordValueWithCorrectTypeToQueries(queries, recordType, recordValue)
	if err != nil {
		return err
	}

	_, err = authenticatedDNSRequest("zones/records/add", toQueries)
	if err != nil {
		log.Println("Error creating record in DNS: ", err)
		return fmt.Errorf("internal server error")
	}

	return nil
}

func deleteRecordInDNS(zone, domain, recordType, recordValue string) (string, error) {
	queries := [][]string{
		{"zone", zone},
		{"domain", domain + zone},
		{"type", recordType},
	}

	toQueries, err := appendRecordValueWithCorrectTypeToQueries(queries, recordType, recordValue)
	if err != nil {
		return "", err
	}

	_, err = authenticatedDNSRequest("zones/records/delete", toQueries)
	if err != nil {
		return "", err
	}

	return "", nil
}

func deleteRecordInDB(parent, subdomain, recordType, recordValue string) error {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return err
	}

	_, err = db.Exec("DELETE FROM sub_domains WHERE parent_domain = ? AND subDomain = ? AND record_type = ? AND record_value = ?", parent, subdomain, recordType, recordValue)
	if err != nil {
		log.Println("Error deleting record from database: ", err)
		return err
	}

	return nil
}

func splitRecordValue(recordValue string) []string {
	return strings.Split(recordValue, " ")
}

func userIsAllowedToaccessServer(serverId string, c echo.Context) bool {
	sid, isAdmin, _, _ := getUserAssociatedWithJWT(c)
	if isAdmin {
		db, _ := connectToDB()
		return checkIfServerExistsInDB(serverId, db)
	} else {
		db, err := connectToDB()
		if err != nil {
			log.Println("Error connecting to database: ", err)
			return false
		}
		if !checkIfServerBelongsToUser(serverId, sid, db) {
			return checkIfServerBelongsToUser(serverId, sid, db)
		}
	}

	return false
}

func appendRecordValueWithCorrectTypeToQueries(queries [][]string, recordType, recordValue string) ([][]string, error) {

	switch recordType {
	case "A":
		{
			if !isIPv4(recordValue) {
				return queries, fmt.Errorf("only ipv4 addresses are allowed for A records")
			}
			queries = append(queries, []string{"ipAddress", recordValue})
		}
	case "MX":
		{
			// split the record value into priority and mail server with spaces
			split := splitRecordValue(recordValue)
			if len(split) != 2 {
				return queries, fmt.Errorf("invalid record value, only 2 values split by spaces are allowed for MX records")
			}
			queries = append(queries, []string{"preference", split[0]}, []string{"exchange", split[1]})
		}
	case "SRV":
		{
			// split the record value into priority, weight, port and target with spaces
			split := splitRecordValue(recordValue)
			if len(split) != 4 {
				return queries, fmt.Errorf("invalid record value, only 4 values split by spaces are allowed for SRV records")
			}
			queries = append(queries, []string{"priority", split[0]}, []string{"weight", split[1]}, []string{"port", split[2]}, []string{"target", split[3]})
		}
	case "CAA":
		{
			// split the record value into flags, tag and value with spaces
			split := splitRecordValue(recordValue)
			if len(split) != 3 {
				return queries, fmt.Errorf("invalid record value, only 3 values split by spaces are allowed for CAA records")
			}

			queries = append(queries, []string{"flags", split[0]}, []string{"tag", split[1]}, []string{"value", split[2]})
		}
	case "AAAA":
		queries = append(queries, []string{"ipAddress", recordValue})
	case "PTR":
		queries = append(queries, []string{"ptrName", recordValue})
	case "CNAME":
		queries = append(queries, []string{"cname", recordValue})
	case "TXT":
		queries = append(queries, []string{"text", recordValue})
	case "DNAME":
		queries = append(queries, []string{"dname", recordValue})
	case "ANAME":
		queries = append(queries, []string{"aname", recordValue})
	default:
		return queries, fmt.Errorf("invalid record type")
	}

	return queries, nil
}
func getRecordsInTechnitium(zone, subDomain string, listZone bool) ([]string, error) {
	var domain string
	if subDomain == "" {
		domain = zone
	} else {
		domain = subDomain + "." + zone
	}
	body, err := authenticatedDNSRequest("zones/records/get", [][]string{{"zone", zone}, {"domain", domain}, {"listZone", strconv.FormatBool(listZone)}})
	if err != nil {
		return nil, err
	}

	var dnsResponse DNSZoneRecordsResponse

	err = json.Unmarshal(body, &dnsResponse)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var records []string
	for _, record := range dnsResponse.Response.Records {
		// TODO: record value toevoegen aan de records
		records = append(records, []string{record.Name, record.Type}...)
	}

	return records, nil
}

func checkIfUserAlreadyHasRecordInDB(db *sql.DB, parent, subdomain, recordType, recordValue string) bool {
	rows, err := db.Query("SELECT * FROM sub_domains WHERE parent_domain = ? AND subDomain = ? AND record_type = ? AND record_value = ?", parent, subdomain, recordType, recordValue)
	if err != nil {
		log.Println("Error checking if user already has record in database: ", err)
		return false
	}

	if rows.Next() {
		return true
	}

	return false
}

func checkIfUserDoesNotHaveMoreThen2SubSubdomains(db *sql.DB, serverId, subdomainWithPrefix string) (bool, error) {
	rows, err := getRecordsForServerFromDB(db, serverId)
	if err != nil {
		return false, err
	}

	var parentSubdomains []string
	var subdomains []string

	for rows.Next() {
		var (
			Id, VM                                     int
			parent, subdomain, recordType, recordValue string
		)
		err = rows.Scan(&Id, &VM, &parent, &subdomain, &recordType, &recordValue)
		if err != nil {
			log.Println("Error scanning rows: ", err)
			return false, err
		}

		subdomains = append(subdomains, subdomain)
	}

	if len(subdomains) > 0 {
		for _, subdomain := range subdomains {
			// check if the domain had > 1 . in it so we know it's a subsubdomain
			if strings.Count(subdomain, ".") == 1 {
				parentSubdomains = append(parentSubdomains, subdomain)
			}
		}

		// explode the subdomainWithPrefix at the dots
		explodedSubdomain := strings.Split(subdomainWithPrefix, ".")

		if len(explodedSubdomain) > 2 {
			// get the last 2 elements of the exploded subdomain
			lastTwoElements := explodedSubdomain[len(explodedSubdomain)-2] + "." + explodedSubdomain[len(explodedSubdomain)-1]

			if !slices.Contains(parentSubdomains, lastTwoElements) {
				return false, nil
			}
		}
	}

	return true, nil
}

func getTopLevelSubDomain(subdomain string) string {
	subdomainParts := strings.Split(subdomain, ".")
	if len(subdomainParts) > 2 {
		return strings.Join(subdomainParts[len(subdomainParts)-2:], ".")
	} else {
		return subdomainParts[0]
	}
}

func subDomainInUse(parent, subdomain string, db *sql.DB) bool {
	rows, err := db.Query(`SELECT id FROM sub_domains where parent_domain = ? AND subDomain = ?`, parent, subdomain)
	if rows.Next() {
		return true
	}
	if err != nil {
		log.Println("Error checking for domain ownership: ", err)
		return true
	}

	return false
}

func userOwnsDomain(parent, subdomain, serverId string, db *sql.DB) bool {
	var domainOwnershipExists bool

	serverIdInt, err := strconv.Atoi(serverId)

	err = db.QueryRow(`
    SELECT EXISTS(
        SELECT 1 
        FROM sub_domains
        WHERE parent_domain = ? 
        AND (subDomain = ? OR subDomain = ?) 
        AND virtual_machines_id = ?
    )`, parent, subdomain, subdomain+"."+getEnvVar("DOMAIN_PREFIX"), serverIdInt).Scan(&domainOwnershipExists)
	if err != nil {
		log.Println("Error checking for domain ownership: ", err)
		return false
	}

	return domainOwnershipExists
}

func deleteDNSRecordsForServer(serverID int, db *sql.DB) error {
	rows, err := db.Query("SELECT parent_domain, subDomain, record_type, record_value FROM sub_domains WHERE virtual_machines_id = ?", serverID)
	if err != nil {
		log.Println("Error fetching records from database: ", err)
		return err
	}

	for rows.Next() {
		var (
			parent, subdomain, recordType, recordValue string
		)
		err = rows.Scan(&parent, &subdomain, &recordType, &recordValue)

		err := deleteRecordInDB(parent, subdomain, recordType, recordValue)
		if err != nil {
			return err
		}

		_, err = deleteRecordInDNS(parent, subdomain, recordType, recordValue)
		if err != nil {
			log.Println("Error deleting record in DNS: ", err)
			return err
		}
	}

	return nil
}
