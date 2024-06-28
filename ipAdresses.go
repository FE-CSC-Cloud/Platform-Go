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

func assignIPToVM(ip string, vmID string) error {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return err
	}

	_, err = db.Exec("UPDATE ip_adresses SET virtual_machine_id = ? WHERE ip = ?", vmID, ip)
	if err != nil {
		log.Println("Error executing query: ", err)
		return err
	}

	return nil
}

func claimIp(ip string) error {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return err
	}

	_, err = db.Exec("UPDATE ip_adresses SET virtual_machine_id = 'claimed' WHERE ip = ?", ip)
	if err != nil {
		log.Println("Error executing query: ", err)
		return err
	}

	return nil
}

func getIpFromVM(vmID string) string {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return ""
	}

	var ip string
	err = db.QueryRow("SELECT ip FROM ip_adresses WHERE virtual_machine_id = ?", vmID).Scan(&ip)
	if err != nil {
		log.Println("Error executing query: ", err)
		return ""
	}

	return ip
}

func unassignIPfromVM(vmID string) error {
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database", err)
	}

	_, err = db.Exec("UPDATE ip_adresses SET virtual_machine_id = NULL WHERE virtual_machine_id = ?", vmID)
	if err != nil {
		log.Println("Error executing query: ", err)
		return err
	}

	return nil
}
