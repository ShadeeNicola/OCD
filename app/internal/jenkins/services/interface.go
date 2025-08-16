package services

import (
	"context"

	"app/internal/jenkins/types"
)

// JenkinsClient defines the interface for Jenkins HTTP operations
type JenkinsClient interface {
	// Core HTTP operations
	Get(ctx context.Context, url string) ([]byte, error)
	Post(ctx context.Context, url string, data map[string]string) ([]byte, error)
	GetWithAuth(ctx context.Context, url string) ([]byte, error)
	PostWithAuth(ctx context.Context, url string, data map[string]string) ([]byte, error)
	
	// Authentication
	IsConfigured() bool
	GetBaseURL() string
}

// ScalingService defines the interface for EKS cluster scaling operations
type ScalingService interface {
	// TriggerScale initiates a scaling operation for an EKS cluster
	TriggerScale(ctx context.Context, request *types.ScaleRequest) (*types.ScaleResponse, error)
	
	// GetScaleJobStatus retrieves the status of a scaling job
	GetScaleJobStatus(ctx context.Context, jobNumber int) (*types.JobStatus, error)
	
	// GetQueueStatus retrieves the status of a queued scaling job
	GetQueueStatus(ctx context.Context, queueURL string) (*types.JobStatus, error)
	
	// ValidateScaleRequest validates a scaling request parameters
	ValidateScaleRequest(request *types.ScaleRequest) (*types.ValidationResult, error)
	
	// GetSupportedClusters returns list of clusters that can be scaled (if available)
	GetSupportedClusters(ctx context.Context) ([]string, error)
}

// ArtifactsService defines the interface for Jenkins artifacts operations
type ArtifactsService interface {
	// ExtractArtifacts extracts deployed artifacts from a Jenkins build
	ExtractArtifacts(ctx context.Context, request *types.ArtifactExtractionRequest) (*types.ArtifactExtractionResponse, error)
	
	// GetBuildInfo retrieves detailed information about a Jenkins build
	GetBuildInfo(ctx context.Context, buildURL string) (*types.BuildInfo, error)
	
	// ValidateArtifactRequest validates an artifact extraction request
	ValidateArtifactRequest(request *types.ArtifactExtractionRequest) (*types.ValidationResult, error)
	
	// GetSupportedArtifactTypes returns list of supported artifact types
	GetSupportedArtifactTypes() []string
	
	// FilterArtifacts filters artifacts based on criteria
	FilterArtifacts(artifacts []types.DeployedArtifact, criteria map[string]interface{}) []types.DeployedArtifact
}

// JobService defines the interface for generic Jenkins job operations
type JobService interface {
	// TriggerJob triggers a generic Jenkins job
	TriggerJob(ctx context.Context, jobName string, parameters map[string]string) (*types.JobStatus, error)
	
	// GetJobStatus retrieves the status of any Jenkins job
	GetJobStatus(ctx context.Context, jobURL string) (*types.JobStatus, error)
	
	// GetJobHistory retrieves the build history for a job
	GetJobHistory(ctx context.Context, jobName string, limit int) ([]*types.JobStatus, error)
	
	// CancelJob attempts to cancel a running job
	CancelJob(ctx context.Context, jobURL string) error
	
	// GetJobHealth retrieves health information for a job
	GetJobHealth(ctx context.Context, jobName string) (*types.JobHealth, error)
}

// MonitoringService defines the interface for Jenkins monitoring and health checks
type MonitoringService interface {
	// HealthCheck performs a health check on Jenkins services
	HealthCheck(ctx context.Context) (*types.ServiceStatus, error)
	
	// GetServiceMetrics retrieves metrics about Jenkins service usage
	GetServiceMetrics(ctx context.Context) (map[string]interface{}, error)
	
	// IsJenkinsAvailable checks if Jenkins is available and responsive
	IsJenkinsAvailable(ctx context.Context) (bool, error)
}

// ConfigService defines the interface for configuration management
type ConfigService interface {
	// GetJobConfig retrieves configuration for a specific job
	GetJobConfig(jobName string) (interface{}, error)
	
	// ReloadConfig reloads the Jenkins configuration
	ReloadConfig() error
	
	// ValidateConfig validates the current configuration
	ValidateConfig() error
	
	// GetGlobalConfig retrieves global Jenkins configuration
	GetGlobalConfig() (interface{}, error)
}

// LoggingService defines the interface for Jenkins operation logging
type LoggingService interface {
	// LogOperation logs a Jenkins operation with context
	LogOperation(ctx context.Context, operation string, details map[string]interface{})
	
	// LogError logs a Jenkins error with context
	LogError(ctx context.Context, operation string, err error, details map[string]interface{})
	
	// LogMetric logs a metric about Jenkins operations
	LogMetric(ctx context.Context, metric string, value interface{}, tags map[string]string)
}

// ServiceFactory defines the interface for creating Jenkins services
type ServiceFactory interface {
	// CreateScalingService creates a new scaling service instance
	CreateScalingService(client JenkinsClient) ScalingService
	
	// CreateArtifactsService creates a new artifacts service instance
	CreateArtifactsService(client JenkinsClient) ArtifactsService
	
	// CreateJobService creates a new job service instance
	CreateJobService(client JenkinsClient) JobService
	
	// CreateMonitoringService creates a new monitoring service instance
	CreateMonitoringService(client JenkinsClient) MonitoringService
	
	// CreateConfigService creates a new config service instance
	CreateConfigService() ConfigService
	
	// CreateLoggingService creates a new logging service instance
	CreateLoggingService() LoggingService
}

// ServiceManager defines the interface for managing all Jenkins services
type ServiceManager interface {
	// GetScalingService returns the scaling service
	GetScalingService() ScalingService
	
	// GetArtifactsService returns the artifacts service
	GetArtifactsService() ArtifactsService
	
	// GetJobService returns the job service
	GetJobService() JobService
	
	// GetMonitoringService returns the monitoring service
	GetMonitoringService() MonitoringService
	
	// GetConfigService returns the config service
	GetConfigService() ConfigService
	
	// GetLoggingService returns the logging service
	GetLoggingService() LoggingService
	
	// Shutdown gracefully shuts down all services
	Shutdown(ctx context.Context) error
	
	// HealthCheck performs health checks on all services
	HealthCheck(ctx context.Context) map[string]*types.ServiceStatus
}

// RequestValidator defines the interface for validating Jenkins requests
type RequestValidator interface {
	// ValidateScaleRequest validates scaling requests
	ValidateScaleRequest(request *types.ScaleRequest) (*types.ValidationResult, error)
	
	// ValidateArtifactRequest validates artifact extraction requests
	ValidateArtifactRequest(request *types.ArtifactExtractionRequest) (*types.ValidationResult, error)
	
	// ValidateJobParameters validates job parameters against configuration
	ValidateJobParameters(jobName string, parameters map[string]string) (*types.ValidationResult, error)
	
	// ValidateURL validates Jenkins URLs
	ValidateURL(url string) (*types.ValidationResult, error)
}

// ResponseParser defines the interface for parsing Jenkins responses
type ResponseParser interface {
	// ParseJobStatus parses job status from Jenkins API response
	ParseJobStatus(data []byte) (*types.JobStatus, error)
	
	// ParseQueueItem parses queue item from Jenkins API response
	ParseQueueItem(data []byte) (*types.QueueItem, error)
	
	// ParseBuildInfo parses build information from Jenkins API response
	ParseBuildInfo(data []byte) (*types.BuildInfo, error)
	
	// ParseArtifacts parses artifacts from Jenkins HTML response
	ParseArtifacts(htmlContent string) ([]types.DeployedArtifact, error)
}