package httpapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"app/internal/jenkins"
	"app/internal/jenkins/services"
	"app/internal/jenkins/types"
)

// JenkinsHandlers contains all Jenkins-related HTTP handlers
type JenkinsHandlers struct {
	client            *jenkins.Client
	scalingService    services.ScalingService
	artifactsService  services.ArtifactsService
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
			scalingService = services.NewScalingService(tempClient)
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
		var jobNumber int
		var username, token string

		if request.Method == "POST" {
			var req struct {
				JobNumber int    `json:"job_number"`
				Username  string `json:"username"`
				Token     string `json:"token"`
			}
			if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
				http.Error(response, "Invalid request format", http.StatusBadRequest)
				return
			}
			jobNumber = req.JobNumber
			username = req.Username
			token = req.Token

			log.Printf("DEBUG: HandleJenkinsStatus POST - jobNumber: %d, username: %s", jobNumber, username)

			// Validate job number
			if jobNumber <= 0 {
				log.Printf("ERROR: Invalid job number: %d", jobNumber)
				http.Error(response, "Invalid job number: must be greater than 0", http.StatusBadRequest)
				return
			}
		} else if request.Method == "GET" {
			jobNumberStr := request.URL.Query().Get("job_number")
			if jobNumberStr == "" {
				http.Error(response, "job_number parameter is required", http.StatusBadRequest)
				return
			}

			var err error
			jobNumber, err = strconv.Atoi(jobNumberStr)
			if err != nil {
				http.Error(response, "Invalid job_number format", http.StatusBadRequest)
				return
			}

			// Check for credentials in query parameters
			username = request.URL.Query().Get("username")
			token = request.URL.Query().Get("token")
		} else {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
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
				http.Error(response, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
				return
			}
			scalingService = services.NewScalingService(tempClient)
		} else {
			scalingService = h.scalingService
		}

		// Get job status
		jobStatus, err := scalingService.GetScaleJobStatus(ctx, jobNumber)
		if err != nil {
			http.Error(response, "Failed to get job status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(jobStatus)
	}
}

// HandleJenkinsQueueStatus handles Jenkins queue status queries
func (h *JenkinsHandlers) HandleJenkinsQueueStatus() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		var queueURL, username, token string

		if request.Method == "POST" {
			var req struct {
				QueueURL string `json:"queue_url"`
				Username string `json:"username"`
				Token    string `json:"token"`
			}
			if err := json.NewDecoder(request.Body).Decode(&req); err != nil {
				http.Error(response, "Invalid request format", http.StatusBadRequest)
				return
			}
			queueURL = req.QueueURL
			username = req.Username
			token = req.Token
		} else if request.Method == "GET" {
			queueURL = request.URL.Query().Get("queue_url")
			if queueURL == "" {
				http.Error(response, "queue_url parameter is required", http.StatusBadRequest)
				return
			}

			// Check for credentials in query parameters
			username = request.URL.Query().Get("username")
			token = request.URL.Query().Get("token")
		} else {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
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
				http.Error(response, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
				return
			}
			scalingService = services.NewScalingService(tempClient)
		} else {
			scalingService = h.scalingService
		}

		// Get queue status
		queueStatus, err := scalingService.GetQueueStatus(ctx, queueURL)
		if err != nil {
			http.Error(response, "Failed to get queue status: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(queueStatus)
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
		log.Printf("DEBUG: HandleJenkinsBuildInfo called - Method: %s, URL: %s", request.Method, request.URL.String())
		if request.Method != "GET" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		buildURL := request.URL.Query().Get("build_url")
		if buildURL == "" {
			http.Error(response, "build_url parameter is required", http.StatusBadRequest)
			return
		}

		// Check for credentials in query parameters
		username := request.URL.Query().Get("username")
		token := request.URL.Query().Get("token")

		// Create context for the request
		ctx := request.Context()

		// Use appropriate client for the request
		var jenkinsClient *jenkins.Client
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				http.Error(response, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
				return
			}
			jenkinsClient = tempClient
		} else {
			jenkinsClient = h.client
		}

		// For job URLs (without build number), get job info to find latest build
		if !regexp.MustCompile(`/\d+/?$`).MatchString(buildURL) {
			// Get trigger parameters for matching
			product := request.URL.Query().Get("product")
			coreVersion := request.URL.Query().Get("core_version")
			branchName := request.URL.Query().Get("branch_name")
			customOrchZipURL := request.URL.Query().Get("custom_orch_zip_url")
			oniImage := request.URL.Query().Get("oni_image")

			log.Printf("DEBUG: Received query parameters - product: '%s', core_version: '%s', branch_name: '%s', custom_orch_zip_url: '%s', oni_image: '%s'", product, coreVersion, branchName, customOrchZipURL, oniImage)

			// This is a job URL, get the job info to find latest build
			jobAPIURL := strings.TrimSuffix(buildURL, "/") + "/api/json"

			responseBody, err := jenkinsClient.GetWithAuth(ctx, jobAPIURL)
			if err != nil {
				http.Error(response, "Failed to get job info: "+err.Error(), http.StatusInternalServerError)
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
				http.Error(response, "Failed to parse job info: "+err.Error(), http.StatusInternalServerError)
				return
			}

			targetBuildNumber := jobInfo.LastBuild.Number

			// If we have trigger parameters, just check the latest build
			if product != "" || coreVersion != "" || branchName != "" || customOrchZipURL != "" || oniImage != "" {
				log.Printf("DEBUG: Trigger parameters - product: %s, core_version: %s, branch_name: %s, custom_orch_zip_url: %s, oni_image: %s", product, coreVersion, branchName, customOrchZipURL, oniImage)

				// Get build parameters for the latest build
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
						// Find parameters action
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

						log.Printf("DEBUG: Latest build %d parameters - branch_name: %s, oni_image: %s", targetBuildNumber, buildParams["branch_name"], buildParams["oni_image"])

						// Check if parameters match
						parametersMatch := true
						if branchName != "" && buildParams["branch_name"] != branchName {
							parametersMatch = false
						}
						if oniImage != "" && buildParams["oni_image"] != oniImage {
							parametersMatch = false
						}

						log.Printf("DEBUG: Latest build %d parameters match: %v", targetBuildNumber, parametersMatch)

						if parametersMatch {
							log.Printf("DEBUG: Using latest matching build: %d", targetBuildNumber)
						} else {
							log.Printf("DEBUG: Latest build doesn't match parameters, returning error for retry")
							http.Error(response, "Latest build parameters don't match", http.StatusNotFound)
							return
						}
					}
				}
			}

			// Return the job info with verified build number
			result := map[string]interface{}{
				"success": true,
				"build_info": map[string]interface{}{
					"lastBuild": map[string]interface{}{
						"number": targetBuildNumber,
						"url":    strings.TrimSuffix(buildURL, "/") + fmt.Sprintf("/%d", targetBuildNumber),
					},
				},
			}

			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(result)
			return
		}

		// For specific build URLs, get build info
		artifactsService := services.NewArtifactsService(jenkinsClient)
		buildInfo, err := artifactsService.GetBuildInfo(ctx, buildURL)
		if err != nil {
			http.Error(response, "Failed to get build info: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(buildInfo)
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
			rnCreationService = services.NewRNCreationService(tempClient)
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
		if request.Method != "GET" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		branch := request.URL.Query().Get("branch")
		if branch == "" {
			http.Error(response, "branch parameter is required", http.StatusBadRequest)
			return
		}

		// Check for Jenkins credentials in query parameters
		username := request.URL.Query().Get("username")
		token := request.URL.Query().Get("token")

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
				http.Error(response, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
				return
			}
			rnCreationService = services.NewRNCreationService(tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		// Get latest customization job
		job, err := rnCreationService.GetLatestCustomizationJob(ctx, branch)
		if err != nil {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to get customization job: " + err.Error(),
			})
			return
		}

		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"success": true,
			"job":     job,
		})
	}
}

