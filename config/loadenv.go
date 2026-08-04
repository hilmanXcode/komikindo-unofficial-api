package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	DSN     string
	API_KEY string
)

func LoadEnvVariables() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	DSN = os.Getenv("DATABASE_DSN")
	API_KEY = os.Getenv("SECRET_SELF_API_KEY")
}
