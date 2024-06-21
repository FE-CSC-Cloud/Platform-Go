package main

import (
	"log"
	"net/http"
	"time"
)

func powerOn(session, id string) bool {
	defer timeTrack(time.Now(), "powerOn")
	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")

	req, err := http.NewRequest("POST", baseURL+"/api/vcenter/vm/"+id+"/power?action=start", nil)
	if err != nil {
		log.Println("Error creating request: ", err)
		return false
	}

	req.Header.Add("vmware-api-session-id", session)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request: ", err)
	}

	if resp.StatusCode != 204 && resp.StatusCode != 400 {
		return false
	}

	defer resp.Body.Close()

	return true
}

func powerOff(session, id string) bool {
	defer timeTrack(time.Now(), "powerOff")
	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")

	req, err := http.NewRequest("POST", baseURL+"/api/vcenter/vm/"+id+"/power?action=shutdown", nil)
	if err != nil {
		log.Println("Error creating request: ", err)
		return false
	}

	req.Header.Add("vmware-api-session-id", session)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request: ", err)
		return false
	}

	log.Println(resp.StatusCode)
	if resp.StatusCode != 204 && resp.StatusCode != 400 {
		return false
	}

	defer resp.Body.Close()

	return true
}

func forcePowerOff(session, id string) bool {
	defer timeTrack(time.Now(), "forcePowerOff")
	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")

	req, err := http.NewRequest("POST", baseURL+"/api/vcenter/vm/"+id+"/power?action=stop", nil)
	if err != nil {
		log.Println("Error creating request: ", err)
	}

	req.Header.Add("vmware-api-session-id", session)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request: ", err)
	}

	if resp.StatusCode != 204 && resp.StatusCode != 400 {
		return false
	}

	defer resp.Body.Close()

	return true
}
