// Package env is deprecated: Bard-era client, removed before v0.2.0. Use gogemini.New and Client.GenerateContent.
package env

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Deprecated: Bard-era client, removed before v0.2.0. Use gogemini.New and Client.GenerateContent.
func Load() {
	env := os.Getenv("APP_ENV")
	if env == "local" {
		err := godotenv.Load("./env/.env.local")
		if err != nil {
			log.Fatal("Error loading .env file")
		}
		return
	}

	if env == "runner" {
		err := godotenv.Load("../env/.env.runner")
		if err != nil {
			log.Fatal("Error loading .env file")
		}

		return
	}

	if env == "test" {
		err := godotenv.Load("../env/.env.test")
		if err != nil {
			log.Fatal("Error loading .env file")
		}

		return
	}
}
