package main

import (
	"log"
	"time"
)

// call function with defer to track time
func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}
