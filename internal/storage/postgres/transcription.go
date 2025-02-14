package postgres

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"jamesfarrell.me/youtube-to-text/internal/storage/models"
)

type TranscriptionRepository struct {
	db *sql.DB
}

func NewTranscriptionRepository(db *sql.DB) *TranscriptionRepository {
	return &TranscriptionRepository{db: db}
}

func (r *TranscriptionRepository) SaveChunks(videoID string, chunks []models.Chunk) error {
	stmt, err := r.db.Prepare(`
        INSERT INTO "VideoChunk" (video_id, chunk_text, chunk_embedding, chunk_start_time, chunk_end_time)
        VALUES ($1, $2, $3::float8[], $4, $5)
    `)
	if err != nil {
		return fmt.Errorf("prepare statement failed: %w", err)
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		// Convert []float32 to []float64 for PostgreSQL compatibility
		embedding64 := make([]float64, len(chunk.Embedding))
		for i, v := range chunk.Embedding {
			embedding64[i] = float64(v)
		}

		_, err = stmt.Exec(
			videoID,
			chunk.Text,
			pq.Array(embedding64),
			chunk.StartTime.Seconds(),  // Convert Duration to seconds
			chunk.EndTime.Seconds(),    // Convert Duration to seconds
		)
		if err != nil {
			return fmt.Errorf("chunk insert failed: %w", err)
		}
	}
	return nil
}

func (r *TranscriptionRepository) SaveFullTranscription(videoID string, transcription string) error {
	const updateSQL = `
		UPDATE "Video" 
		SET transcription = $1, status = 'transcribed', "updatedAt" = CURRENT_TIMESTAMP 
		WHERE id = $2
	`
	result, err := r.db.Exec(updateSQL, transcription, videoID)
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

	return nil
}

func (r *TranscriptionRepository) UpdateVideoStatus(videoID string, status string) error {
	const updateSQL = `
		UPDATE "Video" 
		SET status = $1, "updatedAt" = CURRENT_TIMESTAMP 
		WHERE id = $2
	`
	result, err := r.db.Exec(updateSQL, status, videoID)
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

	return nil
}

func (r *TranscriptionRepository) GetByURL(videoURL string) (*models.Video, error) {
	const query = `
        SELECT id, "videoUrl", transcription, status, "isSearchable"
        FROM "Video"
        WHERE "videoUrl" = $1
        AND transcription IS NOT NULL
        ORDER BY "createdAt" DESC
        LIMIT 1
    `
    
    var video models.Video
    err := r.db.QueryRow(query, videoURL).Scan(
        &video.ID,
        &video.VideoURL,
        &video.Transcription,
        &video.Status,
        &video.IsSearchable,
    )
    if err != nil {
        return nil, err
    }
    
    return &video, nil
} 