// HandleRNBuildParameters handles requests to get build parameters
func (h *JenkinsHandlers) HandleRNBuildParameters() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != "GET" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jobURL := request.URL.Query().Get("job_url")
		if jobURL == "" {
			http.Error(response, "job_url parameter is required", http.StatusBadRequest)
			return
		}

		username := request.URL.Query().Get("username")
		token := request.URL.Query().Get("token")

		ctx := request.Context()
		var rnCreationService services.RNCreationService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				http.Error(response, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
				return
			}
			rnCreationService = services.NewRNCreationService(tempClient)
		} else {
			rnCreationService = h.rnCreationService
		}

		parameters, err := rnCreationService.GetBuildParameters(ctx, jobURL)
		if err != nil {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to get build parameters: " + err.Error(),
			})
			return
		}

		// Also get build description for TLC version extraction
		description, _ := rnCreationService.GetBuildDescription(ctx, jobURL)

		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"success":     true,
			"parameters":  parameters,
			"description": description,
		})
	}
}

// HandleRNArtifactURL handles requests to get artifact URL
func (h *JenkinsHandlers) HandleRNArtifactURL() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != "GET" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jobURL := request.URL.Query().Get("job_url")
		if jobURL == "" {
			http.Error(response, "job_url parameter is required", http.StatusBadRequest)
			return
		}

		username := request.URL.Query().Get("username")
		token := request.URL.Query().Get("token")

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
				http.Error(response, "Failed to create Jenkins client: "+err.Error(), http.StatusInternalServerError)
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
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to extract artifacts: " + err.Error(),
			})
			return
		}

		// Look for att-orchestration*src.zip artifact
		var orchestrationURL string
		for _, artifact := range artifactsResponse.Artifacts {
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
			// Fallback: Try to fetch directly from Nexus using branch name
			log.Printf("Jenkins artifact not found, trying Nexus direct fetch fallback...")
			branch := request.URL.Query().Get("branch")
			log.Printf("Branch parameter from request: '%s'", branch)
			if branch != "" {
				log.Printf("Calling fetchOrchestrationFromNexus with branch: %s", branch)
				fallbackURL, err := fetchOrchestrationFromNexus(branch)
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
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]interface{}{
					"success": false,
					"message": "att-orchestration*src.zip artifact not found in Jenkins artifacts or Nexus fallback",
				})
				return
			}
		}

		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"success":      true,
			"artifact_url": orchestrationURL,
		})
	}
}

