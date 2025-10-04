package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"app/internal/config"
	"app/internal/jenkins/errors"
	"app/internal/jenkins/types"
)

const scalingJobBuildSuffix = "/buildWithParameters"

// ScalingServiceImpl implements the ScalingService interface
type ScalingServiceImpl struct {
	configuration *config.Config
	client        JenkinsClient
}

// NewScalingService creates a new scaling service instance
func NewScalingService(configuration *config.Config, client JenkinsClient) ScalingService {
	return &ScalingServiceImpl{
		configuration: configuration,
		client:        client,
	}
}

// TriggerScale initiates a scaling operation for an EKS cluster
func (s *ScalingServiceImpl) TriggerScale(ctx context.Context, request *types.ScaleRequest) (*types.ScaleResponse, error) {
	// Validate the request
	validation, err := s.ValidateScaleRequest(request)
	if err != nil {
		return nil, err
	}
	if !validation.Valid {
		return nil, errors.NewInvalidParametersError(
			"scaling",
			fmt.Sprintf("validation failed: %v", validation.Errors),
			nil,
		)
	}

	// Get the job URL from configuration
	jobURL, err := s.getScalingJobURL()
	if err != nil {
		return nil, errors.NewConfigurationError("failed to get scaling job URL", err)
	}

	// Prepare parameters
	params := map[string]string{
		"eks_clustername": request.ClusterName,
		"scale_type":      request.ScaleType,
		"account":         request.Account,
	}

	// Add any additional options
	for key, value := range request.Options {
		params[key] = value
	}

	// Execute the scaling job
	responseBody, err := s.client.PostWithAuth(ctx, jobURL, params)
	if err != nil {
		return nil, errors.NewJobExecutionError(
			"scaling",
			"failed to trigger scaling job",
			err,
		)
	}

	// For Jenkins, POST to buildWithParameters returns the queue URL in Location header
	// We need to handle this differently since we don't have direct access to headers here
	// Let's assume the job was queued successfully and create a response

	// Get the base job URL for linking
	baseJobURL, _ := s.getScalingJobURL()
	baseJobURL = strings.TrimSuffix(baseJobURL, scalingJobBuildSuffix)

	response := &types.ScaleResponse{
		JobStatus: &types.JobStatus{
			Status:      "queued",
			URL:         baseJobURL, // Set the job URL so the frontend can show the Jenkins link
			Description: fmt.Sprintf("Scaling %s cluster %s", request.ScaleType, request.ClusterName),
		},
		Message:   "Scaling job triggered successfully",
		RequestID: generateRequestID(),
		Metadata: map[string]string{
			"cluster_name": request.ClusterName,
			"scale_type":   request.ScaleType,
			"account":      request.Account,
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		},
	}

	// If we have response data, try to parse it for additional information
	if len(responseBody) > 0 {
		// Jenkins might return queue information in some cases
		s.parseQueueResponse(responseBody, response)
	}

	return response, nil
}

// GetScaleJobStatus retrieves the status of a scaling job
func (s *ScalingServiceImpl) GetScaleJobStatus(ctx context.Context, jobNumber int) (*types.JobStatus, error) {
	if jobNumber <= 0 {
		return nil, errors.NewInvalidParametersError(
			"scaling",
			fmt.Sprintf("invalid job number: %d", jobNumber),
			nil,
		)
	}

	// Construct the API URL for the specific job
	apiURL := fmt.Sprintf("%s/%d/api/json", s.scalingJobBaseURL(), jobNumber)

	// Get job status from Jenkins API
	responseBody, err := s.client.GetWithAuth(ctx, apiURL)
	if err != nil {
		return nil, errors.NewJobNotFoundError(
			"scaling",
			fmt.Sprintf("failed to get status for job #%d", jobNumber),
			err,
		)
	}

	// Parse the JSON response
	jobStatus, err := s.parseJobStatusResponse(responseBody)
	if err != nil {
		return nil, errors.NewParsingError(
			apiURL,
			"failed to parse job status response",
			err,
		)
	}

	return jobStatus, nil
}

