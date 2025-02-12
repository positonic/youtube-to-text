package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"jamesfarrell.me/youtube-to-text/internal/storage/models"
	"jamesfarrell.me/youtube-to-text/internal/storage/postgres"
)

type VideoHandler struct {
	repo *postgres.VideoRepository
}

func NewVideoHandler(repo *postgres.VideoRepository) *VideoHandler {
	return &VideoHandler{repo: repo}
}

func (h *VideoHandler) AddVideo(w http.ResponseWriter, r *http.Request) {
	var video models.VideoRequest
	if err := json.NewDecoder(r.Body).Decode(&video); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := h.repo.Create(r.Context(), &video)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *VideoHandler) GetVideo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	videoID := vars["id"]

	video, err := h.repo.GetVideo(videoID)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Video not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(video)
}