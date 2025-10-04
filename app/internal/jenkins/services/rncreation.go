package services

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"app/internal/config"
	"app/internal/jenkins/types"
	ocdscripts "deploy-scripts"
)

// Constants for script paths and configurations
const (
	// Removed external script paths - now using embedded scripts
	helmChartsScriptName    = "get-helm-charts.sh"
	imageVersionsScriptName = "get-image-versions.sh"

	// Application constants
	defaultApplication  = "NEO-OSO"
	defaultDefectNumber = "[To be populated]"
	defaultCommitID     = "NA"
)

// RNCreationServiceImpl implements the RNCreationService interface
type RNCreationServiceImpl struct {
	configuration *config.Config
	client        JenkinsClient
}

// NewRNCreationService creates a new RN Creation service instance
func NewRNCreationService(configuration *config.Config, client JenkinsClient) RNCreationService {
	return &RNCreationServiceImpl{
		configuration: configuration,
		client:        client,
	}
}

func (s *RNCreationServiceImpl) storageJobURL(parts ...string) string {
	if s.configuration == nil {
		defaults := config.DefaultEndpoints()
		return defaults.StorageJobURL(parts...)
	}
	return s.configuration.Endpoints.StorageJobURL(parts...)
}

func (s *RNCreationServiceImpl) customizationBaseURL() string {
	base := config.DefaultEndpoints().CustomizationJenkinsBaseURL
	if s.configuration != nil {
		configured := strings.TrimSpace(s.configuration.Endpoints.CustomizationJenkinsBaseURL)
		if configured != "" {
			base = configured
		}
	}
	return strings.TrimRight(base, "/")
}

func (s *RNCreationServiceImpl) bitbucketBaseURL() string {
	base := config.DefaultEndpoints().BitbucketBaseURL
	if s.configuration != nil {
		configured := strings.TrimSpace(s.configuration.Endpoints.BitbucketBaseURL)
		if configured != "" {
			base = configured
		}
	}
	return strings.TrimRight(base, "/")
}

func (s *RNCreationServiceImpl) httpClient(timeout time.Duration) *http.Client {
	client := &http.Client{Timeout: timeout}
	if s.configuration != nil && s.configuration.TLS.InsecureSkipVerify {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}
	return client
}

// TriggerStorageCreation triggers the ATT_Storage_Creation Jenkins job
func (s *RNCreationServiceImpl) TriggerStorageCreation(ctx context.Context, request *types.RNCreationRequest) (*types.RNCreationResponse, error) {
	// Validate request
	if validation := s.validateRequest(request); !validation.Valid {
		return nil, fmt.Errorf("validation failed: %v", validation.Errors)
	}

	jobURL := s.storageJobURL("buildWithParameters")

	// Prepare parameters from request
	params := map[string]string{
		"product":             request.Product,
		"core_version":        request.CoreVersion,
		"env_login":           request.EnvLogin,
		"build_chart_version": request.BuildChartVersion,
		"branch_name":         request.BranchName,
		"custom_orch_zip_url": request.CustomOrchZipURL,
		"oni_image":           request.OniImage,
		"email":               request.Email,
		"layering":            request.Layering,
	}

	// Make POST request to trigger job using direct HTTP client
	// since this Jenkins server is different from the configured one
	// For the original method, we cannot access credentials, so this will likely fail
	// Users should use TriggerStorageCreationWithCredentials instead
	err := s.makeStorageCreationRequestWithAuth(ctx, jobURL, params, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to trigger storage creation job: %w", err)
	}

	// Return response with job URL
	rootURL := s.storageJobURL()

	return &types.RNCreationResponse{
		JobStatus: &types.JobStatus{
			Status: "queued",
			URL:    rootURL,
		},
		Message: "Storage creation job triggered successfully",
		JobURL:  rootURL,
	}, nil
}

