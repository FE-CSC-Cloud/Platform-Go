package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
)

func createIPHostInSopohos(ip string) {
	log.Println("Creating IP host in Sophos")

	resp := doAuthenticatedSophosRequest("")
	log.Println(resp.Status)
}

func doAuthenticatedSophosRequest(xml string) *http.Response {
	var requestXML string = fmt.Sprintf(`
            <Request>
                <Login>
                    <Username>%s</Username>
                    <Password>%s</Password>
                </Login>
                %s
            </Request>`, getEnvVar("SOPHOS_FIREWALL_USER"), getEnvVar("SOPHOS_FIREWALL_PASS"), xml)

	firewallURL := getEnvVar("SOPHOS_FIREWALL_URL")

	// Create a new HTTP client with disabled SSL verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: getBoolEnvVar("VERIFY_TLS")},
	}
	client := &http.Client{Transport: tr}

	// Send the GET request
	resp, err := client.Get(firewallURL + requestXML)
	if err != nil {
		log.Println("Error sending request to Sophos: ", err)
		return nil
	}
	defer resp.Body.Close()

	return resp
}

func findEmptyIp() string {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return ""
	}

	var ip string
	err = db.QueryRow("SELECT ip FROM ip_adresses WHERE virtual_machine_id IS NULL LIMIT 1").Scan(&ip)
	if err != nil {
		log.Println("Error executing query: ", err)
		return ""
	}

	return ip
}
