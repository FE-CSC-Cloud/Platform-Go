package main

import (
    "crypto/tls"
    "encoding/json"
    "io/ioutil"
    "log"
    "net/http"
    "strings"
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

type vCenterTemplates struct {
    Library_item_id string `json:"library_item_id"`
}

func fetchTemplatesFromVCenter(session string) {
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

    log.Println(string(body))

    var templates []vCenterTemplates
    err = json.Unmarshal(body, &templates)
    if err != nil {
        log.Fatal("Error unmarshalling response: ", err)
    }

    defer resp.Body.Close()
}
