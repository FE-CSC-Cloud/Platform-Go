package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
)

func removeFirewallFromServerInSophos(studentID, name string) error {
	var wg sync.WaitGroup
	var errInbound, errOutbound error

	wg.Add(2)

	go func() {
		defer wg.Done()
		errInbound = removeInBoundRuleInSophos(studentID, name)
	}()

	go func() {
		defer wg.Done()
		errOutbound = removeOutBoundRuleInSophos(studentID, name)
	}()

	wg.Wait()

	if errInbound != nil {
		return errInbound
	}

	if errOutbound != nil {
		return errOutbound
	}

	err := removeIPHostInSophos(studentID, name)
	if err != nil {
		return err
	}

	return nil
}

func removeIPHostInSophos(studentID, name string) error {
	xml := fmt.Sprintf(`
                    <Remove>
                        <IPHost>
                            <Name>OICT-AUTO-HOST-%s-%s</Name>
                        </IPHost>
                    </Remove>`, studentID, name)

	resp := doAuthenticatedSophosRequest(xml)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	// check if the response is an error
	if !strings.Contains(string(body), `<Status code="200">`) {
		return fmt.Errorf("error removing IP host in Sophos: %s", string(body))
	}

	return nil
}
func removeInBoundRuleInSophos(studentId, name string) error {
	xml := fmt.Sprintf(`
                        <Remove>
                            <FirewallRule>
                                <Name>OICT-AUTO-Inbound-%s-%s</Name>
                            </FirewallRule>
                        </Remove>`, studentId, name)

	resp := doAuthenticatedSophosRequest(xml)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	// check if the response is an error
	if !strings.Contains(string(body), `<Status code="200">`) {
		return fmt.Errorf("error removing inbound rule in Sophos: %s", string(body))
	}

	return nil

}

func removeOutBoundRuleInSophos(studentId, name string) error {
	xml := fmt.Sprintf(`
                        <Remove>
                            <FirewallRule>
                                <Name>OICT-AUTO-Outbound-%s-%s</Name>
                            </FirewallRule>
                        </Remove>`, studentId, name)

	resp := doAuthenticatedSophosRequest(xml)

	// parse response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	// check if the response is an error
	if !strings.Contains(string(body), `<Status code="200">`) {
		return fmt.Errorf("error removing outbound rule in Sophos: %s", string(body))
	}

	return nil
}
