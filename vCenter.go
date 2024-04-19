package main

import (
	"crypto/tls"
	"net/http"
)

func createVCenterHTTPClient() *http.Client {
	// Create a Transport for our client so we can skip SSL verification because the vCenter certificate is self-signed
	tlsConfig := &tls.Config{InsecureSkipVerify: !getBoolEnvVar("VERIFY_TLS")}
	transport := &http.Transport{TLSClientConfig: tlsConfig}

	// Create client with the Transport that can skip SSL verification if needed
	client := &http.Client{Transport: transport}

	return client
}

func getPower
