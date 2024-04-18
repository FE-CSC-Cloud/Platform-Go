package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type Session struct {
	Value string `json:"value"`
}

func refreshSessionID() {
	log.Println("refresh?")

	user := os.Getenv("VCenterUser")
	pass := os.Getenv("VCenterPass")
	baseURL := os.Getenv("VCENTER_URL")

	client := &http.Client{}
	req, err := http.NewRequest("POST", baseURL+"/rest/com/vmware/cis/session", nil)
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

	var session Session
	err = json.Unmarshal(body, &session)
	if err != nil {
		log.Fatal(err)
	}

	// Here you can use the session value
	log.Println(session.Value)

	db := connectToKeyDB()

	err = db.Put([]byte("session"), []byte(session.Value))

	if err != nil {
		log.Fatal(err)
	}

	log.Println("Session ID refreshed, session ID: ", session.Value)
}
