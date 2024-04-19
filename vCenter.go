package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func refreshSessionID() string {
	db := connectToKeyDB()
	var sessionID string

	sessionID, err := db.Get(context.Background(), "session").Result()
	log.Println("Session ID: ", sessionID)
	if sessionID != "" {
		return sessionID
	}

	log.Println("Session ID not found in KeyDB, refreshing session ID")

	user := getEnvVar("VCENTER_USER")
	pass := getEnvVar("VCENTER_PASS")
	baseURL := getEnvVar("VCENTER_URL")

	// Create a Transport for our client so we can skip SSL verification because the vCenter certificate is self-signed
	tlsConfig := &tls.Config{InsecureSkipVerify: !getBoolEnvVar("VERIFY_TLS")}
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	// Create client with the Transport that skips SSL verification
	client := &http.Client{Transport: transport}

	req, err := http.NewRequest("POST", baseURL+"/api/session", nil)
	if err != nil {
		log.Fatal(err)
	}

	req.SetBasicAuth(user, pass)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(string(body))

	err = json.Unmarshal(body, &sessionID)
	if err != nil {
		log.Fatal(err)
	}

	// Here you can use the session value
	err = db.Set(context.Background(), "session", sessionID, 0).Err()
	if err != nil {
		panic(err)
	}

	return sessionID
}
