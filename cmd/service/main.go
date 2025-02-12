package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"jamesfarrell.me/youtube-to-text/internal/api"
	"jamesfarrell.me/youtube-to-text/internal/storage/db"
	"jamesfarrell.me/youtube-to-text/internal/storage/postgres"
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
	database, err := db.NewConnection(db.Config{URL: dbURL})
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Initialize repositories
	videoRepo := postgres.NewVideoRepository(database)

	// Initialize router with dependencies
	router := api.NewRouter(videoRepo)

	// Start the HTTP server
	log.Println("Starting HTTP server on :8080...")
	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
} 