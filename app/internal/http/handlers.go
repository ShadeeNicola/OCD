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
	"runtime"
	"sort"
	"strings"
	"time"

	"app/internal/executor"
	"app/internal/progress"
	"app/internal/ui"
	"app/internal/version"
	ocdscripts "deploy-scripts"
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

	// DEBUG: Log the start of EKS handler
	fmt.Printf("[DEBUG] EKS Handler: Starting embedded script execution\n")

	// Use embedded list-eks-clusters.sh script following the same pattern as command_executor.go
	tempScriptFile, err := os.CreateTemp("", "list-eks-clusters_*.sh")
	if err != nil {
		fmt.Printf("[DEBUG] EKS Handler: Failed to create temp file: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  false,
			"message":  "DEBUG: Failed to create temp script: " + err.Error(),
			"clusters": []string{},
		})
		return
	}
	defer tempScriptFile.Close()
	fmt.Printf("[DEBUG] EKS Handler: Created temp script: %s\n", tempScriptFile.Name())

	scriptBytes, err := ocdscripts.ReadScript("list-eks-clusters.sh")
	if err != nil {
		fmt.Printf("[DEBUG] EKS Handler: Failed to read embedded script: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  false,
			"message":  "DEBUG: Failed to read embedded script: " + err.Error(),
			"clusters": []string{},
		})
		return
	}
	fmt.Printf("[DEBUG] EKS Handler: Read embedded script, size: %d bytes\n", len(scriptBytes))

	// Convert Windows line endings to Unix line endings for bash compatibility
	scriptContent := strings.ReplaceAll(string(scriptBytes), "\r\n", "\n")
	scriptContent = strings.ReplaceAll(scriptContent, "\r", "\n")
	previewLen := 100
	if len(scriptContent) < previewLen {
		previewLen = len(scriptContent)
	}
	fmt.Printf("[DEBUG] EKS Handler: Script content preview: %s...\n", scriptContent[:previewLen])

	if _, err := tempScriptFile.Write([]byte(scriptContent)); err != nil {
		fmt.Printf("[DEBUG] EKS Handler: Failed to write script: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  false,
			"message":  "DEBUG: Failed to write temp script: " + err.Error(),
			"clusters": []string{},
		})
		return
	}
	if err := os.Chmod(tempScriptFile.Name(), 0755); err != nil {
		fmt.Printf("[DEBUG] EKS Handler: Failed to chmod script: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  false,
			"message":  "DEBUG: Failed to chmod temp script: " + err.Error(),
			"clusters": []string{},
		})
		return
	}
	fmt.Printf("[DEBUG] EKS Handler: Script written and made executable\n")

	// Build command following the same pattern as command_executor.go
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wsl"); err == nil {
			wslScriptPath := convertToWSLPath(tempScriptFile.Name())
			cmdString := fmt.Sprintf("proxy on 2>/dev/null || true && bash %s", wslScriptPath)
			fmt.Printf("[DEBUG] EKS Handler: Executing WSL command: wsl bash -l -c \"%s\"\n", cmdString)
			cmd = exec.Command("wsl", "bash", "-l", "-c", cmdString)
		} else {
			fmt.Printf("[DEBUG] EKS Handler: WSL not available on Windows\n")
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":  false,
				"message":  "DEBUG: WSL not available on Windows. Please install WSL to use EKS features",
				"clusters": []string{},
			})
			return
		}
	case "linux", "darwin":
		cmdString := fmt.Sprintf("proxy on 2>/dev/null || true && bash %s", tempScriptFile.Name())
		fmt.Printf("[DEBUG] EKS Handler: Executing command: bash -l -c \"%s\"\n", cmdString)
		cmd = exec.Command("bash", "-l", "-c", cmdString)
	default:
		fmt.Printf("[DEBUG] EKS Handler: Unsupported OS: %s\n", runtime.GOOS)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  false,
			"message":  fmt.Sprintf("DEBUG: Unsupported operating system: %s", runtime.GOOS),
			"clusters": []string{},
		})
		return
	}

	// Capture both stdout and stderr for debugging
	output, err := cmd.CombinedOutput()
	fmt.Printf("[DEBUG] EKS Handler: Command output length: %d bytes\n", len(output))
	fmt.Printf("[DEBUG] EKS Handler: Command error: %v\n", err)
	if err != nil {
		fmt.Printf("[DEBUG] EKS Handler: Full command output: %s\n", string(output))
	}
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  false,
			"message":  "DEBUG: Failed to list EKS clusters: " + err.Error() + " | Output: " + string(output),
			"clusters": []string{},
		})
		return
	}
	fmt.Printf("[DEBUG] EKS Handler: Command executed successfully, output: %s\n", string(output))

	// Parse the output to extract cluster names
	// AWS CLI with --output text returns space/tab-separated values on a single line
	// Filter out screen size warnings and other noise
	outputStr := strings.TrimSpace(string(output))
	var clusters []string
	if outputStr != "" {
		// Split by lines first to filter out screen size warnings
		lines := strings.Split(outputStr, "\n")
		var cleanOutput []string

		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Skip screen size warnings and other noise
			if line != "" &&
				!strings.Contains(line, "screen size is bogus") &&
				!strings.Contains(line, "expect trouble") &&
				!strings.HasPrefix(line, "your ") {
				cleanOutput = append(cleanOutput, line)
			}
		}

		// Now split by whitespace to handle space or tab-separated cluster names
		for _, cleanLine := range cleanOutput {
			clusterNames := strings.Fields(cleanLine)
			for _, name := range clusterNames {
				name = strings.TrimSpace(name)
				// Additional filtering: only accept valid cluster name patterns
				if name != "" &&
					!strings.HasPrefix(name, "NAME") &&
					!strings.Contains(name, "x1") && // Filter out screen size fragments
					len(name) > 2 { // Cluster names should be reasonable length
					clusters = append(clusters, name)
				}
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

// convertToWSLPath converts Windows paths to WSL paths - same function as in command_executor.go
func convertToWSLPath(windowsPath string) string {
	if runtime.GOOS != "windows" {
		return windowsPath
	}
	wslPath := strings.ReplaceAll(windowsPath, "\\", "/")
	if strings.HasPrefix(wslPath, "C:") {
		wslPath = "/mnt/c" + wslPath[2:]
	} else if len(wslPath) >= 2 && wslPath[1] == ':' {
		drive := strings.ToLower(string(wslPath[0]))
		wslPath = "/mnt/" + drive + wslPath[2:]
	}
	return wslPath
}
