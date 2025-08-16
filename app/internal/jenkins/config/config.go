package config

import (
	"embed"
	"encoding/json"
	"fmt"
	"time"
)

//go:embed jobs.json
var configFS embed.FS

// JobsConfig represents the entire Jenkins jobs configuration
type JobsConfig struct {
	Jobs          map[string]JobConfig       `json:"jobs"`
	Artifacts     ArtifactsConfig            `json:"artifacts"`
	Global        GlobalConfig               `json:"global"`
	ErrorHandling ErrorHandlingConfig        `json:"error_handling"`
}

// JobConfig represents configuration for a specific Jenkins job
type JobConfig struct {
	Name            string                    `json:"name"`
	Description     string                    `json:"description"`
	JobPath         string                    `json:"job_path"`
	Method          string                    `json:"method"`
	EndpointSuffix  string                    `json:"endpoint_suffix"`
	TimeoutSeconds  int                       `json:"timeout_seconds"`
	Parameters      map[string]ParameterConfig `json:"parameters"`
	Responses       ResponseConfig            `json:"responses"`
}

// ParameterConfig represents configuration for a job parameter
type ParameterConfig struct {
	Type         string   `json:"type"`
	Required     bool     `json:"required"`
	Description  string   `json:"description"`
	AllowedValues []string `json:"allowed_values,omitempty"`
	Default      string   `json:"default,omitempty"`
}

// ResponseConfig represents expected response configurations
type ResponseConfig struct {
	QueueSuccessStatus  int `json:"queue_success_status"`
	StatusSuccessStatus int `json:"status_success_status"`
}

// ArtifactsConfig represents configuration for artifact extraction
type ArtifactsConfig struct {
	Parsing ParsingConfig `json:"parsing"`
}

// ParsingConfig represents HTML parsing configuration
type ParsingConfig struct {
	TimeoutSeconds   int               `json:"timeout_seconds"`
	RegexPatterns    RegexPatterns     `json:"regex_patterns"`
	SupportedTypes   []string          `json:"supported_types"`
}

// RegexPatterns contains regex patterns for parsing
type RegexPatterns struct {
	DeployedArtifactsSection string `json:"deployed_artifacts_section"`
	ArtifactItem            string `json:"artifact_item"`
}

// GlobalConfig represents global Jenkins configuration
type GlobalConfig struct {
	DefaultTimeoutSeconds int    `json:"default_timeout_seconds"`
	RetryAttempts        int    `json:"retry_attempts"`
	RetryDelaySeconds    int    `json:"retry_delay_seconds"`
	UserAgent           string `json:"user_agent"`
}

// ErrorHandlingConfig represents error handling configuration
type ErrorHandlingConfig struct {
	LogSensitiveData   bool              `json:"log_sensitive_data"`
	IncludeStackTrace  bool              `json:"include_stack_trace"`
	CustomErrorCodes   map[string]string `json:"custom_error_codes"`
}

var globalConfig *JobsConfig

// LoadConfig loads the Jenkins jobs configuration from the embedded JSON file
func LoadConfig() (*JobsConfig, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}

	data, err := configFS.ReadFile("jobs.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read jobs.json: %w", err)
	}

	var config JobsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse jobs.json: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	globalConfig = &config
	return globalConfig, nil
}

// GetJobConfig returns configuration for a specific job
func (c *JobsConfig) GetJobConfig(jobName string) (*JobConfig, error) {
	job, exists := c.Jobs[jobName]
	if !exists {
		return nil, fmt.Errorf("job '%s' not found in configuration", jobName)
	}
	return &job, nil
}

// GetJobTimeout returns the timeout for a specific job
func (c *JobsConfig) GetJobTimeout(jobName string) time.Duration {
	if job, exists := c.Jobs[jobName]; exists && job.TimeoutSeconds > 0 {
		return time.Duration(job.TimeoutSeconds) * time.Second
	}
	return time.Duration(c.Global.DefaultTimeoutSeconds) * time.Second
}

// GetJobURL constructs the full Jenkins job URL
func (c *JobsConfig) GetJobURL(baseURL, jobName string) (string, error) {
	job, err := c.GetJobConfig(jobName)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/%s", baseURL, job.JobPath)
	if job.EndpointSuffix != "" {
		url = fmt.Sprintf("%s/%s", url, job.EndpointSuffix)
	}
	
	return url, nil
}

// ValidateJobParameters validates parameters against job configuration
func (c *JobsConfig) ValidateJobParameters(jobName string, params map[string]string) error {
	job, err := c.GetJobConfig(jobName)
	if err != nil {
		return err
	}

	// Check required parameters
	for paramName, paramConfig := range job.Parameters {
		value, provided := params[paramName]
		
		if paramConfig.Required && (!provided || value == "") {
			return fmt.Errorf("required parameter '%s' is missing", paramName)
		}

		if provided && len(paramConfig.AllowedValues) > 0 {
			valid := false
			for _, allowedValue := range paramConfig.AllowedValues {
				if value == allowedValue {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("parameter '%s' value '%s' is not in allowed values: %v", 
					paramName, value, paramConfig.AllowedValues)
			}
		}
	}

	return nil
}

// GetErrorCode returns the custom error code for a given error type
func (c *JobsConfig) GetErrorCode(errorType string) string {
	if code, exists := c.ErrorHandling.CustomErrorCodes[errorType]; exists {
		return code
	}
	return "JENKINS_UNKNOWN_ERROR"
}

// validateConfig validates the loaded configuration
func validateConfig(config *JobsConfig) error {
	if config.Jobs == nil || len(config.Jobs) == 0 {
		return fmt.Errorf("no jobs defined in configuration")
	}

	for jobName, job := range config.Jobs {
		if job.JobPath == "" {
			return fmt.Errorf("job '%s' has empty job_path", jobName)
		}
		if job.Method == "" {
			job.Method = "GET" // Default to GET
		}
		if job.TimeoutSeconds <= 0 {
			job.TimeoutSeconds = config.Global.DefaultTimeoutSeconds
		}
	}

	if config.Global.DefaultTimeoutSeconds <= 0 {
		config.Global.DefaultTimeoutSeconds = 30
	}

	if config.Artifacts.Parsing.RegexPatterns.DeployedArtifactsSection == "" {
		return fmt.Errorf("deployed_artifacts_section regex pattern is required")
	}

	if config.Artifacts.Parsing.RegexPatterns.ArtifactItem == "" {
		return fmt.Errorf("artifact_item regex pattern is required")
	}

	return nil
}