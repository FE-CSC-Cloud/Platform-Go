package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func authenticatedDNSRequest(path string, queries [][]string) ([]byte, error) {
	queryString := "&"
	for _, query := range queries {
		queryString += query[0] + "=" + query[1] + "&"
	}
	req, _ := http.NewRequest("GET", getEnvVar("TEHCNITIUM_HOST")+"/api/"+path+"?token="+getEnvVar("TECHNITIUM_API_TOKEN")+queryString, nil)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, err
	}

	// parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if strings.Contains(string(body), "error") {
		return nil, fmt.Errorf(string(body))
	}

	return body, nil
}
