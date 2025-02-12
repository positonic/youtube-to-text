package api

import "time"

type VideoRequest struct {
    URL          string `json:"url"`
    IsSearchable bool   `json:"isSearchable"`
}

type SearchRequest struct {
    Query string `json:"query"`
    Limit int    `json:"limit"`
}

type SearchResponse struct {
    Results []SearchResult `json:"results"`
}

type SearchResult struct {
    VideoID        string  `json:"videoId"`
    ChunkText     string  `json:"chunkText"`
    StartPosition int     `json:"startPosition"`
    EndPosition   int     `json:"endPosition"`
    Similarity    float64 `json:"similarity"`
}

type Video struct {
    ID            string     `json:"id"`
    VideoURL      string     `json:"videoUrl"`
    Transcription *string    `json:"transcription,omitempty"`
    Status        string     `json:"status"`
    CreatedAt     time.Time  `json:"createdAt"`
    UpdatedAt     time.Time  `json:"updatedAt"`
    UserID        int        `json:"userId"`
    IsSearchable  bool       `json:"isSearchable"`
}