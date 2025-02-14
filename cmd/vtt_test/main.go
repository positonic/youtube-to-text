package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"jamesfarrell.me/youtube-to-text/internal/storage/db"
	"jamesfarrell.me/youtube-to-text/internal/transcription"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env file: %v\n", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable must be set")
	}
	// Initialize database connection
	db, err := db.NewConnection(db.Config{URL: dbURL})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Query for videos that need processing
	rows, err := db.Query(`
		SELECT id, transcription 
		FROM "Video" 
		WHERE status = 'transcribed' 
		AND "isSearchable" = true
	`)
	if err != nil {
		log.Fatalf("Failed to query videos: %v", err)
	}
	defer rows.Close()

	// Process each video
	for rows.Next() {
		var (
			videoID      string
			vttContent   string
		)
		if err := rows.Scan(&videoID, &vttContent); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		fmt.Printf("\nProcessing video ID: %s\n", videoID)
		fmt.Println("Parsing VTT content...")

		// Parse the VTT content
		entries, err := transcription.ParseVTT(vttContent)
		if err != nil {
			fmt.Printf("Error parsing VTT for video %s: %v\n", videoID, err)
			continue
		}
		fmt.Println("VTT entries:", entries)
		// Print the parsed entries
		for i, entry := range entries {
			fmt.Printf("\nEntry %d:\n", i+1)
			fmt.Printf("Number: %d\n", entry.Number)
			fmt.Printf("Start: %v\n", entry.Start)
			fmt.Printf("End: %v\n", entry.End)
			fmt.Printf("Text: %q\n", entry.Text)
		}
	}

	if err := rows.Err(); err != nil {
		log.Fatalf("Error iterating rows: %v", err)
	}
} 