// GetLatestCustomizationJob retrieves the latest successful/unstable customization job for a branch
func (s *RNCreationServiceImpl) GetLatestCustomizationJob(ctx context.Context, branch string) (*types.CustomizationJob, error) {
	if branch == "" {
		return nil, fmt.Errorf("branch name cannot be empty")
	}

	// Encode branch name for URL - Jenkins job names with slashes need %252F encoding
	encodedBranch := strings.ReplaceAll(branch, "/", "%252F")

	// Jenkins delivery customization job URL with build details
	// Use tree parameter to get build results in one call
	base := s.customizationBaseURL()
	jobURL := fmt.Sprintf("%s/job/Delivery/job/ATT_OSO/job/customization/job/%s/api/json?tree=builds[number,url,result,timestamp,building]", base, encodedBranch)

	// Get job information
	data, err := s.client.GetWithAuth(ctx, jobURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get customization job info for branch '%s' from URL '%s': %w", branch, jobURL, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty response from Jenkins for branch '%s'", branch)
	}

	// Parse job information
	var jobInfo struct {
		Builds []struct {
			Number    int    `json:"number"`
			URL       string `json:"url"`
			Result    string `json:"result"`
			Timestamp int64  `json:"timestamp"`
			Building  bool   `json:"building"`
		} `json:"builds"`
	}

	if err := json.Unmarshal(data, &jobInfo); err != nil {
		return nil, fmt.Errorf("failed to parse job info for branch '%s': %w", branch, err)
	}

	if len(jobInfo.Builds) == 0 {
		return nil, fmt.Errorf("no builds found for branch '%s'", branch)
	}

	// Find the latest successful or unstable build
	for _, build := range jobInfo.Builds {
		// Check for SUCCESS or UNSTABLE (case-insensitive) and not currently building
		resultUpper := strings.ToUpper(build.Result)
		if !build.Building && (resultUpper == "SUCCESS" || resultUpper == "UNSTABLE") {
			if build.URL == "" {
				return nil, fmt.Errorf("build URL is empty for build %d on branch '%s'", build.Number, branch)
			}
			return &types.CustomizationJob{
				Number:    build.Number,
				URL:       build.URL,
				Status:    strings.ToLower(build.Result),
				Result:    build.Result,
				Timestamp: time.Unix(build.Timestamp/1000, 0),
				Branch:    branch,
				Building:  build.Building,
			}, nil
		}
	}

	return nil, fmt.Errorf("no successful or unstable builds found for branch '%s'. Found %d builds, but none completed successfully", branch, len(jobInfo.Builds))
}

// getBuildInfo retrieves and parses Jenkins build info (generic function to avoid duplication)
func (s *RNCreationServiceImpl) getBuildInfo(ctx context.Context, jobURL string) (*types.JenkinsBuildInfo, error) {
	if jobURL == "" {
		return nil, fmt.Errorf("job URL cannot be empty")
	}

	// Get build info API URL - remove trailing slash and add api/json
	apiURL := strings.TrimSuffix(jobURL, "/") + "/api/json"

	data, err := s.client.GetWithAuth(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get build info from '%s': %w", apiURL, err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty response from Jenkins for URL '%s'", apiURL)
	}

	// Parse build info with all fields we might need
	var buildInfo types.JenkinsBuildInfo
	if err := json.Unmarshal(data, &buildInfo); err != nil {
		return nil, fmt.Errorf("failed to parse build info: %w", err)
	}

	return &buildInfo, nil
}

// GetTLCVersionFromJob extracts TLC version from a customization job
// Uses the same approach as GetEKSClusterNameFromJob - from build description
func (s *RNCreationServiceImpl) GetTLCVersionFromJob(ctx context.Context, jobURL string) (string, error) {
	description, err := s.GetBuildDescription(ctx, jobURL)
	if err != nil {
		return "", fmt.Errorf("failed to get build description: %w", err)
	}

	// Extract TLC version from description using existing method
	tlcVersion := s.extractValueFromDescription(description, "TLC Version")
	if tlcVersion == "" {
		return "", fmt.Errorf("TLC Version not found in build description")
	}

	return tlcVersion, nil
}

// GetEKSClusterNameFromJob extracts eks_clustername from a customization job
// Uses the same approach as TLC Version extraction - from build description
func (s *RNCreationServiceImpl) GetEKSClusterNameFromJob(ctx context.Context, jobURL string) (string, error) {
	description, err := s.GetBuildDescription(ctx, jobURL)
	if err != nil {
		return "", fmt.Errorf("failed to get build description: %w", err)
	}

	// Extract eks_clustername from description using regex (same pattern as TLC Version)
	clusterName := s.extractValueFromDescription(description, "eks_clustername")
	if clusterName == "" {
		return "", fmt.Errorf("eks_clustername not found in build description")
	}

	return clusterName, nil
}

