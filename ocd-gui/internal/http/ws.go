package httpapi

import (
    "net/http"
    "net/url"
    "sync"

    "github.com/gorilla/websocket"

    "ocd-gui/internal/executor"
    "ocd-gui/internal/config"
)

var wsMutex sync.Mutex

var upgrader = websocket.Upgrader{ CheckOrigin: func(r *http.Request) bool { return true } }

func writeToWebSocket(conn *websocket.Conn, data interface{}) error { wsMutex.Lock(); defer wsMutex.Unlock(); return conn.WriteJSON(data) }

func HandleWebSocketDeploy(cfg *config.Config) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Strict origin check if configured (kept behavior)
        origin := r.Header.Get("Origin")
        if origin != "" && !(len(cfg.AllowedOrigins) == 1 && cfg.AllowedOrigins[0] == "*") {
            originURL, err := url.Parse(origin)
            if err != nil { http.Error(w, "Forbidden", http.StatusForbidden); return }
            host := originURL.Hostname()
            allowed := false
            for _, allowedOrigin := range cfg.AllowedOrigins { if host == allowedOrigin { allowed = true; break } }
            if !allowed { http.Error(w, "Forbidden", http.StatusForbidden); return }
        }

        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil { return }
        defer conn.Close()

        var req struct{ FolderPath string `json:"folderPath"` }
        if err := conn.ReadJSON(&req); err != nil { writeToWebSocket(conn, map[string]interface{}{"type":"complete","content":"Failed to read deployment request","success":false}); return }
        if req.FolderPath == "" { writeToWebSocket(conn, map[string]interface{}{"type":"complete","content":"Folder path is required","success":false}); return }

        executor.RunOCDScriptWithWebSocket(req.FolderPath, conn, func(c *websocket.Conn, payload interface{}) error { return writeToWebSocket(c, payload) })
    }
}


