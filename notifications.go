package main

import (
	"crypto/tls"
	"database/sql"
	"log"
	"net"
	"net/smtp"
)

type Notification struct {
	ID      int    `json:"id"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Read    bool   `json:"read"`
}

func createNotificationForUser(db *sql.DB, userID, title, message string) {
	_, err := db.Exec("INSERT INTO notifications (title, message, user_id) VALUES (?, ?, ?)", title, message, userID)
	if err != nil {
		panic(err)
	}
}

func getNotificationsAssociatedWithUser(db *sql.DB, userID string) []Notification {
	rows, err := db.Query("SELECT id, title, message, read_notif FROM notifications WHERE user_id = ?", userID)
	if err != nil {
		panic(err)
	}

	var notifications []Notification
	for rows.Next() {
		var notification Notification
		err = rows.Scan(&notification.ID, &notification.Title, &notification.Message, &notification.Read)
		if err != nil {
			panic(err)
		}

		notifications = append(notifications, notification)
	}

	return notifications
}

func sendEmailNotification(to, title, message string) {
	// Send email
	from := getEnvVar("EMAIL_FROM")
	username := getEnvVar("EMAIL_USER")
	password := getEnvVar("EMAIL_PASS")
	host := getEnvVar("EMAIL_HOST")
	port := getEnvVar("EMAIL_PORT")
	security := getEnvVar("SECURITY_EMAIL")

	// Authenticate to the SMTP server
	auth := smtp.PlainAuth("", username, password, host)

	// Create message
	m := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: " + title + "\n\n" +
		message

	switch security {
	case "SSL":
		// Connect to the SMTP Server
		c, err := smtp.Dial(host + ":" + port)
		if err != nil {
			log.Fatal(err)
		}

		// Upgrade to SSL
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}
		if err := c.StartTLS(tlsconfig); err != nil {
			log.Fatal(err)
		}

		sendMail(c, from, to, m, auth)
	case "STARTTLS":
		// Connect to the SMTP Server
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}

		conn, err := tls.Dial("tcp", host+":"+port, tlsconfig)
		if err != nil {
			log.Fatal(err)
		}

		c, err := smtp.NewClient(conn, host)
		if err != nil {
			log.Fatal(err)
		}

		sendMail(c, from, to, m, auth)
	case "TLS":
		// Connect to the SMTP Server
		conn, err := net.Dial("tcp", host+":"+port)
		if err != nil {
			log.Fatal(err)
		}

		c, err := smtp.NewClient(conn, host)
		if err != nil {
			log.Fatal(err)
		}

		// Upgrade to TLS
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}
		if err := c.StartTLS(tlsconfig); err != nil {
			log.Fatal(err)
		}

		sendMail(c, from, to, m, auth)
	default:
		// Connect to the SMTP Server
		c, err := smtp.Dial(host + ":" + port)
		if err != nil {
			log.Fatal(err)
		}

		sendMail(c, from, to, m, auth)
	}
}

func sendMail(c *smtp.Client, from string, to string, m string, auth smtp.Auth) {
	if err := c.Auth(auth); err != nil {
		log.Fatal(err)
	}

	if err := c.Mail(from); err != nil {
		log.Fatal(err)
	}

	if err := c.Rcpt(to); err != nil {
		log.Fatal(err)
	}

	w, err := c.Data()
	if err != nil {
		log.Fatal(err)
	}

	_, err = w.Write([]byte(m))
	if err != nil {
		log.Fatal(err)
	}

	err = w.Close()
	if err != nil {
		log.Fatal(err)
	}

	c.Quit()
}
