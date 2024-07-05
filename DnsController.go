package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"strconv"
	"strings"
)

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
	var zones []Zone
	for _, zone := range dnsResponse.Response.Zones {
		if !zone.Internal {
			zones = append(zones, zone)
		}
	}

	dnsResponse.Response.Zones = zones

	return c.JSON(200, dnsResponse)
}
func CreateDnsRecord(c echo.Context) error {
	type RequestBody struct {
		Subdomain   string `json:"subdomain"`
		Parent      string `json:"parent"`
		RecordValue string `json:"record_value"`
		Ttl         string `json:"ttl"`
		Type        string `json:"type"`
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

	db, err := connectToDB()

	rows, err := getRecordsForServerFromDB(db, serverId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "could not fetch records from database")
	}

	var parentSubdomains []string
	var subdomains []string

	for rows.Next() {
		var (
			VM                                         int
			parent, subdomain, recordType, recordValue string
		)
		err = rows.Scan(&VM, &parent, &subdomain, &recordType, &recordValue)
		if err != nil {
			log.Println("Error scanning rows: ", err)
			return c.JSON(http.StatusInternalServerError, "could not scan rows")
		}

		subdomains = append(subdomains, subdomain)
	}

	if len(subdomains) > 0 {
		// check how many records are in the zone
		records, err := getRecordsInTechnitium(request.Parent, "", true)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, "could not fetch records in zone")
		}

		for _, subdomain := range subdomains {
			if !strings.Contains(subdomain, ".") {
				parentSubdomains = append(parentSubdomains, subdomain)
			}
		}

		log.Println("records: ", records)
		log.Println("parentSubdomains: ", parentSubdomains)
	}

	errDNS := createRecordInDNS(request.Parent, request.Subdomain, request.Ttl, request.Type, request.RecordValue)
	if errDNS != "" {
		return c.JSON(http.StatusInternalServerError, errDNS)
	}

	err = createRecordForSubInDB(request.Parent, request.Subdomain, serverId, request.Type, request.RecordValue)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "could not create record in database")
	}

	return c.JSON(http.StatusOK, "record created")
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
			VM                                         int
			parent, subdomain, recordType, recordValue string
		)
		err = rows.Scan(&VM, &parent, &subdomain, &recordType, &recordValue)
		if err != nil {
			log.Println("Error scanning rows: ", err)
			return c.JSON(http.StatusInternalServerError, "could not scan rows")
		}

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
	rows, err := db.Query("SELECT virtual_machines_id, parent_domain, subdomain, record_type, record_value FROM sub_domains WHERE virtual_machines_id = ?", serverId)
	if err != nil {
		log.Println("Error fetching records from database: ", err)
		return nil, err
	}

	return rows, nil
}

func createRecordInDNS(zone, domain, ttl, recordType, recordValue string) string {
	queries := [][]string{
		{"zone", zone},
		{"domain", domain + "." + zone},
		{"overwrite", "false"},
		{"ttl", ttl},
		{"type", recordType},
	}

	toQueries, err := appendRecordValueWithCorrectTypeToQueries(queries, recordType, recordValue)

	_, err = authenticatedDNSRequest("zones/records/add", toQueries)
	if err != nil {
		log.Println("Error creating record in DNS: ", err)
		return "internal server error"
	}

	return ""
}

func deleteRecordInDNS(zone, domain, recordType, recordValue string) (string, error) {
	queries := [][]string{
		{"zone", zone},
		{"domain", domain + "." + zone},
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