// GetOniImageFromBitbucket retrieves the latest oni_image value from Bitbucket commit messages
func (s *RNCreationServiceImpl) GetOniImageFromBitbucket(ctx context.Context, branch, repoName, username, token string) (string, error) {
	// Validate input parameters
	if branch == "" {
		return "", fmt.Errorf("branch name cannot be empty")
	}
	if repoName == "" {
		return "", fmt.Errorf("repository name cannot be empty")
	}
	if username == "" || token == "" {
		return "", fmt.Errorf("Bitbucket credentials (username and token) are required")
	}

	client := s.httpClient(30 * time.Second)

	// Bitbucket API URL for commits
	base := s.bitbucketBaseURL()
	project := ""
	if s.configuration != nil {
		project = s.configuration.Endpoints.BitbucketProjectKey
	}
	if project == "" {
		project = "ATTSVO"
	}
	url := fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/commits?until=%s&limit=50", base, project, repoName, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create Bitbucket request: %w", err)
	}

	// Set basic auth
	req.SetBasicAuth(username, token)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make Bitbucket request for branch '%s': %w", branch, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 401 {
			return "", fmt.Errorf("authentication failed for Bitbucket (status %d). Please check your credentials", resp.StatusCode)
		}
		if resp.StatusCode == 404 {
			return "", fmt.Errorf("repository or branch not found (status %d): %s/%s", resp.StatusCode, repoName, branch)
		}
		return "", fmt.Errorf("Bitbucket API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read Bitbucket response: %w", err)
	}

	if len(body) == 0 {
		return "", fmt.Errorf("empty response from Bitbucket for branch '%s'", branch)
	}

	// Parse commits response
	var commitsResp struct {
		Values []struct {
			Author struct {
				DisplayName string `json:"displayName"`
			} `json:"author"`
			Message string `json:"message"`
		} `json:"values"`
	}

	if err := json.Unmarshal(body, &commitsResp); err != nil {
		return "", fmt.Errorf("failed to parse Bitbucket commits response: %w", err)
	}

	if len(commitsResp.Values) == 0 {
		return "", fmt.Errorf("no commits found for branch '%s' in repository '%s'", branch, repoName)
	}

	// Look for commits by jenkins with oni_docker_version message
	re := regexp.MustCompile(`update oni_docker_version with value\s*(.+)`)
	jenkinsCommitCount := 0

	for _, commit := range commitsResp.Values {
		if strings.ToLower(commit.Author.DisplayName) == "jenkins" {
			jenkinsCommitCount++
			matches := re.FindStringSubmatch(commit.Message)
			if len(matches) >= 2 {
				oniValue := strings.TrimSpace(matches[1])
				if oniValue != "" {
					return oniValue, nil
				}
			}
		}
	}

	if jenkinsCommitCount == 0 {
		return "", fmt.Errorf("no Jenkins commits found in recent commits for branch '%s'", branch)
	}

	return "", fmt.Errorf("oni_docker_version not found in %d recent Jenkins commits for branch '%s'", jenkinsCommitCount, branch)
}

// ValidateRNRequest validates an RN creation request
func (s *RNCreationServiceImpl) ValidateRNRequest(request *types.RNCreationRequest) (*types.ValidationResult, error) {
	return s.validateRequest(request), nil
}

// PopulateRequestFromCustomizationJob auto-populates request fields from customization job
func (s *RNCreationServiceImpl) PopulateRequestFromCustomizationJob(ctx context.Context, request *types.RNCreationRequest) error {
	// Get latest customization job if not provided
	customizationJobURL := request.CustomizationJobURL
	if customizationJobURL == "" {
		job, err := s.GetLatestCustomizationJob(ctx, request.Branch)
		if err != nil {
			return fmt.Errorf("failed to get latest customization job: %w", err)
		}
		customizationJobURL = job.URL
		request.CustomizationJobURL = customizationJobURL
	}

	// Get TLC version
	if request.BuildChartVersion == "" {
		tlcVersion, err := s.GetTLCVersionFromJob(ctx, customizationJobURL)
		if err != nil {
			return fmt.Errorf("failed to get TLC version: %w", err)
		}
		request.BuildChartVersion = tlcVersion
	}

	// Artifact URL will be populated by the frontend using existing artifacts service

	// Default values are set by the frontend, no hardcoding here
	if request.BranchName == "" {
		request.BranchName = request.Branch
	}

	return nil
}

// validateRequest performs validation on the RN creation request
func (s *RNCreationServiceImpl) validateRequest(request *types.RNCreationRequest) *types.ValidationResult {
	result := &types.ValidationResult{Valid: true}

	if strings.TrimSpace(request.Branch) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "branch is required")
	}

	// Core Version is required as user must select from dropdown
	if strings.TrimSpace(request.CoreVersion) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "core_version is required")
	}

	// Email is required for notifications
	if strings.TrimSpace(request.Email) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "email is required")
	}

	// Product, env_login, and layering have sensible defaults from frontend

	return result
}

// GetBuildParameters retrieves build parameters from a Jenkins job
func (s *RNCreationServiceImpl) GetBuildParameters(ctx context.Context, jobURL string) ([]types.JobParameter, error) {
	buildInfo, err := s.getBuildInfo(ctx, jobURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get build info: %w", err)
	}

	var parameters []types.JobParameter

	// Look for parameter actions
	for _, action := range buildInfo.Actions {
		if strings.Contains(action.Class, "ParametersAction") {
			for _, param := range action.Parameters {
				// Convert value to string
				valueStr := ""
				if param.Value != nil {
					valueStr = fmt.Sprintf("%v", param.Value)
				}

				jobParam := types.JobParameter{
					Name:  param.Name,
					Value: valueStr,
					Type:  param.Class,
				}
				parameters = append(parameters, jobParam)
			}
			break
		}
	}

	return parameters, nil
}

// GetBuildDescription retrieves build description from a Jenkins job
func (s *RNCreationServiceImpl) GetBuildDescription(ctx context.Context, jobURL string) (string, error) {
	buildInfo, err := s.getBuildInfo(ctx, jobURL)
	if err != nil {
		return "", fmt.Errorf("failed to get build info: %w", err)
	}

	return buildInfo.Description, nil
}

