package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

func createIPHostInSopohos(ip string) {
	log.Println("Creating IP host in Sophos")

	resp := doAuthenticatedSophosRequest("")
	log.Println(resp.Status)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Print the response body
	log.Println(string(body))

	defer resp.Body.Close()
}

func doAuthenticatedSophosRequest(xml string) *http.Response {
	var requestXML string = fmt.Sprintf(`
                    <Request>
                        <Login>
                            <Username>%s</Username> 
                            <Password>%s</Password>
                        </Login>%s
                    </Request>`,
		getEnvVar("SOPHOS_FIREWALL_USER"), getEnvVar("SOPHOS_FIREWALL_PASS"), xml)

	firewallURL := getEnvVar("SOPHOS_FIREWALL_URL")

	// Create a new HTTP client with disabled SSL verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !getBoolEnvVar("VERIFY_TLS")},
	}
	client := &http.Client{Transport: tr}

	log.Println("Sending request to Sophos: ", requestXML)
	// Create a new request
	req, err := http.NewRequest("POST", firewallURL, strings.NewReader(url.Values{"reqxml": {requestXML}}.Encode()))
	if err != nil {
		log.Println("Error creating request: ", err)
		return nil
	}

	// Set the content type to application/x-www-form-urlencoded
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request to Sophos: ", err)
		return nil
	}

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
