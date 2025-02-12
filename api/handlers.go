package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/pgvector/pgvector-go"
	"jamesfarrell.me/youtube-to-text/api/embeddings"
)

func (r *Router) addVideo(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Received request to add video")
	
	var video VideoRequest
	if err := json.NewDecoder(req.Body).Decode(&video); err != nil {
		fmt.Printf("Error decoding request body: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	fmt.Printf("Decoded video request: %+v\n", video)
	
	// Insert into Video table
	const insertSQL = `
		INSERT INTO "Video" (id, "videoUrl", status, "isSearchable", "createdAt", "updatedAt", "userId")
		VALUES (gen_random_uuid(), $1, 'pending', $2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)
		RETURNING id
	`
	
	var videoID string
	err := r.db.QueryRow(insertSQL, video.URL, video.IsSearchable).Scan(&videoID)
	if err != nil {
		fmt.Printf("Error inserting video: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Successfully inserted video with ID: %s\n", videoID)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": videoID})
}

func (r *Router) searchVideos(w http.ResponseWriter, req *http.Request) {
	fmt.Println("Search endpoint hit")
	
	var searchReq SearchRequest
	if err := json.NewDecoder(req.Body).Decode(&searchReq); err != nil {
		fmt.Printf("Error decoding request: %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	fmt.Printf("Search request received: %+v\n", searchReq)

	if searchReq.Limit == 0 {
		searchReq.Limit = 5 // default limit
	}

	// Get embedding for search query
	embedding, err := embeddings.GetEmbedding(searchReq.Query, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		fmt.Printf("Error getting embedding: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert []float32 to pgvector.Vector
	vector := pgvector.NewVector(embedding)

	// Search for similar chunks with cosine similarity
	rows, err := r.db.Query(`
		WITH query_embedding AS (
			SELECT $1::vector AS vec
		)
		SELECT 
			v.id as video_id,
			vc.chunk_text,
			vc.chunk_start,
			vc.chunk_end,
			1 - (vc.chunk_embedding <=> (SELECT vec FROM query_embedding)) as similarity
		FROM video_chunks vc
		JOIN "Video" v ON v.id = vc.video_id
		WHERE v.status = 'completed'
		ORDER BY vc.chunk_embedding <=> (SELECT vec FROM query_embedding)
		LIMIT $2
	`, vector, searchReq.Limit)
	if err != nil {
		fmt.Printf("Error querying database: %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		err := rows.Scan(
			&result.VideoID,
			&result.ChunkText,
			&result.StartPosition,
			&result.EndPosition,
			&result.Similarity,
		)
		if err != nil {
			fmt.Printf("Error scanning row: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, result)
	}

	fmt.Printf("Found %d results\n", len(results))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SearchResponse{Results: results})
}

func (r *Router) getVideo(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	videoID := vars["id"]

	// Query to get video details - note the quoted column names
	const query = `
		SELECT id, "videoUrl", transcription, status, "isSearchable", 
			   "createdAt", "updatedAt"
		FROM "Video"
		WHERE id = $1
	`

	var video struct {
		ID           string    `json:"id"`
		VideoURL     string    `json:"videoUrl"`
		Transcription *string   `json:"transcription,omitempty"`
		Status       string    `json:"status"`
		IsSearchable bool      `json:"isSearchable"`
		CreatedAt    time.Time `json:"createdAt"`
		UpdatedAt    time.Time `json:"updatedAt"`
	}

	err := r.db.QueryRow(query, videoID).Scan(
		&video.ID,
		&video.VideoURL,
		&video.Transcription,
		&video.Status,
		&video.IsSearchable,
		&video.CreatedAt,
		&video.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Video not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(video)
}

func (r *Router) listVideos(w http.ResponseWriter, req *http.Request) {
	var videos []Video  // Initialize the slice
	
	const query = `
		SELECT id, "videoUrl", transcription, status, 
			   "createdAt", "updatedAt", "userId", "isSearchable"
		FROM "Video"
		ORDER BY "createdAt" DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var video Video
		err := rows.Scan(
			&video.ID,
			&video.VideoURL,
			&video.Transcription,
			&video.Status,
			&video.CreatedAt,
			&video.UpdatedAt,
			&video.UserID,
			&video.IsSearchable,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		videos = append(videos, video)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(videos)
} 