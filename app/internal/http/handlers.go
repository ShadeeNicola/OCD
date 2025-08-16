package httpapi

import (
    "encoding/json"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "app/internal/executor"
    "app/internal/progress"
    "app/internal/ui"
    "app/internal/version"
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

func HandleDeploy(runner *executor.Runner) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" { http.Error(w, "Method not allowed", http.StatusMethodNotAllowed); return }
        var req progress.DeployRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil { w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(progress.Response{Message: "Invalid request format", Success: false}); return }
        result := runner.RunOCDScript(req.FolderPath)
        w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(result)
    }
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" { http.Error(w, "Method not allowed", http.StatusMethodNotAllowed); return }
    health := map[string]interface{}{"status": "healthy", "timestamp": time.Now().Unix(), "version": version.Version, "commit": version.Commit, "date": version.Date}
    w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(health)
}

// Jenkins handlers have been moved to handlers_jenkins.go for better organization

func HandleEKSClusters(w http.ResponseWriter, r *http.Request) {
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Use the bash script that follows the same proxy pattern as other scripts
    scriptPath := "../deploy-scripts/scripts/list-eks-clusters.sh"
    cmd := exec.Command("bash", scriptPath)
    output, err := cmd.Output()
    if err != nil {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "success": false,
            "message": "Failed to list EKS clusters: " + err.Error(),
            "clusters": []string{},
        })
        return
    }

    // Parse the output to extract cluster names
    lines := strings.Split(strings.TrimSpace(string(output)), "\n")
    var clusters []string
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line != "" && !strings.HasPrefix(line, "NAME") {
            clusters = append(clusters, line)
        }
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "message":  "EKS clusters retrieved successfully",
        "clusters": clusters,
    })
}