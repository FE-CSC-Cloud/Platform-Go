package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
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
func getPowerStatusFromvCenter(session, vmID string) []vCenterServers {
	defer timeTrack(time.Now(), "getPowerStatusFromvCenter")
	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")

	// Create a new request
	req, err := http.NewRequest("GET", baseURL+"/api/vcenter/vm/"+vmID, nil)
	if err != nil {
		log.Println("Error creating request: ", err)
	}

	req.Header.Add("vmware-api-session-id", session)

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request: ", err)
	}

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response: ", err)
	}

	var servers []vCenterServers
	err = json.Unmarshal(body, &servers)
	if err != nil {
		log.Println("Error unmarshalling response: ", err)
	}

	defer resp.Body.Close()
	return servers
}

func createvCenterVM(session, studentID, vmName, templateName string, storage, memory int) (string, error) {
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
			"cluster": getEnvVar("CLUSTER_ID"),
			"folder":  getEnvVar("FOLDER_ID"),
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
					// storage is in GB, so we need to convert it to bytes and add 1 so it is not exactly the same as the template
					"capacity": storage*1073741824 + 1,
				},
			},
			MemoryUpdate: map[string]int{
				"memory": memory * 1024,
			},
		},
	}

	jsonReqBody, err := json.Marshal(reqBody)
	if err != nil {
		log.Println("Error marshalling request body: ", err)
	}

	// Create a new request
	req, err := http.NewRequest("POST", baseURL+"/api/vcenter/vm-template/library-items/"+templateID+"?action=deploy",
		// body
		bytes.NewBuffer(jsonReqBody))
	if err != nil {
		log.Println("Error creating request: ", err)
	}

	req.Header.Add("vmware-api-session-id", session)
	req.Header.Add("Content-Type", "application/json")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request: ", err)
	}

	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if strings.Contains(string(body[1:len(body)-1]), "\"error_type\":\"ALREADY_EXISTS\"") {
		return "", errors.New("VM already exists")
	}

	// remove the " " from the response and convert it to a string of just the VM ID
	return string(body[1 : len(body)-1]), nil
}

func deletevCenterVM(session, vmID string) bool {
	defer timeTrack(time.Now(), "deletevCenterVM")

	success := forcePowerOff(session, vmID)
	if !success {
		return false
	}

	client := createVCenterHTTPClient()
	baseURL := getEnvVar("VCENTER_URL")

	req, err := http.NewRequest("DELETE", baseURL+"/api/vcenter/vm/"+vmID, nil)
	if err != nil {
		log.Println("Error creating request: ", err)
	}

	req.Header.Add("vmware-api-session-id", session)

	// Send the request to delete the VM
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request: ", err)
	}

	if resp.StatusCode != 204 {
		return false
	}

	return true
}

func runStartScript(session string, startScript startScript, firstName, studentId, vCenterId, ip, vmName string) error {
	type Credentials struct {
		InteractiveSession bool   `json:"interactive_session"`
		Type               string `json:"type"`
		UserName           string `json:"user_name"`
		Password           string `json:"password"`
	}

	type Spec struct {
		Arguments string `json:"arguments"`
		Path      string `json:"path"`
	}

	type StartScript struct {
		Credentials Credentials `json:"credentials"`
		Spec        Spec        `json:"spec"`
	}

	client := createVCenterHTTPClient()

	baseURL := getEnvVar("VCENTER_URL")

	reqBodyPre := StartScript{
		Credentials: Credentials{
			InteractiveSession: false,
			Type:               "USERNAME_PASSWORD",
			UserName:           startScript.User,
			Password:           startScript.Password,
		},
		Spec: Spec{
			Arguments: startScript.ScriptLocation + " " + studentId + " " + firstName + " " + ip + " " + firstName + " " + " " + vmName,
			Path:      startScript.ScriptExecutable,
		},
	}

	jsonBody, err := json.Marshal(reqBodyPre)
	if err != nil {
		log.Println("Error marshalling start script: ", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/api/vcenter/vm/"+vCenterId+"/guest/processes?action=create", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Println("Error creating request: ", err)
	}

	req.Header.Add("vmware-api-session-id", session)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request: ", err)
	}

	if resp.StatusCode != 201 {
		// print the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("Error reading response body: ", err)
		}

		log.Println(string(body))

		return errors.New("Error starting script status:" + string(rune(resp.StatusCode)) + " vCenter body: \n" + string(body))
	}

	return nil
}