// GetQueueStatus retrieves the status of a queued scaling job
func (s *ScalingServiceImpl) GetQueueStatus(ctx context.Context, queueURL string) (*types.JobStatus, error) {
	if queueURL == "" {
		return nil, errors.NewInvalidParametersError(
			"scaling",
			"empty queue URL provided",
			nil,
		)
	}

	// Ensure the queue URL has the API suffix
	apiURL := queueURL
	if !strings.HasSuffix(apiURL, "/api/json") {
		apiURL = strings.TrimSuffix(apiURL, "/") + "/api/json"
	}

	// Get queue status from Jenkins
	responseBody, err := s.client.GetWithAuth(ctx, apiURL)
	if err != nil {
		return nil, errors.NewJobNotFoundError(
			"scaling",
			"failed to get queue status",
			err,
		)
	}

	// Parse the queue response
	queueStatus, err := s.parseQueueStatusResponse(responseBody)
	if err != nil {
		return nil, errors.NewParsingError(
			apiURL,
			"failed to parse queue status response",
			err,
		)
	}

	return queueStatus, nil
}

// ValidateScaleRequest validates a scaling request parameters
func (s *ScalingServiceImpl) ValidateScaleRequest(request *types.ScaleRequest) (*types.ValidationResult, error) {
	result := &types.ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Validate required fields
	if request.ClusterName == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "cluster_name is required")
	}

	if request.ScaleType == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "scale_type is required")
	} else if request.ScaleType != "up" && request.ScaleType != "down" {
		result.Valid = false
		result.Errors = append(result.Errors, "scale_type must be 'up' or 'down'")
	}

	if request.Account == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "account is required")
	}

	// Validate cluster name format (basic validation)
	if request.ClusterName != "" {
		if len(request.ClusterName) < 3 {
			result.Valid = false
			result.Errors = append(result.Errors, "cluster_name is too short (minimum 3 characters)")
		}
		if len(request.ClusterName) > 100 {
			result.Valid = false
			result.Errors = append(result.Errors, "cluster_name is too long (maximum 100 characters)")
		}
	}

	// Add warnings for common issues
	if request.Account != "ATT" {
		result.Warnings = append(result.Warnings, "account is not 'ATT', make sure this is correct")
	}

	return result, nil
}

// GetSupportedClusters returns list of clusters that can be scaled
func (s *ScalingServiceImpl) GetSupportedClusters(ctx context.Context) ([]string, error) {
	// This would typically fetch from Jenkins job configuration or a separate API
	// For now, return a placeholder implementation
	return []string{
		"develop-malkz",
		"staging-cluster",
		"prod-cluster",
	}, nil
}

// Helper methods

// getScalingJobURL constructs the URL for the scaling job
func (s *ScalingServiceImpl) getScalingJobURL() (string, error) {
	return s.scalingJobBaseURL() + scalingJobBuildSuffix, nil
}

func (s *ScalingServiceImpl) scalingJobBaseURL() string {
	base := strings.TrimSuffix(s.client.GetBaseURL(), "/")
	return base + config.NormalizeJobPath(s.scalingJobPath())
}

func (s *ScalingServiceImpl) scalingJobPath() string {
	if s.configuration != nil {
		path := strings.TrimSpace(s.configuration.Endpoints.ScalingJenkinsJobPath)
		if path != "" {
			return path
		}
	}
	return config.DefaultEndpoints().ScalingJenkinsJobPath
}

