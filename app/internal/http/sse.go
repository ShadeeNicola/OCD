package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"app/internal/config"
	"app/internal/executor"
	"app/internal/progress"
)

// Session management for deployments
type DeploymentSession struct {
	ID         string
	Cancel     context.CancelFunc
	Writer     chan []byte
	Done       chan struct{}
	FolderPath string
}

var (
	sessions    = make(map[string]*DeploymentSession)
	sessionsMux = sync.RWMutex{}
)

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	return fmt.Sprintf("deploy_%d", time.Now().UnixNano())
}

// HandleDeployStart initiates a new deployment session
func HandleDeployStart(configuration *config.Config, runner *executor.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req progress.DeployRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		if req.FolderPath == "" {
			http.Error(w, "Folder path is required", http.StatusBadRequest)
			return
		}

		// Create new session
		sessionID := generateSessionID()
		ctx, cancel := context.WithCancel(context.Background())

		session := &DeploymentSession{
			ID:         sessionID,
			Cancel:     cancel,
			Writer:     make(chan []byte, 100), // Buffer for SSE messages
			Done:       make(chan struct{}),
			FolderPath: req.FolderPath,
		}

		// Store session
		sessionsMux.Lock()
		sessions[sessionID] = session
		sessionsMux.Unlock()

		// Start deployment in background
		go func(r *executor.Runner) {
			defer func() {
				close(session.Done)
				sessionsMux.Lock()
				delete(sessions, sessionID)
				sessionsMux.Unlock()
				close(session.Writer)
			}()

			r.RunOCDScriptWithSSE(ctx, req.FolderPath, session.Writer)
		}(runner)

		// Return session ID
		response := map[string]string{"sessionId": sessionID}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// HandleDeployStream provides SSE stream for deployment progress
func HandleDeployStream(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from URL path
	sessionID := r.URL.Path[len("/api/deploy/stream/"):]
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get session
	sessionsMux.RLock()
	session, exists := sessions[sessionID]
	sessionsMux.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send initial connection confirmation
	fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected","sessionId":"`+sessionID+`"}`)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Stream messages until deployment completes
	for {
		select {
		case message, ok := <-session.Writer:
			if !ok {
				// Channel closed, deployment finished
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", string(message))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case <-session.Done:
			// Deployment completed
			return
		case <-r.Context().Done():
			// Client disconnected
			return
		case <-time.After(30 * time.Second):
			// Send keepalive
			fmt.Fprintf(w, "data: %s\n\n", `{"type":"keepalive"}`)
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// HandleDeployCancel cancels an active deployment
func HandleDeployCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from URL path
	sessionID := r.URL.Path[len("/api/deploy/cancel/"):]
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	// Get and cancel session
	sessionsMux.RLock()
	session, exists := sessions[sessionID]
	sessionsMux.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Cancel the deployment
	session.Cancel()

	// Send cancellation confirmation
	response := map[string]string{"status": "cancelled", "sessionId": sessionID}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