// TriggerStorageCreationWithCredentials triggers the ATT_Storage_Creation Jenkins job with explicit credentials
func (s *RNCreationServiceImpl) TriggerStorageCreationWithCredentials(ctx context.Context, request *types.RNCreationRequest, username, token string) (*types.RNCreationResponse, error) {
	// Validate request
	if validation := s.validateRequest(request); !validation.Valid {
		return nil, fmt.Errorf("validation failed: %v", validation.Errors)
	}

	jobURL := s.storageJobURL("buildWithParameters")

	// Prepare parameters from request
	params := map[string]string{
		"product":             request.Product,
		"core_version":        request.CoreVersion,
		"env_login":           request.EnvLogin,
		"build_chart_version": request.BuildChartVersion,
		"branch_name":         request.BranchName,
		"custom_orch_zip_url": request.CustomOrchZipURL,
		"oni_image":           request.OniImage,
		"email":               request.Email,
		"layering":            request.Layering,
	}

	// Make POST request to trigger job using credentials
	err := s.makeStorageCreationRequestWithAuth(ctx, jobURL, params, username, token)
	if err != nil {
		return nil, fmt.Errorf("failed to trigger storage creation job: %w", err)
	}

	// Return response with job URL
	rootURL := s.storageJobURL()

	return &types.RNCreationResponse{
		JobStatus: &types.JobStatus{
			Status: "queued",
			URL:    rootURL,
		},
		Message: "Storage creation job triggered successfully",
		JobURL:  rootURL,
	}, nil
}

// makeStorageCreationRequestWithAuth makes a direct HTTP request to the storage creation Jenkins server with auth
func (s *RNCreationServiceImpl) makeStorageCreationRequestWithAuth(ctx context.Context, jobURL string, params map[string]string, username, token string) error {
	// Create HTTP client with TLS config and cookie jar for session management
	jar, _ := cookiejar.New(nil)
	client := s.httpClient(30 * time.Second)
	client.Jar = jar // Enable cookie support for session management

	// Get crumb token first for CSRF protection (this will establish session)
	crumb, crumbField, err := s.getCrumbToken(ctx, client, username, token)
	if err != nil {
		return fmt.Errorf("failed to get crumb token: %w", err)
	}

	// Build form data
	data := url.Values{}
	for key, value := range params {
		data.Set(key, value)
	}

	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", jobURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Add CSRF crumb if available
	if crumb != "" && crumbField != "" {
		req.Header.Set(crumbField, crumb)
	}

	// Use explicit Jenkins credentials
	if username != "" && token != "" {
		req.SetBasicAuth(username, token)
	}

	// Make the request (cookies from crumb request will be automatically included)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request to storage creation Jenkins: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("storage creation request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// getCrumbToken retrieves the CSRF crumb token from Jenkins
func (s *RNCreationServiceImpl) getCrumbToken(ctx context.Context, client *http.Client, username, token string) (string, string, error) {
	// Get the base URL from the job URL
	baseURL := config.DefaultEndpoints().StorageJenkinsBaseURL
	if s.configuration != nil {
		configured := strings.TrimSpace(s.configuration.Endpoints.StorageJenkinsBaseURL)
		if configured != "" {
			baseURL = configured
		}
	}
	baseURL = strings.TrimRight(baseURL, "/")
	crumbURL := baseURL + "/crumbIssuer/api/json"

	req, err := http.NewRequestWithContext(ctx, "GET", crumbURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create crumb request: %w", err)
	}

	// Set authentication
	if username != "" && token != "" {
		req.SetBasicAuth(username, token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to get crumb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("failed to get crumb, status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read crumb response: %w", err)
	}

	var crumbResp struct {
		Crumb             string `json:"crumb"`
		CrumbRequestField string `json:"crumbRequestField"`
	}

	if err := json.Unmarshal(body, &crumbResp); err != nil {
		return "", "", fmt.Errorf("failed to parse crumb response: %w", err)
	}

	return crumbResp.Crumb, crumbResp.CrumbRequestField, nil
}

// GetCorePatchCharts executes kubectl commands on EKS cluster to get helm charts info
func (s *RNCreationServiceImpl) GetCorePatchCharts(ctx context.Context, clusterName string) ([]types.CorePatchInfo, error) {
	startTime := time.Now()
	log.Printf("[TIMING] GetCorePatchCharts started for cluster: %s", clusterName)

	if clusterName == "" {
		return nil, fmt.Errorf("cluster name cannot be empty")
	}

	// Step 1: Create temporary script with proper setup
	step1Start := time.Now()
	tempDir, scriptPath, err := s.createTempScript()
	if err != nil {
		return nil, fmt.Errorf("failed to create temp script: %w", err)
	}
	defer os.RemoveAll(tempDir)
	step1Duration := time.Since(step1Start)
	log.Printf("[TIMING] Step 1 - Create temp script: %v", step1Duration)

	// Step 2: Execute the script
	step2Start := time.Now()
	output, err := s.executeHelmScript(ctx, scriptPath, clusterName, tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get helm charts for cluster '%s': %w", clusterName, err)
	}
	step2Duration := time.Since(step2Start)
	log.Printf("[TIMING] Step 2 - Execute helm script: %v", step2Duration)

	// Step 3: Parse the output
	step3Start := time.Now()
	result := s.parseHelmOutput(output)
	step3Duration := time.Since(step3Start)
	log.Printf("[TIMING] Step 3 - Parse helm output: %v", step3Duration)

	totalDuration := time.Since(startTime)
	log.Printf("[TIMING] GetCorePatchCharts TOTAL: %v", totalDuration)

	return result, nil
}

// executeCommand executes a shell command (for commands that don't need output)
func (s *RNCreationServiceImpl) executeCommand(ctx context.Context, command string) error {
	_, err := s.executeCommandWithOutput(ctx, command)
	return err
}

// executeCommandWithOutput executes a shell command and returns the output
func (s *RNCreationServiceImpl) executeCommandWithOutput(ctx context.Context, command string) (string, error) {
	return s.executeCommandWithOutputInDir(ctx, command, "../deploy-scripts/scripts")
}

// executeCommandWithOutputInDir executes a shell command in a specific directory and returns the output
// Uses the same proven patterns as the main OCD command executor
func (s *RNCreationServiceImpl) executeCommandWithOutputInDir(ctx context.Context, command string, workingDir string) (string, error) {
	configuration := config.Load()

	var cmd *exec.Cmd

	// Use the exact same pattern as the main command executor
	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wsl"); err == nil {
			// Convert paths to WSL format using the same function as command executor
			wslWorkingDir := s.convertToWSLPath(workingDir)
			wslCommand := s.convertToWSLCommand(command)

			// Build WSL command using the same pattern as buildWSLDirectCommand
			fullCommand := s.buildWSLCommand(wslCommand, wslWorkingDir)

			cmd = exec.CommandContext(ctx, "wsl", "--user", configuration.WSLUser, "bash", "-l", "-c", fullCommand)
		} else {
			return "", fmt.Errorf("WSL not available on Windows. Please install WSL to use OCD")
		}
	case "linux", "darwin":
		// Use the same command execution pattern as the rest of the OCD application
		cmd = exec.CommandContext(ctx, "bash", "-l", "-c", command)
		cmd.Dir = workingDir
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Set up proper environment to avoid terminal issues and kubectl problems
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",            // Fix terminal type
		"COLUMNS=120",                    // Fix screen width
		"LINES=30",                       // Fix screen height
		"DEBIAN_FRONTEND=noninteractive", // Prevent interactive prompts
		"AWS_PAGER=",                     // Disable AWS CLI pager
	)

	log.Printf("DEBUG: Executing command in %s: %s", workingDir, command)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		log.Printf("ERROR: Command failed in %s: %s, Error: %v, Output: %s", workingDir, command, err, outputStr)
		return "", fmt.Errorf("command execution failed: %w, output: %s", err, outputStr)
	}

	log.Printf("DEBUG: Command completed successfully, output length: %d bytes", len(outputStr))
	return outputStr, nil
}

