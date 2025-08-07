package httpapi

import (
    "encoding/json"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "ocd-gui/internal/executor"
    "ocd-gui/internal/progress"
    "ocd-gui/internal/ui"
)

func HandleBrowse(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" { http.Error(w, "Method not allowed", http.StatusMethodNotAllowed); return }
    selectedPath, err := ui.OpenFolderDialog()
    if err != nil {
        response := progress.BrowseResponse{FolderPath: selectedPath, Success: false, Message: err.Error()}
        w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(response); return
    }
    if selectedPath == "" { w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(progress.BrowseResponse{FolderPath: "", Success: false, Message: ""}); return }
    gitDir := filepath.Join(selectedPath, ".git")
    if _, err := os.Stat(gitDir); os.IsNotExist(err) {
        w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(progress.BrowseResponse{FolderPath: "", Success: false, Message: "Selected folder is not a Git repository (no .git folder found)"}); return
    }
    w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(progress.BrowseResponse{FolderPath: selectedPath, Success: true, Message: "Folder selected successfully"})
}

func HandleDeploy(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" { http.Error(w, "Method not allowed", http.StatusMethodNotAllowed); return }
    var req progress.DeployRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil { w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(progress.Response{Message: "Invalid request format", Success: false}); return }
    result := executor.RunOCDScript(req.FolderPath)
    w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(result)
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" { http.Error(w, "Method not allowed", http.StatusMethodNotAllowed); return }
    health := map[string]interface{}{"status": "healthy", "timestamp": time.Now().Unix(), "version": "1.0.0"}
    w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(health)
}


