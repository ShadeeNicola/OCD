package httpapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"app/internal/config"
	"app/internal/jenkins"
	"app/internal/jenkins/services"
	"app/internal/jenkins/types"
)

// JenkinsHandlers contains all Jenkins-related HTTP handlers
type JenkinsHandlers struct {
	configuration     *config.Config
	client            *jenkins.Client
	scalingService    services.ScalingService
	artifactsService  services.ArtifactsService
	rnCreationService services.RNCreationService
	automationService services.AutomationService
}

// NewJenkinsHandlers creates a new Jenkins handlers instance
func NewJenkinsHandlers(configuration *config.Config, client *jenkins.Client) *JenkinsHandlers {
	return &JenkinsHandlers{
		configuration:     configuration,
		client:            client,
		scalingService:    services.NewScalingService(configuration, client),
		artifactsService:  services.NewArtifactsService(client),
		rnCreationService: services.NewRNCreationService(configuration, client),
		automationService: services.NewAutomationService(configuration, client),
	}
}

// HandleJenkinsScale handles EKS cluster scaling requests
func (h *JenkinsHandlers) HandleJenkinsScale() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != "POST" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			types.ScaleRequest
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			http.Error(response, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Check if we have credentials either from environment or request
		hasEnvCredentials := h.client.IsConfigured()
		hasRequestCredentials := req.Username != "" && req.Token != ""

		if !hasEnvCredentials && !hasRequestCredentials {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Jenkins credentials not configured. Please set OCD_JENKINS_USERNAME and OCD_JENKINS_TOKEN environment variables.",
			})
			return
		}

		// Create a context for the request
		ctx := request.Context()

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
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]interface{}{
					"success": false,
					"message": "Failed to create Jenkins client: " + err.Error(),
				})
				return
			}
			scalingService = services.NewScalingService(h.configuration, tempClient)
		} else {
			scalingService = h.scalingService
		}

		// Trigger the scaling operation
		scaleResponse, err := scalingService.TriggerScale(ctx, &req.ScaleRequest)
		if err != nil {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to trigger Jenkins job: " + err.Error(),
			})
			return
		}

		// Return success response
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"success":    true,
			"message":    scaleResponse.Message,
			"job_status": scaleResponse.JobStatus,
			"request_id": scaleResponse.RequestID,
		})
	}
}

