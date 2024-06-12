package main

func logErrorInDB(err error) {
	db, _ := connectToDB()

	// insert the error in the database
	_, err = db.Exec("INSERT INTO errors (message) VALUES (?)", err.Error())

	defer db.Close()
}
