package main

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
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

type DNSResponse struct {
	Response Response `json:"response"`
	Status   string   `json:"status"`
}

func GetDnsZones(c echo.Context) error {
	body, err := authenticatedDNSRequest("zones/list", [][]string{})
	if err != nil {
		log.Println("Error fetching dns zones: ", err)
		return c.JSON(500, "could not fetch dns zones")
	}

	var dnsResponse DNSResponse

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
		RecordValue string `json:"recordValue"`
		Ttl         string `json:"ttl"`
		Type        string `json:"type"`
	}

	serverId := c.Param("serverId")

	request := new(RequestBody)
	if err := c.Bind(request); err != nil {
		log.Println("Error binding request: ", err)
		return c.JSON(http.StatusBadRequest, "could not bind request")
	}

	sid, isAdmin, _, _ := getUserAssociatedWithJWT(c)
	if !isAdmin {
		db, err := connectToDB()
		if err != nil {
			log.Println("Error connecting to database: ", err)
			return c.JSON(500, "could not connect to database")
		}
		if !checkIfServerBelongsToUser(sid, serverId, db) {
			c.JSON(http.StatusNotFound, "BOOOOOO")
		}
	}

	errDNS := createRecordInDNS(request.Parent, request.Subdomain, request.Ttl, request.Type, request.RecordValue)
	if errDNS != "" {
		return c.JSON(http.StatusInternalServerError, errDNS)
	}

	err := createRecordForSubInDB(request.Parent, request.Subdomain, serverId, request.Type, request.RecordValue)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "could not create record in database")
	}

	return c.JSON(http.StatusOK, "record created")
}

func DeleteDnsRecord(c echo.Context) error {
	type RequestBody struct {
		Subdomain   string `json:"subdomain"`
		Parent      string `json:"parent"`
		RecordValue string `json:"recordValue"`
		RecordType  string `json:"recordType"`
	}

	serverId := c.Param("serverId")

	request := new(RequestBody)
	if err := c.Bind(request); err != nil {
		log.Println("Error binding request: ", err)
		return c.JSON(http.StatusBadRequest, "could not bind request")
	}

	sid, isAdmin, _, _ := getUserAssociatedWithJWT(c)
	if !isAdmin {
		db, err := connectToDB()
		if err != nil {
			log.Println("Error connecting to database: ", err)
			return c.JSON(500, "could not connect to database")
		}
		if !checkIfServerBelongsToUser(sid, serverId, db) {
			c.JSON(http.StatusNotFound, "BOOOOOO")
		}
	}

	err := deleteRecordInDB(request.Parent, request.Subdomain, request.RecordType, request.RecordValue)
	if err != nil {
		return err
	}

	errDNS := deleteRecordInDNS(request.Parent, request.Subdomain, request.RecordType, request.RecordValue)
	if errDNS != "" {
		return c.JSON(http.StatusInternalServerError, errDNS)
	}

	return c.JSON(200, "record deleted")
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

func deleteRecordInDNS(zone, domain, recordType, recordValue string) string {
	queries := [][]string{
		{"zone", zone},
		{"domain", domain + "." + zone},
		{"type", recordType},
	}

	toQueries, err := appendRecordValueWithCorrectTypeToQueries(queries, recordType, recordValue)
	if err != nil {
		return ""
	}

	_, err = authenticatedDNSRequest("zones/records/delete", toQueries)
	if err != nil {
		log.Println("Error deleting record in DNS: ", err)
		return "internal server error"
	}

	return ""
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

func getRecordsInZone(zone string) ([]string, error) {
	body, err := authenticatedDNSRequest("zones/records/list", [][]string{{"zone", zone}})
	if err != nil {
		return nil, err
	}

	var dnsResponse DNSResponse

	err = json.Unmarshal(body, &dnsResponse)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var records []string
	for _, record := range dnsResponse.Response.Zones {
		records = append(records, record.Name)
	}

	return records, nil
}
