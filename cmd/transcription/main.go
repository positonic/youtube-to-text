package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"jamesfarrell.me/youtube-to-text/internal/storage/db"
	"jamesfarrell.me/youtube-to-text/internal/storage/postgres"
	"jamesfarrell.me/youtube-to-text/internal/transcription"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("Error loading .env file: %v\n", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	apiKey := os.Getenv("LEMONFOX_API_KEY")

	if apiKey == "" || dbURL == "" {
		log.Fatal("LEMONFOX_API_KEY and DATABASE_URL environment variables must be set")
	}

	// Initialize database connection
	database, err := db.NewConnection(db.Config{URL: dbURL})
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	log.Printf("Connected to database: %s", db.MaskDatabaseURL(dbURL))

	transcriptionRepo := postgres.NewTranscriptionRepository(database)
	transcriptionSvc := transcription.NewService(transcriptionRepo, apiKey, dbURL)

	if err := transcriptionSvc.ListenForNewVideos(); err != nil {
		log.Fatalf("Service error: %v", err)
	}
}