// convertToWSLPath converts Windows paths to WSL paths (same as command executor)
func (s *RNCreationServiceImpl) convertToWSLPath(windowsPath string) string {
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

// buildWSLCommand builds a WSL command using the same pattern as buildWSLDirectCommand
func (s *RNCreationServiceImpl) buildWSLCommand(command, workingDir string) string {
	// Use the same pattern as buildWSLDirectCommand in command executor
	return fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on 2>/dev/null || true && cd %s && %s`, s.shellEscape(workingDir), command)
}

// convertToWSLCommand converts Windows command with paths to WSL-compatible command
func (s *RNCreationServiceImpl) convertToWSLCommand(command string) string {
	if runtime.GOOS != "windows" {
		return command
	}

	// Convert Windows paths in the command to WSL paths
	wslCommand := command

	// Replace Windows drive paths with WSL paths
	if strings.Contains(wslCommand, "C:") {
		wslCommand = strings.ReplaceAll(wslCommand, "C:", "/mnt/c")
	}
	wslCommand = strings.ReplaceAll(wslCommand, "\\", "/")

	return wslCommand
}

// extractValueFromDescription extracts a value from Jenkins build description using regex
func (s *RNCreationServiceImpl) extractValueFromDescription(description, key string) string {
	pattern := fmt.Sprintf(`%s\s*[=:]\s*([^<\s,]+)`, regexp.QuoteMeta(key))
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(description)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// createTempScriptFromEmbedded creates a temporary script from embedded script with proper line ending handling
func (s *RNCreationServiceImpl) createTempScriptFromEmbedded(scriptName, tempDirPrefix string) (string, string, error) {
	// Read and normalize embedded script content
	scriptContent, err := s.readAndNormalizeEmbeddedScript(scriptName)
	if err != nil {
		return "", "", fmt.Errorf("failed to read embedded script: %w", err)
	}

	// Create temporary directory and script
	tempDir, err := os.MkdirTemp("", tempDirPrefix+"_*")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	tempScriptPath := filepath.Join(tempDir, scriptName)
	if err := os.WriteFile(tempScriptPath, []byte(scriptContent), 0755); err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to write temp script: %w", err)
	}

	// Setup shared scripts
	if err := s.setupSharedScripts(tempDir); err != nil {
		os.RemoveAll(tempDir)
		return "", "", fmt.Errorf("failed to setup shared scripts: %w", err)
	}

	return tempDir, tempScriptPath, nil
}

// createTempScript creates a temporary script for helm charts with proper line ending handling
func (s *RNCreationServiceImpl) createTempScript() (string, string, error) {
	return s.createTempScriptFromEmbedded(
		helmChartsScriptName,
		"helm-charts-script",
	)
}

// readAndNormalizeEmbeddedScript reads an embedded script and normalizes line endings
func (s *RNCreationServiceImpl) readAndNormalizeEmbeddedScript(scriptName string) (string, error) {
	scriptBytes, err := ocdscripts.ReadScript(scriptName)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded script %s: %w", scriptName, err)
	}

	// Convert Windows line endings to Unix line endings for bash compatibility
	content := strings.ReplaceAll(string(scriptBytes), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return content, nil
}

// setupSharedScripts copies embedded shared scripts to temp directory with line ending conversion
func (s *RNCreationServiceImpl) setupSharedScripts(tempDir string) error {
	tempSharedDir := filepath.Join(tempDir, "shared")
	if err := os.MkdirAll(tempSharedDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp shared dir: %w", err)
	}

	// Get list of embedded shared scripts
	sharedEntries, err := ocdscripts.ReadDir("scripts/shared")
	if err != nil {
		return fmt.Errorf("failed to read embedded shared scripts directory: %w", err)
	}

	for _, entry := range sharedEntries {
		if strings.HasSuffix(entry.Name(), ".sh") {
			if err := s.copyEmbeddedSharedScript(entry.Name(), tempSharedDir); err != nil {
				// Log error but continue with other files
				log.Printf("Warning: Failed to copy embedded shared script %s: %v", entry.Name(), err)
			}
		}
	}

	return nil
}

// copyEmbeddedSharedScript copies a single embedded shared script with line ending conversion
func (s *RNCreationServiceImpl) copyEmbeddedSharedScript(scriptName, destDir string) error {
	// Read embedded shared script content
	scriptBytes, err := ocdscripts.ReadShared(scriptName)
	if err != nil {
		return fmt.Errorf("failed to read embedded shared script %s: %w", scriptName, err)
	}

	// Convert Windows line endings to Unix line endings for bash compatibility
	content := strings.ReplaceAll(string(scriptBytes), "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	destPath := filepath.Join(destDir, scriptName)
	return os.WriteFile(destPath, []byte(content), 0644)
}

// executeHelmScript executes the helm charts script
func (s *RNCreationServiceImpl) executeHelmScript(ctx context.Context, scriptPath, clusterName, workingDir string) (string, error) {
	command := fmt.Sprintf("%s %s", scriptPath, clusterName)
	return s.executeCommandWithOutputInDir(ctx, command, workingDir)
}

// shellEscape safely escapes a string for use in shell commands (same as command executor)
func (s *RNCreationServiceImpl) shellEscape(str string) string {
	return strconv.Quote(str)
}

// parseHelmOutput parses the output from helm ls commands across namespaces
func (s *RNCreationServiceImpl) parseHelmOutput(output string) []types.CorePatchInfo {
	var result []types.CorePatchInfo

	lines := strings.Split(output, "\n")
	currentNamespace := ""
	var currentCharts []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a namespace header (ends with "namespace:")
		if strings.HasSuffix(line, "namespace:") {
			// Save previous namespace data if exists
			if currentNamespace != "" && len(currentCharts) > 0 {
				result = append(result, types.CorePatchInfo{
					Namespace: currentNamespace,
					Charts:    currentCharts,
				})
			}

			// Start new namespace
			currentNamespace = strings.TrimSuffix(line, " namespace:")
			currentCharts = []string{}
		} else {
			// This is a chart name
			if currentNamespace != "" && line != "" {
				currentCharts = append(currentCharts, line)
			}
		}
	}

	// Add the last namespace if it has data
	if currentNamespace != "" && len(currentCharts) > 0 {
		result = append(result, types.CorePatchInfo{
			Namespace: currentNamespace,
			Charts:    currentCharts,
		})
	}

	return result
}

// ColumnData represents the result of fetching data for a specific table column
type ColumnData struct {
	Name     string
	Content  string
	Error    error
	Duration time.Duration
}

// GenerateRNTableData generates the complete data structure for RN table using safe sequential execution
func (s *RNCreationServiceImpl) GenerateRNTableData(ctx context.Context, request *types.RNTableRequest) (*types.RNTableData, error) {
	overallStart := time.Now()
	log.Printf("[TIMING] GenerateRNTableData started")

	// Validate input
	if err := s.validateRNTableRequest(request); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Step 1: Get EKS cluster name (required by other operations)
	clusterStart := time.Now()
	clusterName, err := s.GetEKSClusterNameFromJob(ctx, request.CustomizationJobURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get EKS cluster name: %w", err)
	}
	log.Printf("[TIMING] Get EKS cluster name: %v", time.Since(clusterStart))

	// Step 2: Get TLC version (no kubectl dependency)
	tlcStart := time.Now()
	tlcVersion, err := s.GetTLCVersionFromJob(ctx, request.CustomizationJobURL)
	if err != nil {
		log.Printf("ERROR: Failed to get TLC version: %v", err)
		tlcVersion = "[TLC version unavailable]"
	}
	log.Printf("[TIMING] Get TLC version: %v", time.Since(tlcStart))

	// Step 3: Get core patch charts (uses kubectl - must be sequential)
	coreStart := time.Now()
	corePatchCharts, err := s.populateCorePatchChartsColumn(ctx, clusterName)
	if err != nil {
		log.Printf("ERROR: Failed to get core patch charts: %v", err)
		corePatchCharts = fmt.Sprintf("[Error connecting to cluster '%s']", clusterName)
	}
	log.Printf("[TIMING] Get core patch charts: %v", time.Since(coreStart))

	// Step 4: Get image versions (uses kubectl - must be sequential after step 3)
	imageStart := time.Now()
	attImage, guidedTaskImage, customizationImage, err := s.GetImageVersions(ctx, clusterName)
	if err != nil {
		log.Printf("ERROR: Failed to get image versions: %v", err)
		attImage = "[ATT image unavailable]"
		guidedTaskImage = "[Guided task image unavailable]"
		customizationImage = "[Customization image unavailable]"
	}
	log.Printf("[TIMING] Get image versions: %v", time.Since(imageStart))

	// Step 5: Format all data
	formatStart := time.Now()
	customOrchZip := s.formatCustomOrchZip(request.CustomOrchZipURL)
	commentsInstructions := s.formatCommentsInstructions(tlcVersion, clusterName, request.OniImage, attImage, guidedTaskImage, customizationImage, request.StorageJobURL)
	log.Printf("[TIMING] Format data: %v", time.Since(formatStart))

	// Build final result
	tableData := &types.RNTableData{
		Application:            defaultApplication,
		DefectNumber:           defaultDefectNumber,
		CorePatchCharts:        corePatchCharts,
		CustomOrchestrationZip: customOrchZip,
		CommitID:               defaultCommitID,
		CommentsInstructions:   commentsInstructions,
	}

	log.Printf("[TIMING] GenerateRNTableData TOTAL: %v", time.Since(overallStart))
	return tableData, nil
}

// validateRNTableRequest validates the input request
func (s *RNCreationServiceImpl) validateRNTableRequest(request *types.RNTableRequest) error {
	if request == nil {
		return fmt.Errorf("request cannot be nil")
	}
	if request.CustomizationJobURL == "" {
		return fmt.Errorf("customization job URL is required")
	}
	return nil
}

// NOTE: Parallel execution functions removed to prevent kubectl config corruption
// Sequential execution is safer and more reliable for kubectl operations

// populateCorePatchChartsColumn fetches and formats data for the Core Patch/Charts column
func (s *RNCreationServiceImpl) populateCorePatchChartsColumn(ctx context.Context, clusterName string) (string, error) {
	corePatchInfo, err := s.GetCorePatchCharts(ctx, clusterName)
	if err != nil {
		log.Printf("ERROR: Failed to get core patch charts for cluster %s: %v", clusterName, err)
		// Return a user-friendly error message instead of failing completely
		return fmt.Sprintf("[Error connecting to cluster '%s' - please check cluster connectivity]", clusterName), nil
	}

	formattedCharts := s.formatCorePatchCharts(corePatchInfo)
	if formattedCharts == "[Charts info not available]" {
		// This means the script ran but returned no data
		return fmt.Sprintf("[No helm charts found in cluster '%s']", clusterName), nil
	}

	return formattedCharts, nil
}

// populateCommentsInstructionsColumn fetches and formats data for the Comments/Instructions column
// NOTE: This function now expects TLC version and image versions to be passed in,
// since they're fetched sequentially in the main function to prevent kubectl config corruption
func (s *RNCreationServiceImpl) formatCommentsInstructionsFromData(tlcVersion, clusterName, oniImage, attImage, guidedTaskImage, customizationImage, storageJobURL string) string {
	return s.formatCommentsInstructions(tlcVersion, clusterName, oniImage, attImage, guidedTaskImage, customizationImage, storageJobURL)
}

// populateCustomOrchestrationZipColumn formats data for the Custom Orchestration Zip column
func (s *RNCreationServiceImpl) populateCustomOrchestrationZipColumn(customOrchZipURL string) string {
	return s.formatCustomOrchZip(customOrchZipURL)
}

// formatCustomOrchZip formats the custom orchestration ZIP URL for display
func (s *RNCreationServiceImpl) formatCustomOrchZip(customOrchZipURL string) string {
	if customOrchZipURL == "" {
		return "[To be populated]"
	}
	return fmt.Sprintf("Custom Orchestration ZIP: %s", customOrchZipURL)
}

// formatCorePatchCharts formats the core patch charts information for display
func (s *RNCreationServiceImpl) formatCorePatchCharts(corePatchInfo []types.CorePatchInfo) string {
	if len(corePatchInfo) == 0 {
		return "[Charts info not available]"
	}

	var formatted []string
	for _, info := range corePatchInfo {
		if len(info.Charts) > 0 {
			// Format namespace header
			namespaceHeader := fmt.Sprintf("%s namespace:", info.Namespace)
			formatted = append(formatted, namespaceHeader)

			// Add each chart on its own line
			for _, chart := range info.Charts {
				formatted = append(formatted, chart)
			}

			// Add empty line between namespaces
			formatted = append(formatted, "")
		}
	}

	if len(formatted) == 0 {
		return "[No charts found]"
	}

	// Join with newlines and remove trailing empty line
	result := strings.Join(formatted, "\n")
	return strings.TrimSuffix(result, "\n")
}

// GetImageVersions executes kubectl commands on EKS cluster to get image versions
func (s *RNCreationServiceImpl) GetImageVersions(ctx context.Context, clusterName string) (string, string, string, error) {
	startTime := time.Now()
	log.Printf("[TIMING] GetImageVersions started for cluster: %s", clusterName)

	if clusterName == "" {
		return "", "", "", fmt.Errorf("cluster name cannot be empty")
	}

	// Step 1: Create temporary script with proper setup
	step1Start := time.Now()
	tempDir, scriptPath, err := s.createImageVersionTempScript()
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create temp script: %w", err)
	}
	defer os.RemoveAll(tempDir)
	step1Duration := time.Since(step1Start)
	log.Printf("[TIMING] Step 1 - Create image version temp script: %v", step1Duration)

	// Step 2: Execute the script
	step2Start := time.Now()
	output, err := s.executeImageVersionScript(ctx, scriptPath, clusterName, tempDir)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get image versions for cluster '%s': %w", clusterName, err)
	}
	step2Duration := time.Since(step2Start)
	log.Printf("[TIMING] Step 2 - Execute image version script: %v", step2Duration)

	// Step 3: Parse the output
	step3Start := time.Now()
	attImage, guidedTaskImage, customizationImage := s.parseImageVersionOutput(output)
	step3Duration := time.Since(step3Start)
	log.Printf("[TIMING] Step 3 - Parse image version output: %v", step3Duration)

	totalDuration := time.Since(startTime)
	log.Printf("[TIMING] GetImageVersions TOTAL: %v", totalDuration)

	return attImage, guidedTaskImage, customizationImage, nil
}

// createImageVersionTempScript creates a temporary script for image version retrieval
func (s *RNCreationServiceImpl) createImageVersionTempScript() (string, string, error) {
	return s.createTempScriptFromEmbedded(
		imageVersionsScriptName,
		"image-versions-script",
	)
}

// executeImageVersionScript executes the image version script
func (s *RNCreationServiceImpl) executeImageVersionScript(ctx context.Context, scriptPath, clusterName, workingDir string) (string, error) {
	command := fmt.Sprintf("%s %s", scriptPath, clusterName)
	return s.executeCommandWithOutputInDir(ctx, command, workingDir)
}

// parseImageVersionOutput parses the output from image version script
func (s *RNCreationServiceImpl) parseImageVersionOutput(output string) (string, string, string) {
	var attImage, guidedTaskImage, customizationImage string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "ATT image: ") {
			attImage = strings.TrimPrefix(line, "ATT image: ")
		} else if strings.HasPrefix(line, "Guided task image: ") {
			guidedTaskImage = strings.TrimPrefix(line, "Guided task image: ")
		} else if strings.HasPrefix(line, "Customization image: ") {
			customizationImage = strings.TrimPrefix(line, "Customization image: ")
		}
	}

	return attImage, guidedTaskImage, customizationImage
}

// formatCommentsInstructions formats the comments/instructions field with TLC version, EKS cluster name, image versions, and storage job URL
func (s *RNCreationServiceImpl) formatCommentsInstructions(tlcVersion, clusterName, oniImage, attImage, guidedTaskImage, customizationImage, storageJobURL string) string {
	result := fmt.Sprintf("TLC Version = %s\neks_clustername = %s", tlcVersion, clusterName)
	if oniImage != "" {
		result += fmt.Sprintf("\nONI image: %s", oniImage)
	}
	if attImage != "" {
		result += fmt.Sprintf("\nATT image: %s", attImage)
	}
	if guidedTaskImage != "" {
		result += fmt.Sprintf("\nGuided task image: %s", guidedTaskImage)
	}
	if customizationImage != "" {
		result += fmt.Sprintf("\nCustomization image: %s", customizationImage)
	}
	if storageJobURL != "" {
		result += fmt.Sprintf("\nStorage: %s", storageJobURL)
	}
	return result
}
