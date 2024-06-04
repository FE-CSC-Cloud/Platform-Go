package main

import (
	"log"
	"net/url"
	"strings"
	"time"
)

// call function with defer to track time
func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

func cleanURL(input string) string {
	defer timeTrack(time.Now(), "cleanURL")
	// Remove newline characters
	clean := strings.ReplaceAll(input, "\n", "")

	// Remove tab characters
	clean = strings.ReplaceAll(clean, "\t", "")

	// Remove return characters
	clean = strings.ReplaceAll(clean, "\r", "")

	// Replace spaces with hyphens
	clean = strings.ReplaceAll(clean, " ", "-")

	// Remove any other control characters
	clean = url.PathEscape(clean)

	return clean
}
