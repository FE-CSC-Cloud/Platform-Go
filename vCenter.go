package main

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

func refreshSessionID() string {
	defer timeTrack(time.Now(), "refreshSessionID")
	var sessionID string

	// Check if the session ID is already stored in Redis so we don't have to get a new one every time
	sessionID = getFromRedis("session")
	if sessionID != "" {
		log.Println("Session ID in Cache: ", sessionID)
		return sessionID
	}

	log.Println("Session ID not found in Cache, refreshing session ID")

	user := getEnvVar("VCENTER_USER")
	pass := getEnvVar("VCENTER_PASS")
	baseURL := getEnvVar("VCENTER_URL")

	// Create a Transport for our client so we can skip SSL verification because the vCenter certificate is self-signed
	tlsConfig := &tls.Config{InsecureSkipVerify: !getBoolEnvVar("VERIFY_TLS")}
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	// Create client with the Transport that can skip SSL verification if needed
	client := &http.Client{Transport: transport}

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

	err = json.Unmarshal(body, &sessionID)
	if err != nil {
		log.Fatal(err)
	}

	setToRedis("session", sessionID)

	log.Println("Session ID from vCenter: ", sessionID)

	return sessionID
}
