package main

import (
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
	"strconv"
)

func GetTickets(c echo.Context) error {
	id := c.Param("id")
	UserId, isAdmin, _, _ := getUserAssociatedWithJWT(c)

	db, err := connectToDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	baseQuery := "SELECT tickets.id, title, message, creator_name, status, tickets.created_at, vm.vcenter_id, vm.name, vm.operating_system, vm.end_date, vm.storage, vm.memory, vm.ip FROM tickets JOIN virtual_machines vm on tickets.server_id = vm.id"

	var query string
	var args []interface{}
	if id != "" {
		if isAdmin {
			query = baseQuery + " WHERE tickets.id = ?"
			args = append(args, id)
		} else {
			query = baseQuery + " WHERE user_id = ? AND tickets.id = ?"
			args = append(args, UserId, id)
		}
	} else {
		if isAdmin {
			query = baseQuery + " ORDER BY FIELD(status, 'Pending', 'Accepted', 'Rejected'), created_at DESC"
		} else {
			query = baseQuery + " WHERE user_id = ? ORDER BY FIELD(status, 'Pending', 'Accepted', 'Rejected'), created_at DESC"
			args = append(args, UserId)
		}
	}

	rows, err := db.Query(query, args...)

	if err != nil {
		log.Println(err)
		return c.JSON(http.StatusNotFound, "Failed to fetch tickets")
	}

	var tickets []map[string]interface{}
	for rows.Next() {
		var (
			id                                  int
			title, message, creatorName, status string
			createdAt                           []byte

			vcenterId, name, operatingSystem, endDate, ip string
			storage, memory                               int
		)

		err := rows.Scan(&id, &title, &message, &creatorName, &status, &createdAt, &vcenterId, &name, &operatingSystem, &endDate, &storage, &memory, &ip)
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

			// This is a nested object
			"vm": map[string]interface{}{
				"vcenterId":       vcenterId,
				"name":            name,
				"operatingSystem": operatingSystem,
				"endDate":         endDate,
				"storage":         storage,
				"memory":          memory,
				"ip":              ip,
			},
		})
	}

	return c.JSON(http.StatusOK, tickets)
}

func CreateTicket(c echo.Context) error {
	type createTicketBody struct {
		Title    string `json:"title"`
		Message  string `json:"message"`
		ServerId *int   `json:"server_id"`
	}

	var body createTicketBody
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, "Invalid request")
	}

	UserId, isAdmin, fullName, _ := getUserAssociatedWithJWT(c)

	// Create ticket
	db, err := connectToDB()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, "Failed to connect to database")
	}

	if body.ServerId == nil {
		_, err = db.Exec("INSERT INTO tickets (title, message, user_id, creator_name, status) VALUES (?, ?, ?, ?, 'Pending')", body.Title, body.Message, UserId, fullName)
	} else {
		// parse the server id to string
		stringServerId := strconv.Itoa(*body.ServerId)

		if !checkIfServerBelongsToUser(UserId, stringServerId, db) && !isAdmin {
			return c.JSON(http.StatusUnauthorized, "You are not allowed to access this server")
		}

		_, err = db.Exec("INSERT INTO tickets (title, message, user_id, creator_name, status, server_id) VALUES (?, ?, ?, ?, 'Pending', ?)", body.Title, body.Message, UserId, fullName, *body.ServerId)
	}
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

	_, err = db.Exec("UPDATE tickets SET status = ? WHERE id = ?", body.Status, id)
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