// HandleJenkinsStatus handles Jenkins job status queries
func (h *JenkinsHandlers) HandleJenkinsStatus() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSONError(response, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			JobNumber int    `json:"job_number"`
			Username  string `json:"username"`
			Token     string `json:"token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeJSONError(response, http.StatusBadRequest, "Invalid request payload")
			return
		}

		jobNumber := req.JobNumber
		username := req.Username
		token := req.Token

		log.Printf("DEBUG: HandleJenkinsStatus POST - jobNumber: %d, username: %s", jobNumber, username)

		if jobNumber <= 0 {
			log.Printf("ERROR: Invalid job number: %d", jobNumber)
			writeJSONError(response, http.StatusBadRequest, "Invalid job number: must be greater than 0")
			return
		}

		// Create context for the request
		ctx := request.Context()

		// Use appropriate client for the request
		var scalingService services.ScalingService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "Failed to create Jenkins client: "+err.Error())
				return
			}
			scalingService = services.NewScalingService(h.configuration, tempClient)
		} else {
			scalingService = h.scalingService
		}

		// Get job status
		jobStatus, err := scalingService.GetScaleJobStatus(ctx, jobNumber)
		if err != nil {
			writeJSONError(response, http.StatusInternalServerError, "Failed to get job status: "+err.Error())
			return
		}

		writeJSON(response, http.StatusOK, jobStatus)
	}
}

// HandleJenkinsQueueStatus handles Jenkins queue status queries
func (h *JenkinsHandlers) HandleJenkinsQueueStatus() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSONError(response, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			QueueURL string `json:"queue_url"`
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeJSONError(response, http.StatusBadRequest, "Invalid request payload")
			return
		}

		queueURL := strings.TrimSpace(req.QueueURL)
		username := req.Username
		token := req.Token

		if queueURL == "" {
			writeJSONError(response, http.StatusBadRequest, "queue_url is required")
			return
		}

		// Create context for the request
		ctx := request.Context()

		// Use appropriate client for the request
		var scalingService services.ScalingService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "Failed to create Jenkins client: "+err.Error())
				return
			}
			scalingService = services.NewScalingService(h.configuration, tempClient)
		} else {
			scalingService = h.scalingService
		}

		// Get queue status
		queueStatus, err := scalingService.GetQueueStatus(ctx, queueURL)
		if err != nil {
			writeJSONError(response, http.StatusInternalServerError, "Failed to get queue status: "+err.Error())
			return
		}

		writeJSON(response, http.StatusOK, queueStatus)
	}
}

// HandleJenkinsArtifacts handles artifact extraction requests
func (h *JenkinsHandlers) HandleJenkinsArtifacts() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != "POST" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			types.ArtifactExtractionRequest
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			http.Error(response, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Check for credentials
		hasEnvCredentials := h.client.IsConfigured()
		hasRequestCredentials := req.Username != "" && req.Token != ""

		if !hasEnvCredentials && !hasRequestCredentials {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Jenkins credentials not configured",
			})
			return
		}

		// Create context for the request
		ctx := request.Context()

		// Use appropriate client for the request
		var artifactsService services.ArtifactsService
		if hasRequestCredentials {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: req.Username,
				Token:    req.Token,
			})
			if err != nil {
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]interface{}{
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
		artifactsResponse, err := artifactsService.ExtractArtifacts(ctx, &req.ArtifactExtractionRequest)
		if err != nil {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to extract artifacts: " + err.Error(),
			})
			return
		}

		// Return success response
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"success":  true,
			"message":  "Artifacts extracted successfully",
			"response": artifactsResponse,
		})
	}
}

// HandleJenkinsBuildInfo handles build information requests
func (h *JenkinsHandlers) HandleJenkinsBuildInfo() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSONError(response, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			BuildURL         string `json:"build_url"`
			Username         string `json:"username"`
			Token            string `json:"token"`
			Product          string `json:"product"`
			CoreVersion      string `json:"core_version"`
			BranchName       string `json:"branch_name"`
			CustomOrchZipURL string `json:"custom_orch_zip_url"`
			OniImage         string `json:"oni_image"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeJSONError(response, http.StatusBadRequest, "Invalid request payload")
			return
		}

		buildURL := strings.TrimSpace(req.BuildURL)
		if buildURL == "" {
			writeJSONError(response, http.StatusBadRequest, "build_url is required")
			return
		}

		username := strings.TrimSpace(req.Username)
		token := strings.TrimSpace(req.Token)

		ctx := request.Context()

		var jenkinsClient *jenkins.Client
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "Failed to create Jenkins client: "+err.Error())
				return
			}
			jenkinsClient = tempClient
		} else {
			jenkinsClient = h.client
		}

		if !regexp.MustCompile(`/\d+/?$`).MatchString(buildURL) {
			product := strings.TrimSpace(req.Product)
			coreVersion := strings.TrimSpace(req.CoreVersion)
			branchName := strings.TrimSpace(req.BranchName)
			customOrchZipURL := strings.TrimSpace(req.CustomOrchZipURL)
			oniImage := strings.TrimSpace(req.OniImage)

			log.Printf("DEBUG: Build info lookup for %s (branch: %s)", buildURL, branchName)

			jobAPIURL := strings.TrimSuffix(buildURL, "/") + "/api/json"

			responseBody, err := jenkinsClient.GetWithAuth(ctx, jobAPIURL)
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "Failed to get job info: "+err.Error())
				return
			}

			var jobInfo struct {
				LastBuild struct {
					Number int    `json:"number"`
					URL    string `json:"url"`
				} `json:"lastBuild"`
				LastCompletedBuild struct {
					Number int    `json:"number"`
					URL    string `json:"url"`
				} `json:"lastCompletedBuild"`
				InQueue bool `json:"inQueue"`
			}

			if err := json.Unmarshal(responseBody, &jobInfo); err != nil {
				writeJSONError(response, http.StatusInternalServerError, "Failed to parse job info: "+err.Error())
				return
			}

			targetBuildNumber := jobInfo.LastBuild.Number

			if product != "" || coreVersion != "" || branchName != "" || customOrchZipURL != "" || oniImage != "" {
				buildAPIURL := strings.TrimSuffix(buildURL, "/") + fmt.Sprintf("/%d/api/json", targetBuildNumber)
				buildResponseBody, err := jenkinsClient.GetWithAuth(ctx, buildAPIURL)
				if err != nil {
					log.Printf("DEBUG: Latest build %d does not exist, using fallback", targetBuildNumber)
				} else {
					var buildInfo struct {
						Actions []struct {
							Class      string `json:"_class"`
							Parameters []struct {
								Name  string `json:"name"`
								Value string `json:"value"`
							} `json:"parameters,omitempty"`
						} `json:"actions"`
					}

					if err := json.Unmarshal(buildResponseBody, &buildInfo); err == nil {
						var buildParams map[string]string
						for _, action := range buildInfo.Actions {
							if action.Class == "hudson.model.ParametersAction" && action.Parameters != nil {
								buildParams = make(map[string]string)
								for _, param := range action.Parameters {
									buildParams[param.Name] = param.Value
								}
								break
							}
						}

						parametersMatch := true
						if branchName != "" && buildParams["branch_name"] != branchName {
							parametersMatch = false
						}
						if oniImage != "" && buildParams["oni_image"] != oniImage {
							parametersMatch = false
						}

						if !parametersMatch {
							writeJSONError(response, http.StatusNotFound, "Latest build parameters don't match")
							return
						}
					}
				}
			}

			result := map[string]interface{}{
				"success": true,
				"build_info": map[string]interface{}{
					"lastBuild": map[string]interface{}{
						"number": targetBuildNumber,
						"url":    strings.TrimSuffix(buildURL, "/") + fmt.Sprintf("/%d", targetBuildNumber),
					},
				},
			}

			writeJSON(response, http.StatusOK, result)
			return
		}

		artifactsService := services.NewArtifactsService(jenkinsClient)
		buildInfo, err := artifactsService.GetBuildInfo(ctx, buildURL)
		if err != nil {
			writeJSONError(response, http.StatusInternalServerError, "Failed to get build info: "+err.Error())
			return
		}

		writeJSON(response, http.StatusOK, buildInfo)
	}
}

