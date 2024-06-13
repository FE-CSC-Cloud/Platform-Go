package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type IpConfig struct {
	Min              string   `json:"min"`
	Max              string   `json:"max"`
	Excluded         []string `json:"excluded"`
	SourceNetworks   []string `json:"sourceNetworks"`
	Services         []string `json:"services"`
	OutboundServices []string `json:"outboundServices"`
}

var (
	sourceNetworks   string
	inboundServices  string
	outboundServices string
)

func createIPHostInSopohos(ip, studentID, name string) error {
	requestXML := fmt.Sprintf(`
                    <Set operation="add">
                		<IPHost>
                			<Name>OICT-AUTO-HOST-%s-%s</Name>
                			<HostType>IP</HostType>
                			<IPAddress>%s</IPAddress>
                		</IPHost>
                	</Set>`, studentID, name, ip)

	resp := doAuthenticatedSophosRequest(requestXML)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// check if the response is an error
	if !strings.Contains(string(body), `<Status code="200">`) {
		return fmt.Errorf("error creating IP host in Sophos: %s", string(body))
	}

	defer resp.Body.Close()

	return nil
}

func createSophosFirewallRules(studentID, name string) error {
	var wg sync.WaitGroup
	var inboundErr, outboundErr error

	wg.Add(2)

	// create inbound and outbound rules concurrently to save a bit of time
	go func() {
		defer wg.Done()
		inboundErr = createInBoundRuleInSophos(studentID, name)
		if inboundErr != nil {
			log.Println("Error creating inbound rule: ", inboundErr)
		}
	}()

	go func() {
		defer wg.Done()
		outboundErr = createOutBoundRuleInSophos(studentID, name)
		if outboundErr != nil {
			log.Println("Error creating outbound rule: ", outboundErr)
		}
	}()

	// Wait for both goroutines to finish
	wg.Wait()

	// Return the first non-nil error, if any
	if inboundErr != nil {
		return inboundErr
	}
	return outboundErr
}

func createInBoundRuleInSophos(studentId, name string) error {
	xml := fmt.Sprintf(`
                        <Set operation="add">
                            <FirewallRule>
                                <Name>OICT-AUTO-Inbound-%s-%s</Name>
                                <Position>bottom</Position>
                                <PolicyType>Network</PolicyType>
                                <NetworkPolicy>
                                    <Action>Accept</Action>
                                    <SourceZones>
                                        <Zone>LAN</Zone>
                                        <Zone>WAN</Zone>
                                    </SourceZones>
                                    <SourceNetworks>
                                        %s
                                    </SourceNetworks>
                                    <Services>
                                        %s
                                    </Services>
                                    <DestinationZones>
                                        <Zone>DMZ</Zone>
                                    </DestinationZones>
                                    <DestinationNetworks>
                                        <Network>OICT-AUTO-HOST-%s-%s</Network>
                                    </DestinationNetworks>
                                </NetworkPolicy>
                            </FirewallRule>
                        </Set>`, studentId, name, sourceNetworks, inboundServices, studentId, name)

	resp := doAuthenticatedSophosRequest(xml)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// check if the response is an error
	if !strings.Contains(string(body), `<Status code="200">`) {
		return fmt.Errorf(string(body))
	}

	return nil
}

func createOutBoundRuleInSophos(studentId, name string) error {
	xml := fmt.Sprintf(`
                        <Set operation="add">
                            <FirewallRule>
                                <Name>OICT-AUTO-Outbound-%s-%s</Name>
                                <Position>bottom</Position>
                                <PolicyType>Network</PolicyType>
                                <NetworkPolicy>
                                    <Action>Accept</Action>
                                    <SourceZones>
                                        <Zone>DMZ</Zone>
                                        <Zone>LAN</Zone>
                                    </SourceZones>
                                    <SourceNetworks>
                                        %s
                                    </SourceNetworks>
                                    <Services>
                                        %s
                                    </Services>
                                    <DestinationZones>
                                        <Zone>WAN</Zone>
                                    </DestinationZones>
                                    <DestinationNetworks>
                                    </DestinationNetworks>
                                </NetworkPolicy>
                            </FirewallRule>
                        </Set>`, studentId, name, sourceNetworks, outboundServices)

	resp := doAuthenticatedSophosRequest(xml)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// check if the response is an error
	if !strings.Contains(string(body), `<Status code="200">`) {
		return fmt.Errorf(string(body))
	}

	return nil
}

