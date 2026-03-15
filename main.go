package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kyleg/skwad/api"
	"github.com/kyleg/skwad/db"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/skwad.db"
	}

	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./static"
	}

	showFPVFCLink := os.Getenv("SHOW_FPVFC_LINK") != "false"

	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Start background cleanup of expired sessions (snapshots metrics before deleting).
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			deleted, err := database.SnapshotAndDeleteExpiredSessions()
			if err != nil {
				log.Printf("Cleanup error: %v", err)
			} else if deleted > 0 {
				log.Printf("Cleaned up %d expired session(s) (snapshots saved)", deleted)
			}
		}
	}()

	srv := api.NewServer(database)

	mux := http.NewServeMux()

	// Health check.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})

	// Config endpoint (feature flags).
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{
			"show_fpvfc_link": showFPVFCLink,
		})
	})

	// API routes.
	mux.HandleFunc("GET /api/usage", srv.HandleUsage)
	mux.HandleFunc("POST /api/sessions", srv.HandleCreateSession)

	mux.HandleFunc("GET /api/sessions/{code}", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		srv.HandleGetSession(w, r, code)
	})

	mux.HandleFunc("POST /api/sessions/{code}/join", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		srv.HandleJoinSession(w, r, code)
	})

	mux.HandleFunc("POST /api/sessions/{code}/preview-join", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		srv.HandlePreviewJoin(w, r, code)
	})

	mux.HandleFunc("GET /api/sessions/{code}/poll", func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		srv.HandlePoll(w, r, code)
	})

	mux.HandleFunc("POST /api/pilots/{id}/preview-channel", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		pilotID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid pilot id", http.StatusBadRequest)
			return
		}
		sessionCode := r.URL.Query().Get("session")
		if sessionCode == "" {
			http.Error(w, "session query parameter required", http.StatusBadRequest)
			return
		}
		srv.HandlePreviewChannelChange(w, r, pilotID, sessionCode)
	})

	mux.HandleFunc("PUT /api/pilots/{id}/channel", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		pilotID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid pilot id", http.StatusBadRequest)
			return
		}
		sessionCode := r.URL.Query().Get("session")
		if sessionCode == "" {
			http.Error(w, "session query parameter required", http.StatusBadRequest)
			return
		}
		srv.HandleUpdatePilotChannel(w, r, pilotID, sessionCode)
	})

	mux.HandleFunc("PUT /api/pilots/{id}/video-system", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		pilotID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid pilot id", http.StatusBadRequest)
			return
		}
		sessionCode := r.URL.Query().Get("session")
		if sessionCode == "" {
			http.Error(w, "session query parameter required", http.StatusBadRequest)
			return
		}
		srv.HandleUpdatePilotVideoSystem(w, r, pilotID, sessionCode)
	})

	mux.HandleFunc("PUT /api/pilots/{id}/callsign", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		pilotID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid pilot id", http.StatusBadRequest)
			return
		}
		sessionCode := r.URL.Query().Get("session")
		if sessionCode == "" {
			http.Error(w, "session query parameter required", http.StatusBadRequest)
			return
		}
		srv.HandleUpdatePilotCallsign(w, r, pilotID, sessionCode)
	})

	mux.HandleFunc("POST /api/sessions/{code}/preview-rebalance", func(w http.ResponseWriter, r *http.Request) {
		srv.HandlePreviewRebalance(w, r, r.PathValue("code"))
	})

	mux.HandleFunc("POST /api/sessions/{code}/rebalance", func(w http.ResponseWriter, r *http.Request) {
		srv.HandleRebalanceAll(w, r, r.PathValue("code"))
	})

	mux.HandleFunc("POST /api/sessions/{code}/transfer-leader", func(w http.ResponseWriter, r *http.Request) {
		srv.HandleTransferLeader(w, r, r.PathValue("code"))
	})

	mux.HandleFunc("POST /api/sessions/{code}/add-pilot", func(w http.ResponseWriter, r *http.Request) {
		srv.HandleAddPilot(w, r, r.PathValue("code"))
	})

	mux.HandleFunc("DELETE /api/pilots/{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		pilotID, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, "invalid pilot id", http.StatusBadRequest)
			return
		}
		sessionCode := r.URL.Query().Get("session")
		if sessionCode == "" {
			http.Error(w, "session query parameter required", http.StatusBadRequest)
			return
		}
		srv.HandleDeactivatePilot(w, r, pilotID, sessionCode)
	})

	// Usage dashboard page.
	mux.HandleFunc("GET /usage", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, staticDir+"/usage.html")
	})

	// Client-side routing: serve index.html for /s/{code} paths.
	mux.HandleFunc("GET /s/{code}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, staticDir+"/index.html")
	})

	// QR alphanumeric mode uppercases URLs — handle /S/{code} the same as /s/{code}.
	mux.HandleFunc("GET /S/{code}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, staticDir+"/index.html")
	})

	// Static file server with no-cache headers so deploys take effect immediately.
	staticFS := http.FileServer(http.Dir(staticDir))
	mux.Handle("GET /", noCacheMiddleware(staticFS))

	// Wrap with CORS middleware.
	handler := corsMiddleware(mux)

	log.Printf("Skwad listening on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}

// noCacheMiddleware sets Cache-Control: no-cache on static file responses.
// Browsers can still cache, but must revalidate with the server (If-Modified-Since)
// before using cached content, ensuring deploys take effect immediately.
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers for /api/ paths and handles OPTIONS preflight.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Pilot-ID")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
