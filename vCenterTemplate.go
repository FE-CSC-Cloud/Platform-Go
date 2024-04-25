package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func getTemplatesFromVCenter(session string) []string {
	var templateNames []string
	if existsInRedis("templates_last_updated") == false {
		setToRedis("templates_last_updated", "0")
	}
	templatesLastUpdated := getFromRedis("templates_last_updated")

	templates := fetchTemplateLibraryIdsFromVCenter(session)

	// check if the templates were updated today, otherwise update them
	if time.Now().Unix()-stringToInt64(templatesLastUpdated) > 86400 {
		updateTemplatesFromVCenter(session, templates)
	}

	// get the template names from redis and return them as an array of strings
	for _, template := range templates {
		templateNames = append(templateNames, getFromRedis(template))
	}
	return templateNames
}

func fetchTemplateLibraryIdsFromVCenter(session string) []string {
	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")
	payload := strings.NewReader("{\"type\":\"vm-template\"}")

	req, err := http.NewRequest("POST", baseURL+"/api/content/library/item?action=find", payload)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("vmware-api-session-id", session)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error sending request: ", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response: ", err)
	}

	var templates []string
	err = json.Unmarshal(body, &templates)
	if err != nil {
		log.Fatal("Error unmarshalling response: ", err)
	}

	defer resp.Body.Close()

	return templates
}

func updateTemplatesFromVCenter(session string, templateIDs []string) {
	type vCenterTemplate struct {
		Name string `json:"name"`
	}
	for _, templateID := range templateIDs {
		client := createVCenterHTTPClient()
		baseURL := getEnvVar("VCENTER_URL")

		// Create the request
		req, err := http.NewRequest("GET", baseURL+"/api/content/library/item/"+templateID, nil)
		if err != nil {
			log.Fatal("Error creating request:", err)
			return
		}

		// Add the session ID to the request header
		req.Header.Add("vmware-api-session-id", session)
		req.Header.Add("Content-Type", "application/json")

		// Make the request
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal("Error making request:", err)
			return
		}

		// Read the response body
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Fatal("Error reading response body:", err)
			return
		}

		// Unmarshal the response body
		var template vCenterTemplate
		err = json.Unmarshal(body, &template)

		// Add the template to redis
		setToRedis(templateID, template.Name)
		setToRedis(template.Name, templateID)
		err = resp.Body.Close()
		if err != nil {
			return
		}
	}

	// set the time the templates were last updated as unix int to redis
	setToRedis("templates_last_updated", strconv.FormatInt(time.Now().Unix(), 10))
}
