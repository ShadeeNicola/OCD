package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var wsMutex sync.Mutex

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
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
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	var req DeployRequest
	if err := conn.ReadJSON(&req); err != nil {
		log.Printf("Failed to read deploy request: %v", err)
		return
	}

	runOCDScriptWithWebSocket(req.FolderPath, conn)
}
