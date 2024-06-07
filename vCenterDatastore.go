package main

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

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

	body, err := io.ReadAll(resp.Body)
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
	setToRedis("data_store_id", dataStores[0].DataStore, 0)
	// save the date and time the data store ID was last updated
	setToRedis("data_store_id_last_updated", strconv.FormatInt(time.Now().Unix(), 10), 0)

	return dataStores[0].DataStore
}

func refreshDataStores(c echo.Context) error {
	session := getVCenterSession()
	dataStores := updateDataStoreID(session)
	return c.JSON(http.StatusOK, dataStores)
}
