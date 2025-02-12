package api

import (
	"database/sql"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

type Router struct {
	*mux.Router
	db *sql.DB
}

func NewRouter(db *sql.DB) *Router {
	r := &Router{
		Router: mux.NewRouter(),
		db:     db,
	}
	
	// Public routes
	public := r.Router.PathPrefix("/public").Subrouter()
	public.HandleFunc("/health", r.healthCheck).Methods(http.MethodGet)
	
	// Protected routes
	protected := r.Router.PathPrefix("").Subrouter()
	protected.Use(r.authMiddleware)
	
	// Video routes
	videos := protected.PathPrefix("/videos").Subrouter()
	videos.HandleFunc("", r.listVideos).Methods(http.MethodGet)
	videos.HandleFunc("", r.addVideo).Methods(http.MethodPost)
	videos.HandleFunc("/{id}", r.getVideo).Methods(http.MethodGet)
 
	// Search routes
	search := protected.PathPrefix("/search").Subrouter()
	search.HandleFunc("", r.searchVideos).Methods(http.MethodPost)
	
	return r
}

func (r *Router) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		apiKey := req.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "Missing API key", http.StatusUnauthorized)
			return
		}
		
		// In production, you'd want to validate the API key against a database
		if apiKey != os.Getenv("SERVICE_API_KEY") {
			http.Error(w, "Invalid API key", http.StatusUnauthorized)
			return
		}
		
		next.ServeHTTP(w, req)
	})
}

func (r *Router) healthCheck(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
} 