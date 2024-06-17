package main

import (
	"fmt"
	"github.com/go-ldap/ldap/v3"
	"log"
	"strings"
)

func connectAndBind(username string, password string) (*ldap.Conn, error) {
	ldapURL := "ldap://" + getEnvVar("LDAP_HOST") + ":389"
	ldapConn, err := ldap.DialURL(ldapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP server")
	}

	// Bind with provided username and password to validate the user
	err = ldapConn.Bind(username+"@"+getEnvVar("LDAP_READ_DOMAIN"), password)
	if err != nil {
		return nil, fmt.Errorf("email or password is incorrect")
	}

	return ldapConn, nil
}

func fetchUserInfoWithSID(sid string) (string, string, string, string, error) {
	// Connect to LDAP
	ldapConn, err := connectAndBind(getEnvVar("LDAP_READ_USER"), getEnvVar("LDAP_READ_PASS"))

	// Search for the user with the given SID
	searchRequest := ldap.NewSearchRequest(
		getEnvVar("LDAP_BASE_DN"),
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(objectSid=%s))", sid),
		[]string{"memberOf", "givenName", "description", "mail", "sn"},
		nil,
	)

	sr, err := ldapConn.Search(searchRequest)
	if err != nil {
		return "", "", "", "", fmt.Errorf("failed to search LDAP server: %v", err)
	}

	if len(sr.Entries) == 0 {
		return "", "", "", "", fmt.Errorf("no entries found for user with SID %s", sid)
	}

	entry := sr.Entries[0]

	memberOf := entry.GetAttributeValues("memberOf")

	firstName := entry.GetAttributeValue("givenName")

	description := entry.GetAttributeValue("description")

	email := entry.GetAttributeValue("mail")
	log.Println("Email: ", email)
	// last name is an array for some reason so we have to check if it exists
	var lastName string
	if len(entry.GetAttributeValues("sn")) >= 1 {
		lastName = entry.GetAttributeValues("sn")[0]
	}

	fullName := firstName + " " + lastName

	var groups []string

	// Get only the CN= and not the OU=
	for _, dn := range memberOf {
		// Extract the part of the DN starting with "CN=" and ending before the next comma
		start := strings.Index(dn, "CN=")
		if start != -1 {
			start += 3 // Skip past "CN="
			end := strings.Index(dn[start:], ",")
			if end != -1 {
				groups = append(groups, dn[start:start+end])
			} else {
				groups = append(groups, dn[start:])
			}
		}
	}

	return fullName, description, sid, email, nil
}

func fetchUserInfo(ldapConn *ldap.Conn, username string) ([]string, string, string, string, error) {
	searchRequest := ldap.NewSearchRequest(
		getEnvVar("LDAP_BASE_DN"),
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", username),
		[]string{"memberOf", "givenName", "description", "sn", "objectSid"},
		nil,
	)

	sr, err := ldapConn.Search(searchRequest)
	if err != nil {
		return nil, "", "", "", fmt.Errorf("failed to search LDAP server: %v", err)
	}

	if len(sr.Entries) == 0 {
		return nil, "", "", "", fmt.Errorf("no entries found for user %s", username)
	}

	entry := sr.Entries[0]

	memberOf := entry.GetAttributeValues("memberOf")

	firstName := entry.GetAttributeValue("givenName")

	description := entry.GetAttributeValue("description")
	// last name is an array for some reason so we have to check if it exists
	var lastName string
	if len(entry.GetAttributeValues("sn")) >= 1 {
		lastName = entry.GetAttributeValues("sn")[0]
	}

	fullName := firstName + " " + lastName

	var groups []string

	// Get only the CN= and not the OU=
	for _, dn := range memberOf {
		// Extract the part of the DN starting with "CN=" and ending before the next comma
		start := strings.Index(dn, "CN=")
		if start != -1 {
			start += 3 // Skip past "CN="
			end := strings.Index(dn[start:], ",")
			if end != -1 {
				groups = append(groups, dn[start:start+end])
			} else {
				groups = append(groups, dn[start:])
			}
		}
	}

	// Extract the SID
	objectSid := entry.GetRawAttributeValue("objectSid")
	sidString := sidToString(objectSid)

	return groups, fullName, description, sidString, err
}
