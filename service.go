// Service that listens for new videos and transcribes them
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/lib/pq"
	"jamesfarrell.me/youtube-to-text/api" // Add this import
	"jamesfarrell.me/youtube-to-text/api/embeddings"
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
    
    // Start the HTTP server in a goroutine
    go func() {
        router := api.NewRouter(db)  // Use the new router
        fmt.Println("Starting HTTP server on :8080...")
        if err := http.ListenAndServe(":8080", router); err != nil {
            log.Fatalf("HTTP server error: %v", err)
        }
    }()

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
        IsSearchable  bool      `json:"isSearchable"`
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
    if err := saveTranscription(video.ID, transcription, video.IsSearchable); err != nil {
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

func saveTranscription(videoID string, transcription string, isSearchable bool) error {
    // First save the full transcription as before
    if err := saveFullTranscription(videoID, transcription); err != nil {
        return err
    }

    // Only process chunks if isSearchable is true
    if isSearchable {
        chunks := chunkText(transcription, 500, 50) // 500 chars with 50 char overlap
        if err := saveChunks(videoID, chunks); err != nil {
            // If chunk processing fails, update the video status to reflect the error
            updateErr := updateVideoStatus(videoID, "chunk_processing_failed")
            if updateErr != nil {
                // Log both errors but return the original error
                log.Printf("Failed to update video status: %v", updateErr)
            }
            return fmt.Errorf("failed to save chunks: %w", err)
        }
        
        // Update video status to indicate successful chunk processing
        if err := updateVideoStatus(videoID, "completed"); err != nil {
            return fmt.Errorf("failed to update video status: %w", err)
        }
    }

    return nil
}

type Chunk struct {
    Text          string
    StartPosition int
    EndPosition   int
    Embedding     []float32
}

func chunkText(text string, chunkSize, overlap int) []Chunk {
    var chunks []Chunk
    sentences := strings.Split(text, ".")
    currentChunk := ""
    startPos := 0
    
    for _, sentence := range sentences {
        sentence = strings.TrimSpace(sentence) + "."
        if len(currentChunk)+len(sentence) > chunkSize && len(currentChunk) > 0 {
            chunks = append(chunks, Chunk{
                Text:          currentChunk,
                StartPosition: startPos,
                EndPosition:   startPos + len(currentChunk),
            })
            // Move back by overlap
            currentChunk = currentChunk[len(currentChunk)-overlap:] + sentence
            startPos = startPos + len(currentChunk) - overlap
        } else {
            currentChunk += sentence
        }
    }
    
    // Add the last chunk if there's anything left
    if len(currentChunk) > 0 {
        chunks = append(chunks, Chunk{
            Text:          currentChunk,
            StartPosition: startPos,
            EndPosition:   startPos + len(currentChunk),
        })
    }
    
    return chunks
}

func saveChunks(videoID string, chunks []Chunk) error {
    // Prepare the statement
    stmt, err := db.Prepare(`
        INSERT INTO video_chunks (video_id, chunk_text, chunk_embedding, chunk_start, chunk_end)
        VALUES ($1, $2, $3::float8[], $4, $5)
    `)
    if err != nil {
        return fmt.Errorf("prepare statement failed: %w", err)
    }
    defer stmt.Close()

    for _, chunk := range chunks {
        embedding, err := embeddings.GetEmbedding(chunk.Text, os.Getenv("OPENAI_API_KEY"))
        if err != nil {
            return err
        }
        
        // Convert []float32 to []float64
        embedding64 := make([]float64, len(embedding))
        for i, v := range embedding {
            embedding64[i] = float64(v)
        }
        
        _, err = stmt.Exec(
            videoID,
            chunk.Text,
            pq.Array(embedding64), // Use pq.Array to properly encode the slice
            chunk.StartPosition,
            chunk.EndPosition,
        )
        if err != nil {
            return fmt.Errorf("chunk insert failed: %w", err)
        }
    }

    return nil
}

// Example function to query similar chunks
func findSimilarChunks(query string, limit int) ([]Chunk, error) {
    queryEmbedding, err := embeddings.GetEmbedding(query, os.Getenv("OPENAI_API_KEY"))
    if err != nil {
        return nil, err
    }

    rows, err := db.Query(`
        SELECT chunk_text, chunk_start, chunk_end
        FROM video_chunks
        ORDER BY chunk_embedding <=> $1
        LIMIT $2
    `, queryEmbedding, limit)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var chunks []Chunk
    for rows.Next() {
        var chunk Chunk
        err := rows.Scan(&chunk.Text, &chunk.StartPosition, &chunk.EndPosition)
        if err != nil {
            return nil, err
        }
        chunks = append(chunks, chunk)
    }

    return chunks, nil
}

func saveFullTranscription(videoID string, transcription string) error {
    const updateSQL = `
        UPDATE "Video" 
        SET transcription = $1, "updatedAt" = CURRENT_TIMESTAMP 
        WHERE id = $2
    `
    
    result, err := db.Exec(updateSQL, transcription, videoID)
    if err != nil {
        return fmt.Errorf("failed to save transcription: %w", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to get rows affected: %w", err)
    }

    if rows == 0 {
        return fmt.Errorf("no video found with ID: %s", videoID)
    }

    return nil
} 