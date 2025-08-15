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
    "app/internal/jenkins"
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

func HandleJenkinsScale(client *jenkins.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req struct {
            jenkins.ScaleRequest
            Username string `json:"username"`
            Token    string `json:"token"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request format", http.StatusBadRequest)
            return
        }

        // Check if we have credentials either from environment or request
        hasEnvCredentials := client.IsConfigured()
        hasRequestCredentials := req.Username != "" && req.Token != ""
        
        if !hasEnvCredentials && !hasRequestCredentials {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{
                "success": false,
                "message": "Jenkins credentials not configured. Please set OCD_JENKINS_USERNAME and OCD_JENKINS_TOKEN environment variables.",
            })
            return
        }

        if req.ClusterName == "" || req.ScaleType == "" {
            http.Error(w, "cluster_name and scale_type are required", http.StatusBadRequest)
            return
        }

        if req.ScaleType != "up" && req.ScaleType != "down" {
            http.Error(w, "scale_type must be 'up' or 'down'", http.StatusBadRequest)
            return
        }

        if req.Account == "" {
            req.Account = "ATT" // Default account
        }

        // Use credentials from request if provided, otherwise fall back to environment
        var jobStatus *jenkins.JobStatus
        var err error
        
        if hasRequestCredentials {
            tempClient := jenkins.NewClientWithConfig(jenkins.ClientConfig{
                URL:      client.GetURL(),
                Username: req.Username,
                Token:    req.Token,
            })
            jobStatus, err = tempClient.TriggerScaleJob(req.ScaleRequest)
        } else {
            jobStatus, err = client.TriggerScaleJob(req.ScaleRequest)
        }
        
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{
                "success": false,
                "message": "Failed to trigger Jenkins job: " + err.Error(),
            })
            return
        }

        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "success":    true,
            "message":    "Jenkins job triggered successfully",
            "job_status": jobStatus,
        })
    }
}

func HandleJenkinsStatus(client *jenkins.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req struct {
            JobNumber int    `json:"job_number"`
            Username  string `json:"username"`
            Token     string `json:"token"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request format", http.StatusBadRequest)
            return
        }

        if req.JobNumber <= 0 {
            http.Error(w, "job_number is required and must be positive", http.StatusBadRequest)
            return
        }

        // Use credentials from request if provided, otherwise fall back to environment
        var tempClient *jenkins.Client
        if req.Username != "" && req.Token != "" {
            tempClient = jenkins.NewClientWithConfig(jenkins.ClientConfig{
                URL:      client.GetURL(),
                Username: req.Username,
                Token:    req.Token,
            })
        } else {
            tempClient = client
        }

        jobStatus, err := tempClient.GetJobStatus(req.JobNumber)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{
                "success": false,
                "message": "Failed to get job status: " + err.Error(),
            })
            return
        }

        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "success":    true,
            "job_status": jobStatus,
        })
    }
}

func HandleJenkinsQueueStatus(client *jenkins.Client) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        var req struct {
            QueueURL string `json:"queue_url"`
            Username string `json:"username"`
            Token    string `json:"token"`
        }

        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            http.Error(w, "Invalid request format", http.StatusBadRequest)
            return
        }

        if req.QueueURL == "" {
            http.Error(w, "queue_url is required", http.StatusBadRequest)
            return
        }

        // Use credentials from request if provided, otherwise fall back to environment
        var tempClient *jenkins.Client
        if req.Username != "" && req.Token != "" {
            tempClient = jenkins.NewClientWithConfig(jenkins.ClientConfig{
                URL:      client.GetURL(),
                Username: req.Username,
                Token:    req.Token,
            })
        } else {
            tempClient = client
        }

        jobStatus, err := tempClient.GetQueueItemStatus(req.QueueURL)
        if err != nil {
            w.Header().Set("Content-Type", "application/json")
            _ = json.NewEncoder(w).Encode(map[string]interface{}{
                "success": false,
                "message": "Failed to get queue status: " + err.Error(),
            })
            return
        }

        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]interface{}{
            "success":    true,
            "job_status": jobStatus,
        })
    }
}

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
            "message": "Failed to fetch EKS clusters. Make sure AWS CLI is configured and proxy is set up: " + err.Error(),
            "clusters": []string{},
        })
        return
    }

    // Parse the output - AWS CLI returns cluster names separated by tabs/spaces
    clusterOutput := strings.TrimSpace(string(output))
    var clusters []string
    
    if clusterOutput != "" {
        // Split by whitespace and filter out empty strings
        rawClusters := strings.Fields(clusterOutput)
        for _, cluster := range rawClusters {
            cluster = strings.TrimSpace(cluster)
            if cluster != "" {
                clusters = append(clusters, cluster)
            }
        }
    }

    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]interface{}{
        "success":  true,
        "message":  "EKS clusters fetched successfully",
        "clusters": clusters,
    })
}


