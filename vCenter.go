package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

type vCenterTemplates struct {
}

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
func getPowerStatusFromvCenter(session string, vmID string) []vCenterServers {
	defer timeTrack(time.Now(), "getPowerStatusFromvCenter")
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

	// Read the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response: ", err)
	}

	var servers []vCenterServers
	err = json.Unmarshal(body, &servers)
	if err != nil {
		log.Fatal("Error unmarshalling response: ", err)
	}

	defer resp.Body.Close()
	return servers
}

func createvCenterVM(session string, studentID string, vmName string, templateName string) string {
	defer timeTrack(time.Now(), "createvCenterVM")

	type HardwareCustomization struct {
		DisksToUpdate map[string]map[string]int `json:"disks_to_update,omitempty"`
		MemoryUpdate  map[string]int            `json:"memory_update,omitempty"`
	}

	type VMCreateRequest struct {
		Name                  string                `json:"name"`
		Placement             map[string]string     `json:"placement"`
		DiskStorage           map[string]string     `json:"disk_storage"`
		VMHomeStorage         map[string]string     `json:"vm_home_storage"`
		HardwareCustomization HardwareCustomization `json:"hardware_customization,omitempty"`
	}

	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")
	templateID := getFromRedis(templateName)

	datastore := getvCenterDataStoreID(session)

	reqBody := VMCreateRequest{
		Name: "OICT-AUTO-" + studentID + "-" + vmName,
		Placement: map[string]string{
			"cluster": "domain-c8",
			"folder":  "group-v4",
		},
		DiskStorage: map[string]string{
			"datastore": datastore,
		},
		VMHomeStorage: map[string]string{
			"datastore": datastore,
		},
		HardwareCustomization: HardwareCustomization{
			DisksToUpdate: map[string]map[string]int{
				"2000": {
					"capacity": 21474836480,
				},
			},
			MemoryUpdate: map[string]int{
				"memory": 1024,
			},
		},
	}

	jsonReqBody, err := json.Marshal(reqBody)
	if err != nil {
		panic(err)
	}

	// Create a new request
	req, err := http.NewRequest("POST", baseURL+"/api/vcenter/vm-template/library-items/"+templateID+"?action=deploy",
		// body
		bytes.NewBuffer(jsonReqBody))
	if err != nil {
		log.Fatal("Error creating request: ", err)
	}

	req.Header.Add("vmware-api-session-id", session)
	req.Header.Add("Content-Type", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error sending request: ", err)
	}

	// Read the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response: ", err)
	}

	defer resp.Body.Close()

	return string(body)
}

func getvCenterDataStoreID(session string) string {
	if existsInRedis("data_store_id_last_updated") == false {
		return updateDataStoreID(session)
	}

	// check if the data store ID was updated today, otherwise update it
	dataStoreIDLastUpdated := getFromRedis("data_store_id_last_updated")
	if dataStoreIDLastUpdated == "" {
		dataStoreIDLastUpdated = "0"
	}
	if time.Now().Unix()-stringToInt64(dataStoreIDLastUpdated) > 86400 {
		return updateDataStoreID(session)
	}

	return getFromRedis("data_store_id")
}

func updateDataStoreID(session string) string {
	type DataStore struct {
		DataStore string `json:"datastore"`
		Name      string `json:"name"`
		Type      string `json:"type"`
		FreeSpace int64  `json:"free_space"`
		Capacity  int64  `json:"capacity"`
	}

	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")

	// https://172.16.1.80/api/vcenter/datastore?names=datastore1
	req, err := http.NewRequest("GET", baseURL+"/api/vcenter/datastore?names="+getEnvVar("VCENTER_DATASTORE_NAME"), nil)
	if err != nil {
		log.Fatal("Error creating request: ", err)
	}

	// Add the session ID to the request header
	req.Header.Add("vmware-api-session-id", session)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Error sending request: ", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error reading response: ", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatal("Error getting data store ID: ", resp)
	}

	var dataStores []DataStore

	err = json.Unmarshal(body, &dataStores)

	// set the data store ID to redis so we don't have to fetch it every time
	setToRedis("data_store_id", dataStores[0].DataStore)
	// save the date and time the data store ID was last updated
	setToRedis("data_store_id_last_updated", strconv.FormatInt(time.Now().Unix(), 10))

	return dataStores[0].DataStore
}
