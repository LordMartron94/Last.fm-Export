package main

import (
	"fmt"
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
	data, err := backend.GetScrobbles(user, apiKey, true)

	if err != nil {
		log.Println("Error fetching scrobbles.")
		panic(err)
	}

	log.Println("Fetched scrobbles.")
	var x backend.ScrobbleArray
	x = data
	csv := x.ToCsv("\t")
	file := "scrobbles.csv"

	err = SaveCsvFile(csv, file)
	if err != nil {
		log.Panicf("Error saving to %s.\n", file)
		panic(err)
	}
	log.Printf("Saved %s.\n", file)
}

// SaveCsvFile saves csv to file
func SaveCsvFile(csv []string, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, line := range csv {
		fmt.Fprintln(f, line)
	}
	defer f.Close()

	return nil
}
