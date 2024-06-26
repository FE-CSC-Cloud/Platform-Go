package main

import (
	"database/sql"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

func GetTickets(c echo.Context) error {
	id := c.Param("id")
	UserId, isAdmin, _, _ := getUserAssociatedWithJWT(c)

	db, err := connectToDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	var rows *sql.Rows

	if id != "" {
		rows, err = db.Query("SELECT id, title, message, creator_name, status, created_at FROM tickets WHERE user_id = ? AND id = ?", UserId, id)
	} else if isAdmin && id != "" {
		rows, err = db.Query("SELECT id, title, message, creator_name, status, created_at FROM tickets WHERE id = ?", id)
	} else if isAdmin {
		rows, err = db.Query("SELECT id, title, message, creator_name, status, created_at FROM tickets")
	} else {
		rows, err = db.Query("SELECT id, title, message, creator_name, status, created_at FROM tickets WHERE user_id = ?", UserId)
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to fetch tickets")
	}

	var tickets []map[string]interface{}
	for rows.Next() {
		var id int
		var title, message, creatorName, status string
		var createdAt []byte
		err := rows.Scan(&id, &title, &message, &creatorName, &status, &createdAt)
		if err != nil {
			log.Println(err)
			continue
		}

		createdAtStr := string(createdAt)
		createdAt = []byte(createdAtStr[8:10] + "-" + createdAtStr[5:7] + "-" + createdAtStr[0:4])

		tickets = append(tickets, map[string]interface{}{
			"id":          id,
			"title":       title,
			"message":     message,
			"creatorName": creatorName,
			"status":      status,
			"createdAt":   createdAtStr,
		})
	}

	return c.JSON(http.StatusOK, tickets)
}

func CreateTicket(c echo.Context) error {
	type createTicketBody struct {
		Title   string `json:"title"`
		Message string `json:"message"`
	}

	var body createTicketBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid request")
	}

	UserId, _, fullName, _ := getUserAssociatedWithJWT(c)

	// Create ticket
	db, err := connectToDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	_, err = db.Exec("INSERT INTO tickets (title, message, user_id, creator_name, status) VALUES (?, ?, ?, ?, 'Pending')", body.Title, body.Message, UserId, fullName)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, "Failed to create ticket")
	}

	return c.JSON(http.StatusOK, "Ticket created")
}

func UpdateTicket(c echo.Context) error {
	// I made this a struct so I can easily add more fields later.
	// I didn't do it yet because I didn't feel like dealing with the permissions and query to do so cleanly
	type updateTicketBody struct {
		Status string `json:"status"`
	}

	var body updateTicketBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid request")
	}

	id := c.Param("id")
	UserId, isAdmin, _, _ := getUserAssociatedWithJWT(c)

	if !isAdmin {
		return c.JSON(http.StatusUnauthorized, "You are not allowed to make this change")
	}

	if !isAllowedAccessToTicket(isAdmin, UserId, id) {
		return c.JSON(http.StatusUnauthorized, "You are not allowed to access this ticket")
	}

	// Update ticket
	db, err := connectToDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	_, err = db.Exec("UPDATE tickets SET status = ? WHERE user_id = ? AND id = ?", body.Status, UserId, id)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, "Failed to update ticket")
	}

	return c.JSON(http.StatusOK, "Ticket updated")
}

func DeleteTicket(c echo.Context) error {
	id := c.Param("id")
	UserId, isAdmin, _, _ := getUserAssociatedWithJWT(c)

	if !isAllowedAccessToTicket(isAdmin, UserId, id) {
		return c.JSON(http.StatusNotFound, "ONVOLDOENDE")
	}

	db, err := connectToDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	_, err = db.Exec("DELETE FROM tickets WHERE id = ?", id)
	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusInternalServerError, "Failed to delete ticket")
	}

	return c.JSON(http.StatusOK, "Ticket deleted")
}

func isAllowedAccessToTicket(isAdmin bool, UserId, id string) bool {
	db, err := connectToDB()
	if err != nil {
		return false
	}

	var userId string
	err = db.QueryRow("SELECT user_id FROM tickets WHERE id = ?", id).Scan(&userId)
	if err != nil {
		return false
	}

	if isAdmin || UserId == userId {
		return true
	}

	return false
}