package main

import (
	"github.com/LordMartron94/Last.fm-Export/backend"
	"github.com/joho/godotenv"
	"log"
	"os"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Retrieve environment variables
	key := os.Getenv("KEY")
	user := os.Getenv("USER")

	GetSrobbles(user, key)
}

// GetSrobbles gets scrobbles
func GetSrobbles(user string, apiKey string) {
	log.Printf("Fetching scrobbles for user %s with api key %s...\n", user, apiKey)
	err := backend.GetScrobblesCSV(user, apiKey, "scrobbles.csv")

	if err != nil {
		log.Println("Error fetching scrobbles.")
		panic(err)
	}
}
