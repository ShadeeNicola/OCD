package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"app/internal/jenkins"
	"app/internal/jenkins/services"
	"app/internal/jenkins/types"
)

// JenkinsHandlers contains all Jenkins-related HTTP handlers
type JenkinsHandlers struct {
	client         *jenkins.Client
	scalingService services.ScalingService
	artifactsService services.ArtifactsService
}

// NewJenkinsHandlers creates a new Jenkins handlers instance
func NewJenkinsHandlers(client *jenkins.Client) *JenkinsHandlers {
	return &JenkinsHandlers{
		client:           client,
		scalingService:   services.NewScalingService(client),
		artifactsService: services.NewArtifactsService(client),
	}
}

// HandleJenkinsScale handles EKS cluster scaling requests
func (h *JenkinsHandlers) HandleJenkinsScale() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			types.ScaleRequest
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Check if we have credentials either from environment or request
		hasEnvCredentials := h.client.IsConfigured()
		hasRequestCredentials := req.Username != "" && req.Token != ""
		
		if !hasEnvCredentials && !hasRequestCredentials {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Jenkins credentials not configured. Please set OCD_JENKINS_USERNAME and OCD_JENKINS_TOKEN environment variables.",
			})
			return
		}

		// Create a context for the request
		ctx := r.Context()

		// Use appropriate client for the request
		var scalingService services.ScalingService
		if hasRequestCredentials {
			// Create temporary client with request credentials
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: req.Username,
				Token:    req.Token,
			})
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "Failed to create Jenkins client: " + err.Error(),
				})
				return
			}
			scalingService = services.NewScalingService(tempClient)
		} else {
			scalingService = h.scalingService
		}

		// Trigger the scaling operation
		response, err := scalingService.TriggerScale(ctx, &req.ScaleRequest)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to trigger Jenkins job: " + err.Error(),
			})
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"message":    response.Message,
			"job_status": response.JobStatus,
			"request_id": response.RequestID,
		})
	}
}

// HandleJenkinsStatus handles Jenkins job status queries
func (h *JenkinsHandlers) HandleJenkinsStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jobNumberStr := r.URL.Query().Get("job_number")
		if jobNumberStr == "" {
			http.Error(w, "job_number parameter is required", http.StatusBadRequest)
			return
		}

		jobNumber, err := strconv.Atoi(jobNumberStr)
		if err != nil {
			http.Error(w, "Invalid job_number format", http.StatusBadRequest)
			return
		}

		// Check for credentials in query parameters
		username := r.URL.Query().Get("username")
		token := r.URL.Query().Get("token")

		// Create context for the request
		ctx := r.Context()

		// Use appropriate client for the request
		var scalingService services.ScalingService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				http.Error(w, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
				return
			}
			scalingService = services.NewScalingService(tempClient)
		} else {
			scalingService = h.scalingService
		}

		// Get job status
		jobStatus, err := scalingService.GetScaleJobStatus(ctx, jobNumber)
		if err != nil {
			http.Error(w, "Failed to get job status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jobStatus)
	}
}

// HandleJenkinsQueueStatus handles Jenkins queue status queries
func (h *JenkinsHandlers) HandleJenkinsQueueStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		queueURL := r.URL.Query().Get("queue_url")
		if queueURL == "" {
			http.Error(w, "queue_url parameter is required", http.StatusBadRequest)
			return
		}

		// Check for credentials in query parameters
		username := r.URL.Query().Get("username")
		token := r.URL.Query().Get("token")

		// Create context for the request
		ctx := r.Context()

		// Use appropriate client for the request
		var scalingService services.ScalingService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				http.Error(w, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
				return
			}
			scalingService = services.NewScalingService(tempClient)
		} else {
			scalingService = h.scalingService
		}

		// Get queue status
		queueStatus, err := scalingService.GetQueueStatus(ctx, queueURL)
		if err != nil {
			http.Error(w, "Failed to get queue status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(queueStatus)
	}
}

// HandleJenkinsArtifacts handles artifact extraction requests
func (h *JenkinsHandlers) HandleJenkinsArtifacts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			types.ArtifactExtractionRequest
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Check for credentials
		hasEnvCredentials := h.client.IsConfigured()
		hasRequestCredentials := req.Username != "" && req.Token != ""
		
		if !hasEnvCredentials && !hasRequestCredentials {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Jenkins credentials not configured",
			})
			return
		}

		// Create context for the request
		ctx := r.Context()

		// Use appropriate client for the request
		var artifactsService services.ArtifactsService
		if hasRequestCredentials {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: req.Username,
				Token:    req.Token,
			})
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "Failed to create Jenkins client: " + err.Error(),
				})
				return
			}
			artifactsService = services.NewArtifactsService(tempClient)
		} else {
			artifactsService = h.artifactsService
		}

		// Extract artifacts
		response, err := artifactsService.ExtractArtifacts(ctx, &req.ArtifactExtractionRequest)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to extract artifacts: " + err.Error(),
			})
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"message":  "Artifacts extracted successfully",
			"response": response,
		})
	}
}

// HandleJenkinsBuildInfo handles build information requests
func (h *JenkinsHandlers) HandleJenkinsBuildInfo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		buildURL := r.URL.Query().Get("build_url")
		if buildURL == "" {
			http.Error(w, "build_url parameter is required", http.StatusBadRequest)
			return
		}

		// Check for credentials in query parameters
		username := r.URL.Query().Get("username")
		token := r.URL.Query().Get("token")

		// Create context for the request
		ctx := r.Context()

		// Use appropriate client for the request
		var artifactsService services.ArtifactsService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				http.Error(w, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
				return
			}
			artifactsService = services.NewArtifactsService(tempClient)
		} else {
			artifactsService = h.artifactsService
		}

		// Get build info
		buildInfo, err := artifactsService.GetBuildInfo(ctx, buildURL)
		if err != nil {
			http.Error(w, "Failed to get build info: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(buildInfo)
	}
}

// RegisterJenkinsRoutes registers all Jenkins-related routes with a mux
func (h *JenkinsHandlers) RegisterJenkinsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/jenkins/scale", h.HandleJenkinsScale())
	mux.HandleFunc("/jenkins/status", h.HandleJenkinsStatus())
	mux.HandleFunc("/jenkins/queue-status", h.HandleJenkinsQueueStatus())
	mux.HandleFunc("/jenkins/artifacts", h.HandleJenkinsArtifacts())
	mux.HandleFunc("/jenkins/build-info", h.HandleJenkinsBuildInfo())
}