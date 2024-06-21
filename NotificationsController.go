package main

import (
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

func GetNotifications(c echo.Context) error {
	studentID, _, _, _ := getUserAssociatedWithJWT(c)
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return c.String(http.StatusInternalServerError, "Internal server error")
	}

	notifications := getNotificationsAssociatedWithUser(db, studentID)

	return c.JSON(http.StatusOK, notifications)
}

func ChangeReadStatusOfNotification(c echo.Context) error {
	studentID, _, _, _ := getUserAssociatedWithJWT(c)
	db, err := connectToDB()
	if err != nil {
		log.Println("Error connecting to database: ", err)
		return c.String(http.StatusInternalServerError, "Internal server error")
	}

	notificationID := c.Param("id")

	_, err = db.Exec("UPDATE notifications SET read_notif = IF(read_notif = 1, 0, 1) WHERE user_id = ? AND id = ?", studentID, notificationID)
	if err != nil {
		log.Println("Error executing query: ", err)
		return c.String(http.StatusInternalServerError, "Internal server error")
	}

	return c.String(http.StatusOK, "Notification status updated")
}
