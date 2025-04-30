package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// SetupRouter configures the HTTP router
func SetupRouter(handler *Handler) http.Handler {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)

	// Performance middleware
	r.Use(middleware.Compress(5)) // Compress responses with level 5 compression
	r.Use(middleware.Timeout(120 * time.Second)) // 2 minute timeout for large requests

	// CORS middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	})

	// Enable gzip/deflate for large responses
	r.Use(middleware.Compress(5, "application/json"))

	// Routes for client connections
	r.Get("/events", handler.HandleEvents)

	// Routes for store management
	r.Post("/store", handler.HandleStoreInitialize)
	r.Patch("/store", handler.HandleStoreUpdate)
	r.Get("/store", handler.HandleStoreQuery)

	// Server information routes
	r.Get("/metrics", handler.HandleMetrics)
	r.Get("/health", handler.HandleHealth) // Use our new health handler

	// Catch-all route for 404s
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		sendJSONError(w, http.StatusNotFound, "not_found", "Resource not found")
	})

	// Method not allowed handler
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		sendJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
	})

	return r
}