// HandleRNOniImage handles requests to get ONI image
func (h *JenkinsHandlers) HandleRNOniImage() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != "GET" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		branch := request.URL.Query().Get("branch")
		if branch == "" {
			http.Error(response, "branch parameter is required", http.StatusBadRequest)
			return
		}

		username := request.URL.Query().Get("username")
		token := request.URL.Query().Get("token")

		if username == "" || token == "" {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Bitbucket credentials required",
			})
			return
		}

		ctx := request.Context()
		rnCreationService := h.rnCreationService

		oniImage, err := rnCreationService.GetOniImageFromBitbucket(ctx, branch, "customization", username, token)
		if err != nil {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to get ONI image: " + err.Error(),
			})
			return
		}

		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"success":   true,
			"oni_image": oniImage,
		})
	}
}

// HandleRNTableData handles RN table data generation requests
func (h *JenkinsHandlers) HandleRNTableData() http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		if request.Method != "GET" {
			http.Error(response, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get parameters from query string
		customizationJobURL := request.URL.Query().Get("customization_job_url")
		customOrchZipURL := request.URL.Query().Get("custom_orch_zip_url")
		oniImage := request.URL.Query().Get("oni_image")
		storageJobURL := request.URL.Query().Get("storage_job_url")

		log.Printf("DEBUG: RN Table Data parameters - storageJobURL: '%s'", storageJobURL)

		if customizationJobURL == "" {
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "customization_job_url parameter is required",
			})
			return
		}

		// Get credentials from query parameters (same pattern as HandleRNBuildParameters)
		username := request.URL.Query().Get("username")
		token := request.URL.Query().Get("token")

		ctx := request.Context()
		var rnCreationService services.RNCreationService
		if username != "" && token != "" {
			tempClient, err := jenkins.NewClientWithConfig(jenkins.ClientConfig{
				URL:      h.client.GetBaseURL(),
				Username: username,
				Token:    token,
			})
			if err != nil {
				response.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(response).Encode(map[string]interface{}{
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
		tableRequest := &types.RNTableRequest{
			CustomizationJobURL: customizationJobURL,
			CustomOrchZipURL:    customOrchZipURL,
			OniImage:            oniImage,
			StorageJobURL:       storageJobURL,
		}

		// Generate RN table data
		rnTableData, err := rnCreationService.GenerateRNTableData(ctx, tableRequest)
		if err != nil {
			log.Printf("ERROR: GenerateRNTableData failed: %v", err)
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]interface{}{
				"success": false,
				"message": "Failed to generate RN table data: " + err.Error(),
			})
			return
		}

		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(map[string]interface{}{
			"success":       true,
			"rn_table_data": rnTableData,
		})
	}
}

// fetchOrchestrationFromNexus fetches orchestration artifact URL directly from Nexus
func fetchOrchestrationFromNexus(branch string) (string, error) {
	log.Printf("Starting Nexus fetch for branch: %s", branch)

	// Normalize branch name - remove feature/ and release/ prefixes
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

	// Construct Nexus search URL - use wildcard to match version timestamps
	nexusURL := fmt.Sprintf("https://oss-nexus2.oss.corp.amdocs.aws/service/rest/v1/search?repository=att.maven.snapshot&group=com.amdocs.oss.att.customization&name=att-orchestration&version=10.4-%s*", normalizedBranch)
	log.Printf("Querying Nexus URL: %s", nexusURL)

	// Create HTTP client with aggressive timeout and no SSL verification
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Make request to Nexus API
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

	// Parse JSON response
	var nexusResp struct {
		Items []struct {
			Assets []struct {
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

	// Find the latest src.zip file
	var latestSrcZip string
	var latestTimestamp string

	for _, item := range nexusResp.Items {
		log.Printf("Processing item with %d assets", len(item.Assets))
		for _, asset := range item.Assets {
			log.Printf("Checking asset: %s", asset.Path)
			if strings.Contains(asset.Path, "-src.zip") {
				log.Printf("Found src.zip asset: %s", asset.Path)
				// Extract timestamp from path like: att-orchestration-10.4-develop-20250916.141559-535-src.zip
				parts := strings.Split(asset.Path, "-")
				if len(parts) >= 6 {
					// Find timestamp part (format: YYYYMMDD.HHMMSS)
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

	// Apply host swapping for internal network access
	// From: https://oss-nexus2.oss.corp.amdocs.aws/repository/att.maven.snapshot/
	// To: http://illin3613:8081/repository/maven.group/
	finalURL := strings.Replace(latestSrcZip, "https://oss-nexus2.oss.corp.amdocs.aws/repository/att.maven.snapshot/", "http://illin3613:8081/repository/maven.group/", 1)

	log.Printf("Final URL after host swapping: %s", finalURL)
	return finalURL, nil
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
