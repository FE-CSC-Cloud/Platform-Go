package main

import (
	"encoding/json"
	"fmt"
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

	req, err := http.NewRequest("GET", baseURL+"/api/vcenter/datastore?names="+getEnvVar("VCENTER_DATASTORE_NAME"), nil)
	if err != nil {
		log.Println("Error creating request: ", err)
		return ""
	}

	req.Header.Add("vmware-api-session-id", session)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request: ", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response: ", err)
		return ""
	}

	fmt.Println("Response: ", string(body))

	if resp.StatusCode != 200 {
		log.Println("Error getting data store ID: ", resp)
		return ""
	}

	var dataStores []DataStore
	err = json.Unmarshal(body, &dataStores)
	if err != nil {
		log.Println("Error unmarshalling response: ", err)
		return ""
	}

	if len(dataStores) == 0 {
		log.Println("No data stores found in response: ", string(body))
		return ""
	}

	setToRedis("data_store_id", dataStores[0].DataStore, 0)
	setToRedis("data_store_id_last_updated", strconv.FormatInt(time.Now().Unix(), 10), 0)

	return dataStores[0].DataStore
}

func RefreshDataStores(c echo.Context) error {
	session := getVCenterSession()
	dataStores := updateDataStoreID(session)
	return c.JSON(http.StatusOK, dataStores)
}