// parseJobStatusResponse parses the JSON response from Jenkins job status API
func (s *ScalingServiceImpl) parseJobStatusResponse(data []byte) (*types.JobStatus, error) {
	var jenkinsResp struct {
		Number            int    `json:"number"`
		Result            string `json:"result"`
		Building          bool   `json:"building"`
		Duration          int64  `json:"duration"`
		URL               string `json:"url"`
		FullDisplayName   string `json:"fullDisplayName"`
		Timestamp         int64  `json:"timestamp"`
		EstimatedDuration int64  `json:"estimatedDuration"`
	}

	if err := json.Unmarshal(data, &jenkinsResp); err != nil {
		return nil, err
	}

	// Convert Jenkins status to our status
	status := "unknown"
	if jenkinsResp.Building {
		status = "running"
	} else if jenkinsResp.Result == "SUCCESS" {
		status = "success"
	} else if jenkinsResp.Result == "FAILURE" {
		status = "failed"
	} else if jenkinsResp.Result == "ABORTED" {
		status = "aborted"
	} else if jenkinsResp.Result == "UNSTABLE" {
		status = "unstable"
	}

	// Create description
	description := fmt.Sprintf("Job #%d", jenkinsResp.Number)
	if status == "running" {
		description = fmt.Sprintf("Job #%d is running", jenkinsResp.Number)
	} else if status == "success" {
		description = fmt.Sprintf("Job #%d completed successfully", jenkinsResp.Number)
	} else if status == "failed" {
		description = fmt.Sprintf("Job #%d failed", jenkinsResp.Number)
	}

	var startTime, endTime *time.Time
	if jenkinsResp.Timestamp > 0 {
		t := time.Unix(jenkinsResp.Timestamp/1000, 0)
		startTime = &t

		if !jenkinsResp.Building && jenkinsResp.Duration > 0 {
			et := t.Add(time.Duration(jenkinsResp.Duration) * time.Millisecond)
			endTime = &et
		}
	}

	return &types.JobStatus{
		Number:      jenkinsResp.Number,
		Status:      status,
		URL:         jenkinsResp.URL,
		Duration:    time.Duration(jenkinsResp.Duration) * time.Millisecond,
		Description: description,
		StartTime:   startTime,
		EndTime:     endTime,
		Result:      jenkinsResp.Result,
		Building:    jenkinsResp.Building,
	}, nil
}

// parseQueueStatusResponse parses the JSON response from Jenkins queue API
func (s *ScalingServiceImpl) parseQueueStatusResponse(data []byte) (*types.JobStatus, error) {
	var queueItem struct {
		Executable struct {
			Number int    `json:"number"`
			URL    string `json:"url"`
		} `json:"executable"`
		Why       string `json:"why"`
		Cancelled bool   `json:"cancelled"`
		Task      struct {
			Name string `json:"name"`
		} `json:"task"`
		InQueueSince int64 `json:"inQueueSince"`
	}

	if err := json.Unmarshal(data, &queueItem); err != nil {
		return nil, err
	}

	// Check if job was cancelled
	if queueItem.Cancelled {
		return &types.JobStatus{
			Status:      "aborted",
			Description: "Job was cancelled",
		}, nil
	}

	// Check if job has started executing
	if queueItem.Executable.Number > 0 {
		return &types.JobStatus{
			Number:      queueItem.Executable.Number,
			Status:      "running",
			URL:         queueItem.Executable.URL,
			Description: fmt.Sprintf("Job #%d is running", queueItem.Executable.Number),
		}, nil
	}

	// Job is still queued
	reason := queueItem.Why
	if reason == "" {
		reason = "Job is queued"
	}

	var queuedSince *time.Time
	if queueItem.InQueueSince > 0 {
		t := time.Unix(queueItem.InQueueSince/1000, 0)
		queuedSince = &t
	}

	return &types.JobStatus{
		Status:      "queued",
		Description: reason,
		StartTime:   queuedSince,
	}, nil
}

// parseQueueResponse attempts to parse queue information from response
func (s *ScalingServiceImpl) parseQueueResponse(data []byte, response *types.ScaleResponse) {
	// This is a placeholder for parsing queue response
	// In practice, Jenkins returns the queue URL in the Location header
	// which we would need to handle at the HTTP client level
}

// generateRequestID generates a unique request ID
func generateRequestID() string {
	return fmt.Sprintf("scale-%d", time.Now().UnixNano())
}
