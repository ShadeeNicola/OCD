package main

import (
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
)

var wsMutex sync.Mutex

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Allow requests without origin (like from Postman, curl, etc.)
		}

		// Check if all origins are allowed
		if len(appConfig.AllowedOrigins) == 1 && appConfig.AllowedOrigins[0] == "*" {
			return true
		}

		originURL, err := url.Parse(origin)
		if err != nil {
			return false
		}

		host := originURL.Hostname()

		// Check against configured allowed origins
		for _, allowedOrigin := range appConfig.AllowedOrigins {
			if host == allowedOrigin {
				return true
			}
		}

		return false
	},
}

func writeToWebSocket(conn *websocket.Conn, data interface{}) error {
	wsMutex.Lock()
	defer wsMutex.Unlock()
	return conn.WriteJSON(data)
}

func handleWebSocketDeploy(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		appLogger.Errorf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	var req DeployRequest
	if err := conn.ReadJSON(&req); err != nil {
		appLogger.Errorf("Failed to read deploy request: %v", err)
		// Send error message to client before closing
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: "Failed to read deployment request",
			Success: false,
		})
		return
	}

	// Validate request before processing
	if req.FolderPath == "" {
		appLogger.Warn("Empty folder path received in WebSocket request")
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: "Folder path is required",
			Success: false,
		})
		return
	}

	// CommandExecutor will handle the logging from here
	runOCDScriptWithWebSocket(req.FolderPath, conn)
}