// HandleRNCreate handles RN creation requests
func (h *JenkinsHandlers) HandleRNCreate() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != "POST" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			types.RNCreationRequest
			Username          string `json:"username"`
			Token             string `json:"token"`
			BitbucketUsername string `json:"bitbucket_username"`
			BitbucketToken    string `json:"bitbucket_token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			http.Error(response, "Invalid request format", http.StatusBadRequest)
			return
		}

		// Check if we have credentials
		hasEnvCredentials := h.client.IsConfigured()
		hasRequestCredentials := req.Username != "" && req.Token != ""

		if !hasEnvCredentials && !hasRequestCredentials {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Jenkins credentials not configured. Please set credentials in Settings.",
			})
			return
		}

		// Create context for the request
		ctx := request.Context()

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
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]interface{}{
					"success": false,
					"message": "Failed to create Jenkins client: " + err.Error(),
				})
				return
			}
			rnCreationService = services.NewRNCreationService(h.configuration, tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		// Auto-populate request from customization job
		if err := rnCreationService.PopulateRequestFromCustomizationJob(ctx, &req.RNCreationRequest); err != nil {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
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
		var rnResponse *types.RNCreationResponse
		var err error

		if hasRequestCredentials {
			// Use explicit credentials for the different Jenkins server
			rnResponse, err = rnCreationService.TriggerStorageCreationWithCredentials(ctx, &req.RNCreationRequest, req.Username, req.Token)
		} else {
			// Fall back to regular method (may not work for different server)
			rnResponse, err = rnCreationService.TriggerStorageCreation(ctx, &req.RNCreationRequest)
		}

		if err != nil {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to trigger storage creation job: " + err.Error(),
			})
			return
		}

		// Return success response
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"success":    true,
			"message":    rnResponse.Message,
			"job_url":    rnResponse.JobURL,
			"job_status": rnResponse.JobStatus,
			"request_id": rnResponse.RequestID,
		})
	}
}

// HandleRNCustomizationJob handles requests to get customization job info
func (h *JenkinsHandlers) HandleRNCustomizationJob() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSONError(response, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			Branch   string `json:"branch"`
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeJSONError(response, http.StatusBadRequest, "Invalid request payload")
			return
		}

		branch := strings.TrimSpace(req.Branch)
		if branch == "" {
			writeJSONError(response, http.StatusBadRequest, "branch is required")
			return
		}

		username := strings.TrimSpace(req.Username)
		token := strings.TrimSpace(req.Token)

		// Create context for the request
		ctx := request.Context()

		// Use appropriate client for the request
		var rnCreationService services.RNCreationService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "Failed to create Jenkins client: "+err.Error())
				return
			}
			rnCreationService = services.NewRNCreationService(h.configuration, tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		// Get latest customization job
		job, err := rnCreationService.GetLatestCustomizationJob(ctx, branch)
		if err != nil {
			writeJSON(response, http.StatusOK, map[string]interface{}{
				"success": false,
				"message": "Failed to get customization job: " + err.Error(),
			})
			return
		}

		writeJSON(response, http.StatusOK, map[string]interface{}{
			"success": true,
			"job":     job,
		})
	}
}

// HandleRNBuildParameters handles requests to get build parameters
func (h *JenkinsHandlers) HandleRNBuildParameters() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSONError(response, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			JobURL   string `json:"job_url"`
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeJSONError(response, http.StatusBadRequest, "Invalid request payload")
			return
		}

		jobURL := strings.TrimSpace(req.JobURL)
		if jobURL == "" {
			writeJSONError(response, http.StatusBadRequest, "job_url is required")
			return
		}

		username := strings.TrimSpace(req.Username)
		token := strings.TrimSpace(req.Token)

		ctx := request.Context()
		var rnCreationService services.RNCreationService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "Failed to create Jenkins client: "+err.Error())
				return
			}
			rnCreationService = services.NewRNCreationService(h.configuration, tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		parameters, err := rnCreationService.GetBuildParameters(ctx, jobURL)
		if err != nil {
			writeJSON(response, http.StatusOK, map[string]interface{}{
				"success": false,
				"message": "Failed to get build parameters: " + err.Error(),
			})
			return
		}

		// Also get build description for TLC version extraction
		description, _ := rnCreationService.GetBuildDescription(ctx, jobURL)

		writeJSON(response, http.StatusOK, map[string]interface{}{
			"success":     true,
			"parameters":  parameters,
			"description": description,
		})
	}
}

// HandleRNArtifactURL handles requests to get artifact URL
func (h *JenkinsHandlers) HandleRNArtifactURL() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSONError(response, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			JobURL   string `json:"job_url"`
			Username string `json:"username"`
			Token    string `json:"token"`
			Branch   string `json:"branch"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeJSONError(response, http.StatusBadRequest, "Invalid request payload")
			return
		}

		jobURL := strings.TrimSpace(req.JobURL)
		if jobURL == "" {
			writeJSONError(response, http.StatusBadRequest, "job_url is required")
			return
		}

		username := strings.TrimSpace(req.Username)
		token := strings.TrimSpace(req.Token)
		branch := strings.TrimSpace(req.Branch)

		ctx := request.Context()

		// Use existing artifacts service to parse "Deployed Artifacts" section
		var artifactsService services.ArtifactsService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				writeJSONError(response, http.StatusInternalServerError, "Failed to create Jenkins client: "+err.Error())
				return
			}
			artifactsService = services.NewArtifactsService(tempClient)
		} else {
			artifactsService = h.artifactsService
		}

		// Extract artifacts using existing service
		extractRequest := &types.ArtifactExtractionRequest{
			BuildURL:    jobURL,
			FilterTypes: []string{"zip"}, // Filter for zip files
		}

		artifactsResponse, err := artifactsService.ExtractArtifacts(ctx, extractRequest)
		if err != nil {
			writeJSON(response, http.StatusOK, map[string]interface{}{
				"success": false,
				"message": "Failed to extract artifacts: " + err.Error(),
			})
			return
		}

		// Look for att-orchestration*src.zip artifact
		var orchestrationURL string
		for _, artifact := range artifactsResponse.Artifacts {
			if strings.Contains(artifact.Name, "att-orchestration") && strings.Contains(artifact.Name, "src.zip") {
				originalURL := artifact.URL
				if h.configuration != nil {
					orchestrationURL = h.configuration.Endpoints.ReplaceWithInternalNexus(originalURL)
				} else {
					orchestrationURL = originalURL
				}
				break
			}
		}

		if orchestrationURL == "" {
			log.Printf("Jenkins artifact not found, trying Nexus direct fetch fallback...")
			if branch != "" {
				log.Printf("Calling fetchOrchestrationFromNexus with branch: %s", branch)
				fallbackURL, err := h.fetchOrchestrationFromNexus(branch)
				if err == nil && fallbackURL != "" {
					orchestrationURL = fallbackURL
					log.Printf("Successfully fetched orchestration URL from Nexus: %s", orchestrationURL)
				} else {
					log.Printf("Nexus fallback also failed: %v", err)
				}
			} else {
				log.Printf("No branch parameter provided, skipping Nexus fallback")
			}

			if orchestrationURL == "" {
				writeJSON(response, http.StatusOK, map[string]interface{}{
					"success": false,
					"message": "att-orchestration*src.zip artifact not found in Jenkins artifacts or Nexus fallback",
				})
				return
			}
		}

		writeJSON(response, http.StatusOK, map[string]interface{}{
			"success":      true,
			"artifact_url": orchestrationURL,
		})
	}
}

