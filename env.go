package main

import (
	"github.com/joho/godotenv"
	"log"
	"os"
)

func getEnvVar(varName string) string {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	return os.Getenv(varName)
}
