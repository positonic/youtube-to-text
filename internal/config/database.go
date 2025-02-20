package config

import (
	"fmt"
	"log"
	"os"
)

// GetDatabaseURL returns the database URL based on the provided identifier
// If no identifier is provided, it defaults to "DEFAULT"
func GetDatabaseURL() string {
	// Get the database identifier from command line args
	dbID := "DEFAULT"
	if len(os.Args) > 1 {
		dbID = os.Args[1]
	}

	// Construct the environment variable name
	dbURLKey := fmt.Sprintf("DATABASE_URL_%s", dbID)
	dbURL := os.Getenv(dbURLKey)
	
	if dbURL == "" {
		log.Fatalf("No database URL found for %s", dbURLKey)
	}

	return dbURL
} 