package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func createVCenterHTTPClient() *http.Client {
	// Create a Transport for our client so we can skip SSL verification because the vCenter certificate is self-signed
	tlsConfig := &tls.Config{InsecureSkipVerify: !getBoolEnvVar("VERIFY_TLS")}
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	// Create client with the Transport that can skip SSL verification if needed
	client := &http.Client{Transport: transport}

	return client
}

// vmID is optional, if it is empty, it will return the power status of all VMs, otherwise it will return the power status of the specified VM
// if you know how to make this optional please do
func getPowerStatus(session string, vmID string) []vCenterServers {
	defer timeTrack(time.Now(), "getPowerStatus")
	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")

	// Create a new request
	req, err := http.NewRequest("GET", baseURL+"/api/vcenter/vm/"+vmID, nil)
	if err != nil {
		log.Fatal("Error creating request: ", err)
	}

	req.Header.Add("vmware-api-session-id", session)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error sending request: ", err)
	}
	// defer resp.Body.Close()

	// Read the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response: ", err)
	}

	// Print the response
	return body
}
