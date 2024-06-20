package main

import (
	"encoding/json"
	"fmt"
	"github.com/labstack/echo/v4"
	"log"
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
		ServerId    string `json:"serverId"`
		RecordValue string `json:"recordValue"`
		Ttl         string `json:"ttl"`
		Type        string `json:"type"`
	}

	request := new(RequestBody)
	if err := c.Bind(request); err != nil {
		return err
	}

	err := createRecordForSubInDB(request.Subdomain, request.Parent, request.ServerId)
	if err != nil {
		return c.JSON(500, "could not create record in database")
	}

	err = createRecordInDNS(request.Parent, request.Subdomain, request.Ttl, request.Type, request.RecordValue)
	if err != nil {
		return c.JSON(500, err)
	}

	return c.JSON(200, "record created")
}

func createRecordForSubInDB(subdomain, parent, VM string) error {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return err
	}
	// create a record for the subdomain in the database
	_, err = db.Exec("INSERT INTO subDomains (virtual_machines_id, parentDomain, subDomain) VALUES (?, ?, ?)", VM, parent, subdomain)
	if err != nil {
		log.Println("Error inserting subdomain in database: ", err)
		return err
	}
	return nil
}

func createRecordInDNS(zone, domain, ttl, recordType, recordValue string) error {
	queries := [][]string{
		{"zone", zone},
		{"domain", domain},
		{"overwrite", "false"},
		{"ttl", ttl},
		{"type", recordType},
	}

	switch recordType {
	case "A":
		{
			if !isIPv4(recordValue) {
				return fmt.Errorf("only ipv4 addresses are allowed for A records")
			}
			queries = append(queries, []string{"ipAddress", recordValue})
		}
	case "AAAA":
		{
			if !isIPv6(recordValue) {
				return fmt.Errorf("only ipv6 addresses are allowed for AAAA records")
			}
			queries = append(queries, []string{"ipAddress", recordValue})
		}
	case "CNAME":
		queries = append(queries, []string{"cname", recordValue})
	case "TXT":
		queries = append(queries, []string{"text", recordValue})
	case "DNAME":
		queries = append(queries, []string{"dname", recordValue})
	case "ANAME":
		queries = append(queries, []string{"aname", recordValue})
	default:
		return fmt.Errorf("invalid record type")
	}

	_, err := authenticatedDNSRequest("records/add", queries)
	if err != nil {
		log.Println("Error creating record in DNS: ", err)
		return fmt.Errorf("internal server error")
	}

	return nil
}
