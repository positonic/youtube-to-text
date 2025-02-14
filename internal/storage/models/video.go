package models

import (
	"strings"
	"time"
)

type Video struct {
	ID            string     `json:"id"`
	VideoURL      string     `json:"videoUrl"`
	Slug          string     `json:"slug"`
	Transcription *string    `json:"transcription,omitempty"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	UserID        string     `json:"userId"`
	IsSearchable  bool       `json:"isSearchable"`
}

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
	VideoID       string  `json:"videoId"`
	ChunkText     string  `json:"chunkText"`
	StartPosition int     `json:"startPosition"`
	EndPosition   int     `json:"endPosition"`
	Similarity    float64 `json:"similarity"`
}

type SRTEntry struct {
	Number    int
	Start     time.Duration
	End       time.Duration
	Text      string
}

type Chunk struct {
	Text          string
	StartTime     time.Duration
	EndTime       time.Duration
	Embedding     []float32
}

func ExtractSlugFromURL(url string) string {
	// Find the v= parameter
	vIndex := strings.Index(url, "v=")
	if vIndex == -1 {
		return ""
	}
	
	// Start after "v="
	start := vIndex + 2
	slug := url[start:]
	
	// If there are other parameters, cut at the &
	if ampIndex := strings.Index(slug, "&"); ampIndex != -1 {
		slug = slug[:ampIndex]
	}
	
	return slug
}