// HandleRNOniImage handles requests to get ONI image
func (h *JenkinsHandlers) HandleRNOniImage() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSONError(response, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			Branch   string `json:"branch"`
			Username string `json:"username"`
			Token    string `json:"token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeJSONError(response, http.StatusBadRequest, "Invalid request payload")
			return
		}

		branch := strings.TrimSpace(req.Branch)
		if branch == "" {
			writeJSONError(response, http.StatusBadRequest, "branch is required")
			return
		}

		username := strings.TrimSpace(req.Username)
		token := strings.TrimSpace(req.Token)

		if username == "" || token == "" {
			writeJSON(response, http.StatusOK, map[string]interface{}{
				"success": false,
				"message": "Bitbucket credentials required",
			})
			return
		}

		ctx := request.Context()
		rnCreationService := h.rnCreationService

		oniImage, err := rnCreationService.GetOniImageFromBitbucket(ctx, branch, "customization", username, token)
		if err != nil {
			writeJSON(response, http.StatusOK, map[string]interface{}{
				"success": false,
				"message": "Failed to get ONI image: " + err.Error(),
			})
			return
		}

		writeJSON(response, http.StatusOK, map[string]interface{}{
			"success":   true,
			"oni_image": oniImage,
		})
	}
}

// HandleRNTableData handles RN table data generation requests
func (h *JenkinsHandlers) HandleRNTableData() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writeJSONError(response, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			CustomizationJobURL string `json:"customization_job_url"`
			CustomOrchZipURL    string `json:"custom_orch_zip_url"`
			OniImage            string `json:"oni_image"`
			StorageJobURL       string `json:"storage_job_url"`
			Username            string `json:"username"`
			Token               string `json:"token"`
		}
		if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
			writeJSONError(response, http.StatusBadRequest, "Invalid request payload")
			return
		}

		customizationJobURL := strings.TrimSpace(req.CustomizationJobURL)
		customOrchZipURL := strings.TrimSpace(req.CustomOrchZipURL)
		oniImage := strings.TrimSpace(req.OniImage)
		storageJobURL := strings.TrimSpace(req.StorageJobURL)
		username := strings.TrimSpace(req.Username)
		token := strings.TrimSpace(req.Token)

		log.Printf("DEBUG: RN Table Data parameters - storageJobURL: '%s'", storageJobURL)

		if customizationJobURL == "" {
			writeJSON(response, http.StatusOK, map[string]interface{}{
				"success": false,
				"message": "customization_job_url is required",
			})
			return
		}

		ctx := request.Context()
		var rnCreationService services.RNCreationService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				writeJSON(response, http.StatusOK, map[string]interface{}{
					"success": false,
					"message": "Failed to create Jenkins client: " + err.Error(),
				})
				return
			}
			rnCreationService = services.NewRNCreationService(h.configuration, tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		tableRequest := &types.RNTableRequest{
			CustomizationJobURL: customizationJobURL,
			CustomOrchZipURL:    customOrchZipURL,
			OniImage:            oniImage,
			StorageJobURL:       storageJobURL,
		}

		rnTableData, err := rnCreationService.GenerateRNTableData(ctx, tableRequest)
		if err != nil {
			log.Printf("ERROR: GenerateRNTableData failed: %v", err)
			writeJSON(response, http.StatusOK, map[string]interface{}{
				"success": false,
				"message": "Failed to generate RN table data: " + err.Error(),
			})
			return
		}

		writeJSON(response, http.StatusOK, map[string]interface{}{
			"success":       true,
			"rn_table_data": rnTableData,
		})
	}
}

// fetchOrchestrationFromNexus fetches orchestration artifact URL directly from Nexus
func (h *JenkinsHandlers) fetchOrchestrationFromNexus(branch string) (string, error) {
	log.Printf("Starting Nexus fetch for branch: %s", branch)

	normalizedBranch := branch
	if strings.HasPrefix(branch, "feature/") {
		normalizedBranch = strings.TrimPrefix(branch, "feature/")
		log.Printf("Normalized feature branch: %s -> %s", branch, normalizedBranch)
	} else if strings.HasPrefix(branch, "release/") {
		normalizedBranch = strings.TrimPrefix(branch, "release/")
		log.Printf("Normalized release branch: %s -> %s", branch, normalizedBranch)
	} else {
		log.Printf("Using branch as-is: %s", normalizedBranch)
	}

	nexusURL := h.configuration.Endpoints.NexusSearchURL
	log.Printf("Querying Nexus URL: %s", nexusURL)

	client := h.httpClient(10 * time.Second)

	log.Printf("Making HTTP request to Nexus...")
	resp, err := client.Get(nexusURL)
	if err != nil {
		log.Printf("Nexus HTTP request failed: %v", err)
		return "", fmt.Errorf("failed to query Nexus API: %v", err)
	}
	defer resp.Body.Close()

	log.Printf("Nexus API response status: %d", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		log.Printf("Nexus API returned non-200 status: %d", resp.StatusCode)
		return "", fmt.Errorf("Nexus API returned status %d", resp.StatusCode)
	}

	var nexusResp struct {
		Items []struct {
			Repository string `json:"repository"`
			Group      string `json:"group"`
			Name       string `json:"name"`
			Version    string `json:"version"`
			Assets     []struct {
				DownloadURL string `json:"downloadUrl"`
				Path        string `json:"path"`
			} `json:"assets"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&nexusResp); err != nil {
		log.Printf("Failed to decode Nexus JSON response: %v", err)
		return "", fmt.Errorf("failed to decode Nexus response: %v", err)
	}

	log.Printf("Nexus returned %d items", len(nexusResp.Items))

	var latestSrcZip string
	var latestTimestamp string
	versionPattern := fmt.Sprintf("10.4-%s-", normalizedBranch)

	for _, item := range nexusResp.Items {
		if item.Repository != "att.maven.snapshot" ||
			item.Group != "com.amdocs.oss.att.customization" ||
			item.Name != "att-orchestration" ||
			!strings.Contains(item.Version, versionPattern) {
			log.Printf("Skipping item: repository=%s, group=%s, name=%s, version=%s",
				item.Repository, item.Group, item.Name, item.Version)
			continue
		}

		log.Printf("Processing matching item: repository=%s, version=%s", item.Repository, item.Version)
		log.Printf("Processing item with %d assets", len(item.Assets))
		for _, asset := range item.Assets {
			log.Printf("Checking asset: %s", asset.Path)
			if strings.Contains(asset.Path, "-src.zip") {
				log.Printf("Found src.zip asset: %s", asset.Path)
				parts := strings.Split(asset.Path, "-")
				if len(parts) >= 6 {
					for _, part := range parts {
						if len(part) == 15 && strings.Contains(part, ".") {
							timestamp := part
							log.Printf("Found timestamp: %s (current latest: %s)", timestamp, latestTimestamp)
							if timestamp > latestTimestamp {
								latestTimestamp = timestamp
								latestSrcZip = asset.DownloadURL
								log.Printf("New latest src.zip: %s", latestSrcZip)
							}
							break
						}
					}
				}
			}
		}
	}

	if latestSrcZip == "" {
		log.Printf("No src.zip artifacts found for branch %s", normalizedBranch)
		return "", fmt.Errorf("no src.zip artifacts found for branch %s", normalizedBranch)
	}

	log.Printf("Selected latest src.zip: %s", latestSrcZip)

	finalURL := h.configuration.Endpoints.ReplaceWithInternalNexus(latestSrcZip)

	log.Printf("Final URL after host swapping: %s", finalURL)
	return finalURL, nil
}

