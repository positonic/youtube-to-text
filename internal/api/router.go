package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"jamesfarrell.me/youtube-to-text/internal/api/handlers"
	"jamesfarrell.me/youtube-to-text/internal/api/middleware"
	"jamesfarrell.me/youtube-to-text/internal/storage/postgres"
)

func NewRouter(videoRepo *postgres.VideoRepository) http.Handler {
	r := mux.NewRouter()

	// Public routes
	r.HandleFunc("/health", healthCheck).Methods(http.MethodGet)

	// Protected routes
	protected := r.PathPrefix("").Subrouter()
	videoHandler := handlers.NewVideoHandler(videoRepo)
	
	// Use AuthMiddleware instead of Auth
	protected.Use(middleware.AuthMiddleware)

	// Video routes
	videos := protected.PathPrefix("/videos").Subrouter()
	videos.HandleFunc("", videoHandler.AddVideo).Methods(http.MethodPost)
	videos.HandleFunc("/{id}", videoHandler.GetVideo).Methods(http.MethodGet)

	return r
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
} 