func updateFirewallRuleGroupInSophos(studentId, name string) error {
	xml := fmt.Sprintf(`
                    <Set operation="update">
                        <FirewallRuleGroup>
                            <Name>Autonet</Name>
                            <SecurityPolicyList>
                                <SecurityPolicy>OICT-AUTO-Inbound-%s-%s</SecurityPolicy>
                                <SecurityPolicy>OICT-AUTO-Outbound-%s-%s</SecurityPolicy>
                            </SecurityPolicyList>
                        </FirewallRuleGroup>
                    </Set>`, studentId, name, studentId, name)

	resp := doAuthenticatedSophosRequest(xml)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// check if the response is an error
	if !strings.Contains(string(body), `<Status code="200">`) {
		return fmt.Errorf(string(body))
	}

	defer resp.Body.Close()

	return nil
}

func parseAndSetIpListForSophos() {
	jsonFile, err := os.Open(getEnvVar("IP_LIST"))
	if err != nil {
		log.Fatal("could nog open IP list JSON: ", err)
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	var ipConfig IpConfig
	json.Unmarshal(byteValue, &ipConfig)

	for _, value := range ipConfig.SourceNetworks {
		sourceNetworks += "<Network>" + value + "</Network>"
	}

	for _, value := range ipConfig.Services {
		inboundServices += "<Service>" + value + "</Service>"
	}

	for _, value := range ipConfig.OutboundServices {
		outboundServices += "<Service>" + value + "</Service>"
	}
}

func doAuthenticatedSophosRequest(xml string) *http.Response {
	var requestXML string = fmt.Sprintf(`
                    <Request>
                        <Login>
                            <Username>%s</Username> 
                            <Password>%s</Password>
                        </Login>%s
                    </Request>`,
		getEnvVar("SOPHOS_FIREWALL_USER"), getEnvVar("SOPHOS_FIREWALL_PASS"), xml)

	firewallURL := getEnvVar("SOPHOS_FIREWALL_URL")

	// Create a new HTTP client with disabled SSL verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: !getBoolEnvVar("VERIFY_TLS")},
	}
	client := &http.Client{Transport: tr}

	// Create a new request
	req, err := http.NewRequest("POST", firewallURL, strings.NewReader(url.Values{"reqxml": {requestXML}}.Encode()))
	if err != nil {
		log.Println("Error creating request: ", err)
		return nil
	}

	// Set the content type to application/x-www-form-urlencoded
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error sending request to Sophos: ", err)
		return nil
	}

	return resp
}

func getSophosIpHost() (string, error) {
	requestXML := fmt.Sprintf(`
                    <Get>
                        <IPHost>
                        </IPHost>
                    </Get>`)

	resp := doAuthenticatedSophosRequest(requestXML)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func addIpToSophos(studentID, ip string, count int) error {
	reqXML := fmt.Sprintf(`
                    <Set operation="add">
                        <IPHost>
                            <Name>OICT-AUTO %s Prive %d</Name>
                            <HostType>IP</HostType>
                            <IPAddress>%s</IPAddress>
                            <HostGroupList>
                                <HostGroup>Students Private IP's</HostGroup>
                            </HostGroupList>
                        </IPHost>
                    </Set>`, studentID, count, ip)

	resp := doAuthenticatedSophosRequest(reqXML)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	// check if the response is an error
	// not simply a 200 OK because Sophos returns multiple error codes for each task (adding ip and adding host group)
	if !sophosResponseContainsError(string(body), []int{500, 502, 503, 541}) {
		return fmt.Errorf("error adding home IP in Sophos: %s", string(body))
	}

	return nil
}

func sophosResponseContainsError(response string, errorCodesToLookFor []int) bool {
	for _, code := range errorCodesToLookFor {
		if strings.Contains(response, fmt.Sprintf(`<Status code="%d">`, code)) {
			return true
		}
	}

	return false
}