func (h *JenkinsHandlers) httpClient(timeout time.Duration) *http.Client {
	client := &http.Client{Timeout: timeout}
	if h.configuration != nil && h.configuration.TLS.InsecureSkipVerify {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	return client
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

	mux.HandleFunc("/api/automation/builds", h.HandleAutomationBuilds())
	mux.HandleFunc("/api/automation/test-report", h.HandleAutomationTestReport())
	mux.HandleFunc("/api/automation/compare", h.HandleAutomationCompare())
	mux.HandleFunc("/api/automation/trends", h.HandleAutomationTrends())
}

func (h *JenkinsHandlers) HandleAutomationBuilds() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			Limit    int    `json:"limit"`
			JobPath  string `json:"jobPath"`
			Username string `json:"username"`
			Token    string `json:"token"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		service, err := h.automationServiceForRequest(req.Username, req.Token, req.JobPath)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := r.Context()
		builds, err := service.GetBuildList(ctx, req.Limit)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data": map[string]interface{}{
				"builds":     builds,
				"totalCount": len(builds),
			},
		})
	}
}

func (h *JenkinsHandlers) HandleAutomationTestReport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			BuildNumber int    `json:"buildNumber"`
			JobPath     string `json:"jobPath"`
			Username    string `json:"username"`
			Token       string `json:"token"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		service, err := h.automationServiceForRequest(req.Username, req.Token, req.JobPath)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := r.Context()
		report, err := service.GetTestReport(ctx, req.BuildNumber)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    report,
		})
	}
}

