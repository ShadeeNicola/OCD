package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

func handleBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	selectedPath, err := openFolderDialog()
	if err != nil {
		response := BrowseResponse{
			FolderPath: selectedPath,
			Success:    false,
			Message:    err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Add this check for empty path (user cancelled)
	if selectedPath == "" {
		response := BrowseResponse{
			FolderPath: "",
			Success:    false,
			Message:    "",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate that the selected folder is a git repository
	gitDir := filepath.Join(selectedPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		response := BrowseResponse{
			FolderPath: "",
			Success:    false,
			Message:    "Selected folder is not a Git repository (no .git folder found)",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	response := BrowseResponse{
		FolderPath: selectedPath,
		Success:    true,
		Message:    "Folder selected successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := Response{
			Message: "Invalid request format",
			Success: false,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	result := runOCDScript(req.FolderPath)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
