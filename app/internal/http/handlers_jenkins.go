package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"app/internal/jenkins"
	"app/internal/jenkins/services"
	"app/internal/jenkins/types"
)

// JenkinsHandlers contains all Jenkins-related HTTP handlers
type JenkinsHandlers struct {
	client           *jenkins.Client
	scalingService   services.ScalingService
	artifactsService services.ArtifactsService
	rnCreationService services.RNCreationService
}

// NewJenkinsHandlers creates a new Jenkins handlers instance
func NewJenkinsHandlers(client *jenkins.Client) *JenkinsHandlers {
	return &JenkinsHandlers{
		client:            client,
		scalingService:    services.NewScalingService(client),
		artifactsService:  services.NewArtifactsService(client),
		rnCreationService: services.NewRNCreationService(client),
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
		var jobNumber int
		var username, token string

		if r.Method == "POST" {
			var req struct {
				JobNumber int    `json:"job_number"`
				Username  string `json:"username"`
				Token     string `json:"token"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request format", http.StatusBadRequest)
				return
			}
			jobNumber = req.JobNumber
			username = req.Username
			token = req.Token
		} else if r.Method == "GET" {
			jobNumberStr := r.URL.Query().Get("job_number")
			if jobNumberStr == "" {
				http.Error(w, "job_number parameter is required", http.StatusBadRequest)
				return
			}

			var err error
			jobNumber, err = strconv.Atoi(jobNumberStr)
			if err != nil {
				http.Error(w, "Invalid job_number format", http.StatusBadRequest)
				return
			}

			// Check for credentials in query parameters
			username = r.URL.Query().Get("username")
			token = r.URL.Query().Get("token")
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

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
		var queueURL, username, token string

		if r.Method == "POST" {
			var req struct {
				QueueURL string `json:"queue_url"`
				Username string `json:"username"`
				Token    string `json:"token"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Invalid request format", http.StatusBadRequest)
				return
			}
			queueURL = req.QueueURL
			username = req.Username
			token = req.Token
		} else if r.Method == "GET" {
			queueURL = r.URL.Query().Get("queue_url")
			if queueURL == "" {
				http.Error(w, "queue_url parameter is required", http.StatusBadRequest)
				return
			}

			// Check for credentials in query parameters
			username = r.URL.Query().Get("username")
			token = r.URL.Query().Get("token")
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

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

// HandleRNCreate handles RN creation requests
func (h *JenkinsHandlers) HandleRNCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			types.RNCreationRequest
			Username string `json:"username"`
			Token    string `json:"token"`
			BitbucketUsername string `json:"bitbucket_username"`
			BitbucketToken    string `json:"bitbucket_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Check if we have credentials
		hasEnvCredentials := h.client.IsConfigured()
		hasRequestCredentials := req.Username != "" && req.Token != ""
		
		if !hasEnvCredentials && !hasRequestCredentials {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Jenkins credentials not configured. Please set credentials in Settings.",
			})
			return
		}

		// Create context for the request
		ctx := r.Context()

		// Use appropriate client for the request
		var rnCreationService services.RNCreationService
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
			rnCreationService = services.NewRNCreationService(tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		// Auto-populate request from customization job
		if err := rnCreationService.PopulateRequestFromCustomizationJob(ctx, &req.RNCreationRequest); err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to populate request from customization job: " + err.Error(),
			})
			return
		}

		// Get oni_image from Bitbucket if credentials provided
		if req.BitbucketUsername != "" && req.BitbucketToken != "" && req.OniImage == "" {
			oniImage, err := rnCreationService.GetOniImageFromBitbucket(ctx, req.Branch, "customization", req.BitbucketUsername, req.BitbucketToken)
			if err == nil {
				req.OniImage = oniImage
			}
		}

		// Generate email if not provided
		if req.Email == "" && req.BitbucketUsername != "" {
			req.Email = req.BitbucketUsername + "@amdocs.com"
		}

		// Trigger the RN creation operation using credentials
		var response *types.RNCreationResponse
		var err error
		
		if hasRequestCredentials {
			// Use explicit credentials for the different Jenkins server
			response, err = rnCreationService.TriggerStorageCreationWithCredentials(ctx, &req.RNCreationRequest, req.Username, req.Token)
		} else {
			// Fall back to regular method (may not work for different server)
			response, err = rnCreationService.TriggerStorageCreation(ctx, &req.RNCreationRequest)
		}
		
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to trigger storage creation job: " + err.Error(),
			})
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"message":    response.Message,
			"job_url":    response.JobURL,
			"job_status": response.JobStatus,
			"request_id": response.RequestID,
		})
	}
}

// HandleRNCustomizationJob handles requests to get customization job info
func (h *JenkinsHandlers) HandleRNCustomizationJob() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		branch := r.URL.Query().Get("branch")
		if branch == "" {
			http.Error(w, "branch parameter is required", http.StatusBadRequest)
			return
		}

		// Check for Jenkins credentials in query parameters
		username := r.URL.Query().Get("username")
		token := r.URL.Query().Get("token")

		// Create context for the request
		ctx := r.Context()

		// Use appropriate client for the request
		var rnCreationService services.RNCreationService
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
			rnCreationService = services.NewRNCreationService(tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		// Get latest customization job
		job, err := rnCreationService.GetLatestCustomizationJob(ctx, branch)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to get customization job: " + err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"job":     job,
		})
	}
}

// HandleRNBuildParameters handles requests to get build parameters
func (h *JenkinsHandlers) HandleRNBuildParameters() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jobURL := r.URL.Query().Get("job_url")
		if jobURL == "" {
			http.Error(w, "job_url parameter is required", http.StatusBadRequest)
			return
		}

		username := r.URL.Query().Get("username")
		token := r.URL.Query().Get("token")

		ctx := r.Context()
		var rnCreationService services.RNCreationService
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
			rnCreationService = services.NewRNCreationService(tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		parameters, err := rnCreationService.GetBuildParameters(ctx, jobURL)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to get build parameters: " + err.Error(),
			})
			return
		}

		// Also get build description for TLC version extraction
		description, _ := rnCreationService.GetBuildDescription(ctx, jobURL)
		
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"parameters":  parameters,
			"description": description,
		})
	}
}

// HandleRNArtifactURL handles requests to get artifact URL
func (h *JenkinsHandlers) HandleRNArtifactURL() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jobURL := r.URL.Query().Get("job_url")
		if jobURL == "" {
			http.Error(w, "job_url parameter is required", http.StatusBadRequest)
			return
		}

		username := r.URL.Query().Get("username")
		token := r.URL.Query().Get("token")

		ctx := r.Context()

		// Use existing artifacts service to parse "Deployed Artifacts" section
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

		// Extract artifacts using existing service
		request := &types.ArtifactExtractionRequest{
			BuildURL: jobURL,
			FilterTypes: []string{"zip"}, // Filter for zip files
		}

		response, err := artifactsService.ExtractArtifacts(ctx, request)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to extract artifacts: " + err.Error(),
			})
			return
		}

		// Look for att-orchestration*src.zip artifact
		var orchestrationURL string
		for _, artifact := range response.Artifacts {
			if strings.Contains(artifact.Name, "att-orchestration") && strings.Contains(artifact.Name, "src.zip") {
				// Apply host swapping as specified in requirements
				originalURL := artifact.URL
				orchestrationURL = strings.Replace(originalURL, 
					"https://oss-nexus2.oss.corp.amdocs.aws/repository/att.maven.snapshot/", 
					"http://illin3613:8081/repository/maven.group/", 1)
				break
			}
		}

		if orchestrationURL == "" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "att-orchestration*src.zip artifact not found",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"artifact_url": orchestrationURL,
		})
	}
}

// HandleRNOniImage handles requests to get ONI image
func (h *JenkinsHandlers) HandleRNOniImage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		branch := r.URL.Query().Get("branch")
		if branch == "" {
			http.Error(w, "branch parameter is required", http.StatusBadRequest)
			return
		}

		username := r.URL.Query().Get("username")
		token := r.URL.Query().Get("token")

		if username == "" || token == "" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Bitbucket credentials required",
			})
			return
		}

		ctx := r.Context()
		rnCreationService := h.rnCreationService

		oniImage, err := rnCreationService.GetOniImageFromBitbucket(ctx, branch, "customization", username, token)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to get ONI image: " + err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"oni_image": oniImage,
		})
	}
}

// HandleRNTableData handles RN table data generation requests
func (h *JenkinsHandlers) HandleRNTableData() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get parameters from query string
		customizationJobURL := r.URL.Query().Get("customization_job_url")
		customOrchZipURL := r.URL.Query().Get("custom_orch_zip_url")
		oniImage := r.URL.Query().Get("oni_image")

		if customizationJobURL == "" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "customization_job_url parameter is required",
			})
			return
		}

		// Get credentials from query parameters (same pattern as HandleRNBuildParameters)
		username := r.URL.Query().Get("username")
		token := r.URL.Query().Get("token")

		ctx := r.Context()
		var rnCreationService services.RNCreationService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "Failed to create Jenkins client: " + err.Error(),
				})
				return
			}
			rnCreationService = services.NewRNCreationService(tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		// Create request structure
		request := &types.RNTableRequest{
			CustomizationJobURL: customizationJobURL,
			CustomOrchZipURL:    customOrchZipURL,
			OniImage:           oniImage,
		}

		// Generate RN table data
		rnTableData, err := rnCreationService.GenerateRNTableData(ctx, request)
		if err != nil {
			log.Printf("ERROR: GenerateRNTableData failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to generate RN table data: " + err.Error(),
			})
			return
		}

		log.Printf("DEBUG: GenerateRNTableData succeeded, returning data: %+v", rnTableData)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":        true,
			"rn_table_data":  rnTableData,
		})
	}
}

// RegisterJenkinsRoutes registers all Jenkins-related routes with a mux
func (h *JenkinsHandlers) RegisterJenkinsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/jenkins/scale", h.HandleJenkinsScale())
	mux.HandleFunc("/api/jenkins/status", h.HandleJenkinsStatus())
	mux.HandleFunc("/api/jenkins/queue-status", h.HandleJenkinsQueueStatus())
	mux.HandleFunc("/api/jenkins/artifacts", h.HandleJenkinsArtifacts())
	mux.HandleFunc("/api/jenkins/build-info", h.HandleJenkinsBuildInfo())
	mux.HandleFunc("/api/jenkins/rn-create", h.HandleRNCreate())
	mux.HandleFunc("/api/jenkins/rn-customization-job", h.HandleRNCustomizationJob())
	mux.HandleFunc("/api/jenkins/rn-build-parameters", h.HandleRNBuildParameters())
	mux.HandleFunc("/api/jenkins/rn-artifact-url", h.HandleRNArtifactURL())
	mux.HandleFunc("/api/jenkins/rn-oni-image", h.HandleRNOniImage())
	mux.HandleFunc("/api/jenkins/rn-table-data", h.HandleRNTableData())
}