func (h *JenkinsHandlers) HandleAutomationCompare() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			BuildA   int    `json:"buildA"`
			BuildB   int    `json:"buildB"`
			JobPath  string `json:"jobPath"`
			Username string `json:"username"`
			Token    string `json:"token"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		service, err := h.automationServiceForRequest(req.Username, req.Token, req.JobPath)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := r.Context()
		comparison, err := service.CompareBuilds(ctx, req.BuildA, req.BuildB)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    comparison,
		})
	}
}

func (h *JenkinsHandlers) HandleAutomationTrends() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req struct {
			NumBuilds int    `json:"numBuilds"`
			JobPath   string `json:"jobPath"`
			Username  string `json:"username"`
			Token     string `json:"token"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
			return
		}

		service, err := h.automationServiceForRequest(req.Username, req.Token, req.JobPath)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := r.Context()
		trends, err := service.GetTestTrends(ctx, req.NumBuilds)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    trends,
		})
	}
}

func (h *JenkinsHandlers) automationServiceForRequest(username, token, jobPath string) (services.AutomationService, error) {
	hasEnvCredentials := h.client.IsConfigured()
	hasRequestCredentials := strings.TrimSpace(username) != "" && strings.TrimSpace(token) != ""

	if !hasEnvCredentials && !hasRequestCredentials {
		return nil, fmt.Errorf("Jenkins credentials not configured. Please set credentials in Settings.")
	}

	service := h.automationService

	if hasRequestCredentials {
		tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
			URL:      h.configuration.Endpoints.CustomizationJenkinsBaseURL,
			Username: username,
			Token:    token,
		})
		if err != nil {
			return nil, fmt.Errorf("Failed to create Jenkins client: %w", err)
		}
		service = services.NewAutomationService(h.configuration, tempClient)
	}

	if strings.TrimSpace(jobPath) != "" {
		service = service.WithJobPath(jobPath)
	}

	return service, nil
}
