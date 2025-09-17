package httpapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"app/internal/executor"
	"app/internal/progress"
	"app/internal/ui"
	"app/internal/version"
)

func HandleBrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	selectedPath, err := ui.OpenFolderDialog()
	if err != nil {
		response := progress.BrowseResponse{FolderPath: selectedPath, Success: false, Message: err.Error()}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
		return
	}
	if selectedPath == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(progress.BrowseResponse{FolderPath: "", Success: false, Message: ""})
		return
	}
	gitDir := filepath.Join(selectedPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(progress.BrowseResponse{FolderPath: "", Success: false, Message: "Selected folder is not a Git repository (no .git folder found)"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(progress.BrowseResponse{FolderPath: selectedPath, Success: true, Message: "Folder selected successfully"})
}

func HandleDeploy(runner *executor.Runner) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req progress.DeployRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(progress.Response{Message: "Invalid request format", Success: false})
			return
		}
		result := runner.RunOCDScript(req.FolderPath)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	}
}

func HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	health := map[string]interface{}{"status": "healthy", "timestamp": time.Now().Unix(), "version": version.Version, "commit": version.Commit, "date": version.Date}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(health)
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
			"success":  false,
			"message":  "Failed to list EKS clusters: " + err.Error(),
			"clusters": []string{},
		})
		return
	}

	// Parse the output to extract cluster names
	// AWS CLI with --output text returns space/tab-separated values on a single line
	outputStr := strings.TrimSpace(string(output))
	var clusters []string
	if outputStr != "" {
		// Split by whitespace to handle space or tab-separated cluster names
		clusterNames := strings.Fields(outputStr)
		for _, name := range clusterNames {
			name = strings.TrimSpace(name)
			if name != "" && !strings.HasPrefix(name, "NAME") {
				clusters = append(clusters, name)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "EKS clusters retrieved successfully",
		"clusters": clusters,
	})
}

type GitBranch struct {
	Name       string `json:"name"`
	LastCommit string `json:"lastCommit"`
}

func HandleGitBranchesCustomization(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Try to get credentials from request headers or environment
	username := r.Header.Get("X-Bitbucket-Username")
	token := r.Header.Get("X-Bitbucket-Token")

	// Fallback to environment variables if headers not provided
	if username == "" {
		username = os.Getenv("BITBUCKET_USERNAME")
	}
	if token == "" {
		token = os.Getenv("BITBUCKET_TOKEN")
	}

	var branches []GitBranch
	var err error

	if username != "" && token != "" {
		// Use Bitbucket REST API with authentication
		branches, err = fetchBranchesFromBitbucketAPI(username, token)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":  false,
				"message":  "Failed to fetch branches from customization repository: " + err.Error(),
				"branches": []GitBranch{},
			})
			return
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      false,
			"message":      "Authentication required. Please configure Bitbucket credentials in Settings.",
			"branches":     []GitBranch{},
			"requiresAuth": true,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Git branches retrieved successfully",
		"branches": branches,
	})
}

type BitbucketBranch struct {
	ID           string            `json:"id"`
	DisplayID    string            `json:"displayId"`
	Type         string            `json:"type"`
	LatestCommit string            `json:"latestCommit"`
	Metadata     BitbucketMetadata `json:"metadata"`
}

type BitbucketMetadata struct {
	LatestCommitMetadata LatestCommitMetadata `json:"com.atlassian.bitbucket.server.bitbucket-branch:latest-commit-metadata"`
}

type LatestCommitMetadata struct {
	ID                 string `json:"id"`
	DisplayID          string `json:"displayId"`
	AuthorTimestamp    int64  `json:"authorTimestamp"`
	CommitterTimestamp int64  `json:"committerTimestamp"`
	Message            string `json:"message"`
}

type BitbucketResponse struct {
	Size          int               `json:"size"`
	Limit         int               `json:"limit"`
	IsLastPage    bool              `json:"isLastPage"`
	Values        []BitbucketBranch `json:"values"`
	Start         int               `json:"start"`
	NextPageStart int               `json:"nextPageStart"`
}

// HandleRNCreate handles RN creation requests
func HandleRNCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Branch string `json:"branch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Invalid request format",
		})
		return
	}

	if req.Branch == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Branch is required",
		})
		return
	}

	// For now, return a placeholder response
	// The actual implementation will be completed with the Jenkins integration
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "RN creation initiated for branch: " + req.Branch,
		"jobUrl":  "http://ilososp030.corp.amdocs.com:7070/job/ATT_Storage_Creation/",
	})
}

func fetchBranchesFromBitbucketAPI(username, token string) ([]GitBranch, error) {
	// Create HTTP client with TLS config
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Bitbucket REST API URL for branches with metadata for commit timestamps
	url := "https://ossbucket:7990/rest/api/1.0/projects/ATTSVO/repos/customization/branches?limit=100&details=true"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set basic auth
	req.SetBasicAuth(username, token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	var bitbucketResp BitbucketResponse
	if err := json.Unmarshal(body, &bitbucketResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// Convert to our GitBranch format with commit timestamps
	var branchesWithTimestamp []struct {
		GitBranch
		CommitTimestamp int64
	}

	for _, branch := range bitbucketResp.Values {
		commitHash := branch.LatestCommit
		if len(commitHash) > 8 {
			commitHash = commitHash[:8]
		}

		branchWithTimestamp := struct {
			GitBranch
			CommitTimestamp int64
		}{
			GitBranch: GitBranch{
				Name:       branch.DisplayID,
				LastCommit: commitHash,
			},
			CommitTimestamp: branch.Metadata.LatestCommitMetadata.CommitterTimestamp,
		}
		branchesWithTimestamp = append(branchesWithTimestamp, branchWithTimestamp)
	}

	// Sort branches by commit timestamp (newest first)
	sort.Slice(branchesWithTimestamp, func(i, j int) bool {
		return branchesWithTimestamp[i].CommitTimestamp > branchesWithTimestamp[j].CommitTimestamp
	})

	// Extract just the GitBranch structs
	var branches []GitBranch
	for _, branchWithTimestamp := range branchesWithTimestamp {
		branches = append(branches, branchWithTimestamp.GitBranch)
	}

	return branches, nil
}
