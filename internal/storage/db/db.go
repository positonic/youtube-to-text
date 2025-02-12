package db

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type Config struct {
	URL string
}

// NewConnection creates and verifies a new database connection
func NewConnection(cfg Config) (*sql.DB, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	// Set reasonable defaults for connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	log.Printf("Successfully connected to database")
	return db, nil
}

// MaskDatabaseURL masks sensitive information in database URL for logging
func MaskDatabaseURL(url string) string {
	if url == "" {
		return ""
	}
	// Simple masking - in production you might want more sophisticated masking
	return "postgres://[masked]@[masked]"
} 