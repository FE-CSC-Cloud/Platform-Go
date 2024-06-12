package main

import "database/sql"

func createNotificationForUser(db *sql.DB, userID, title, message string) {
	_, err := db.Exec("INSERT INTO notifications (title, message, user_id) VALUES (?, ?, ?)", title, message, userID)
	if err != nil {
		panic(err)
	}
}
