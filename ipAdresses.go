package main

import "log"

func findEmptyIp() string {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return ""
	}

	var ip string
	err = db.QueryRow("SELECT ip FROM ip_adresses WHERE virtual_machine_id IS NULL LIMIT 1").Scan(&ip)
	if err != nil {
		log.Println("Error executing query: ", err)
		return ""
	}

	return ip
}

func assignIPToVM(ip string, vmID string) bool {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return false
	}

	_, err = db.Exec("UPDATE ip_adresses SET virtual_machine_id = ? WHERE ip = ?", vmID, ip)
	if err != nil {
		log.Println("Error executing query: ", err)
		return false
	}

	return true
}
