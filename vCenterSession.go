package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// getVCenterSession gets the session ID from vCenter
func getVCenterSession() string {
	defer timeTrack(time.Now(), "getVCenterSession")
	var sessionID string

	// Check if the session ID is already stored in Redis so we don't have to get a new one every time
	sessionID = getFromRedis("session")
	if sessionID != "" && sessionID != "Unauthorized" {
		log.Println("Session ID in Cache: ", sessionID)
		expired := checkIfvCenterSessionIsExpired(sessionID)

		if !expired {
			return sessionID
		}

		log.Println("Session ID in Cache is expired, refreshing session ID")
	} else {
		log.Println("Session ID not found in Cache, refreshing session ID")
	}

	sessionID = refreshVCenterSession()

	log.Println("Session ID from vCenter: ", sessionID)

	return sessionID
}

func vCenterFetchSession() string {
	var sessionID string
	user := getEnvVar("VCENTER_USER")
	pass := getEnvVar("VCENTER_PASS")
	baseURL := getEnvVar("VCENTER_URL")

	client := createVCenterHTTPClient()

	// Create a new request to get a new session ID
	req, err := http.NewRequest("POST", baseURL+"/api/session", nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(user, pass)
	req.Header.Add("Content-Type", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// handle error_type UNAUTHENTICATED from vCenter
	if resp.StatusCode == 401 {
		return "Unauthorized"
	}

	err = json.Unmarshal(body, &sessionID)
	if err != nil {
		log.Fatal(err)
	}

	return sessionID
}

func checkIfvCenterSessionIsExpired(sessionID string) bool {
	client := createVCenterHTTPClient()

	// Create a new request to check if the session ID is still valid
	req, err := http.NewRequest("GET", getEnvVar("VCENTER_URL")+"/api/session", nil)
	if err != nil {
		log.Fatal(err)
	}

	user := getEnvVar("VCENTER_USER")
	pass := getEnvVar("VCENTER_PASS")

	req.SetBasicAuth(user, pass)
	req.Header.Add("vmware-api-session-id", sessionID)
	req.Header.Add("Content-Type", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return false
	}

	return true
}

func refreshVCenterSession() string {
	sessionID := vCenterFetchSession()
	if sessionID != "Unauthorized" {
		setToRedis("session", sessionID)
	} else {
		log.Println("Unauthorized, clearing session from cache")
	}

	log.Println("Refreshing session, id: ", sessionID)

	return sessionID
}
