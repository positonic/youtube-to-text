// Service that listens for new videos and transcribes them
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lib/pq"
)

var db *sql.DB // Global database connection pool

func main() {
    // Load .env file
    if err := godotenv.Load(); err != nil {
        fmt.Printf("Error loading .env file: %v\n", err)
    }

    // Get environment variables
    dbURL := os.Getenv("DATABASE_URL")
    apiKey := os.Getenv("LEMONFOX_API_KEY")
    
    fmt.Println("Starting transcription service...")
    
    if apiKey == "" || dbURL == "" {
        fmt.Println("Error: LEMONFOX_API_KEY and DATABASE_URL environment variables must be set")
        return
    }

    // Initialize database connection pool
    var err error
    db, err = sql.Open("postgres", dbURL)
    if err != nil {
        fmt.Printf("Error connecting to database: %v\n", err)
        return
    }
    defer db.Close()

    // Test the connection
    if err := db.Ping(); err != nil {
        fmt.Printf("Error pinging database: %v\n", err)
        return
    }

    fmt.Printf("Connecting to database: %s\n", maskDatabaseURL(dbURL))
    
    // Start listening for new videos
    if err := listenForNewVideos(dbURL, apiKey); err != nil {
        fmt.Printf("Service error: %v\n", err)
    }
}

// Helper function to mask sensitive information
func maskDatabaseURL(dbURL string) string {
    // Return only the host/database part, mask credentials
    parts := strings.Split(dbURL, "@")
    if len(parts) > 1 {
        return "..." + parts[len(parts)-1]
    }
    return "...masked..."
}

func listenForNewVideos(dbURL string, apiKey string) error {
    listener := pq.NewListener(dbURL, 10*time.Second, time.Minute,
        func(ev pq.ListenerEventType, err error) {
            if err != nil {
                fmt.Printf("Listen error: %v\n", err)
            }
        })
    defer listener.Close()

    err := listener.Listen("new_video")
    if err != nil {
        return fmt.Errorf("listen error: %w", err)
    }

    fmt.Printf("Successfully connected to database. Listening on 'new_video' channel...\n")
    for {
        select {
        case n := <-listener.Notify:
            if n == nil {
                fmt.Println("Received nil notification, continuing...")
                continue
            }
            fmt.Printf("Received new video notification: %s\n", n.Extra)
            // Process the new video notification
            if err := processVideoNotification(n.Extra, apiKey); err != nil {
                fmt.Printf("Error processing video: %v\n", err)
            } else {
                fmt.Println("Successfully processed video notification")
            }
        case <-time.After(time.Minute):
            fmt.Println("Sending ping to keep connection alive...")
            go listener.Ping()
        }
    }
}

func processVideoNotification(jsonData string, apiKey string) error {
    // Parse the notification data
    var video struct {
        ID            string    `json:"id"`
        VideoURL      string    `json:"videoUrl"`
        Transcription *string   `json:"transcription"`
        Status        string    `json:"status"`
        CreatedAt     string    `json:"createdAt"`
        UpdatedAt     string    `json:"updatedAt"`
        UserID        int       `json:"userId"`
    }
    if err := json.Unmarshal([]byte(jsonData), &video); err != nil {
        return fmt.Errorf("json parse error: %w", err)
    }

    fmt.Printf("Processing video ID: %s, URL: %s\n", video.ID, video.VideoURL)
    
    // Update status to processing
    if err := updateVideoStatus(video.ID, "processing"); err != nil {
        return fmt.Errorf("failed to update status to processing: %w", err)
    }
    
    // Process the video
    outputPath := fmt.Sprintf("./temp_%s.mp3", video.ID)
    defer os.Remove(outputPath) // Clean up temp file

    fmt.Printf("Downloading audio to: %s\n", outputPath)
    if err := downloadAudio(video.VideoURL, outputPath); err != nil {
        // Update status to failed
        updateVideoStatus(video.ID, "failed")
        return fmt.Errorf("download error: %w", err)
    }
    fmt.Println("Audio download completed successfully")

    fmt.Println("Sending audio to Lemonfox for transcription...")
    transcription, err := sendAudioToLemonfox(outputPath, apiKey)
    if err != nil {
        updateVideoStatus(video.ID, "failed")
        return fmt.Errorf("transcription error: %w", err)
    }
    fmt.Println("Transcription completed successfully")

    // Save transcription to database
    if err := saveTranscription(video.ID, transcription); err != nil {
        return fmt.Errorf("failed to save transcription: %w", err)
    }

    // Update status to completed
    if err := updateVideoStatus(video.ID, "completed"); err != nil {
        return fmt.Errorf("failed to update status to completed: %w", err)
    }	

    return nil
}

func updateVideoStatus(videoID string, status string) error {
    const updateSQL = `
        UPDATE "Video" 
        SET status = $1, "updatedAt" = CURRENT_TIMESTAMP 
        WHERE id = $2
    `
    
    result, err := db.Exec(updateSQL, status, videoID)
    if err != nil {
        return fmt.Errorf("failed to execute update: %w", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to get rows affected: %w", err)
    }

    if rows == 0 {
        return fmt.Errorf("no video found with ID: %s", videoID)
    }

    fmt.Printf("Updated video %s status to: %s\n", videoID, status)
    return nil
}

func saveTranscription(videoID string, transcription string) error {
    const saveSQL = `
        UPDATE "Video" 
        SET transcription = $1, status = 'completed', "updatedAt" = CURRENT_TIMESTAMP 
        WHERE id = $2
    `
    
    result, err := db.Exec(saveSQL, transcription, videoID)
    if err != nil {
        return fmt.Errorf("failed to execute update: %w", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to get rows affected: %w", err)
    }

    if rows == 0 {
        return fmt.Errorf("no video found with ID: %s", videoID)
    }

    fmt.Printf("Saved transcription for video %s\n", videoID)
    return nil
} 