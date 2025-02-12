package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"jamesfarrell.me/youtube-to-text/internal/storage/models"
)

type VideoRepository struct {
	db *sql.DB
}

func NewVideoRepository(db *sql.DB) *VideoRepository {
	return &VideoRepository{db: db}
}

func (r *VideoRepository) Create(ctx context.Context, video *models.VideoRequest) (string, error) {
	const query = `
		INSERT INTO "Video" (id, "videoUrl", slug, status, "isSearchable", "createdAt", "updatedAt", "userId")
		VALUES (gen_random_uuid(), $1, $2, 'pending', $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $4)
		RETURNING id
	`

	userID := os.Getenv("VIDEO_OWNER_USER_ID")
	if userID == "" {
		return "", fmt.Errorf("VIDEO_OWNER_USER_ID environment variable must be set")
	}

	var id string
	err := r.db.QueryRowContext(ctx, query, 
		video.URL, 
		models.ExtractSlugFromURL(video.URL), 
		video.IsSearchable,
		userID,
	).Scan(&id)
	return id, err
}

func (r *VideoRepository) Get(ctx context.Context, id string) (*models.Video, error) {
	const query = `
		SELECT id, "videoUrl", transcription, status, "isSearchable", 
			   "createdAt", "updatedAt", "userId"
		FROM "Video"
		WHERE id = $1
	`

	var video models.Video
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&video.ID,
		&video.VideoURL,
		&video.Transcription,
		&video.Status,
		&video.IsSearchable,
		&video.CreatedAt,
		&video.UpdatedAt,
		&video.UserID,
	)
	if err != nil {
		return nil, err
	}
	return &video, nil
}

