package errors

import (
	"errors"
	"fmt"
)

// Error types for Jenkins operations
var (
	ErrAuthenticationFailed = errors.New("jenkins authentication failed")
	ErrJobNotFound         = errors.New("jenkins job not found")
	ErrInvalidParameters   = errors.New("invalid job parameters")
	ErrTimeout             = errors.New("jenkins request timeout")
	ErrParsingFailed       = errors.New("failed to parse jenkins response")
	ErrConfigurationError  = errors.New("jenkins configuration error")
	ErrNetworkError        = errors.New("jenkins network error")
	ErrInvalidURL          = errors.New("invalid jenkins URL")
	ErrJobExecutionFailed  = errors.New("jenkins job execution failed")
)

// JenkinsError represents a Jenkins-specific error with additional context
type JenkinsError struct {
	Type        string      // Error type identifier
	Code        string      // Custom error code from configuration
	Message     string      // Human-readable error message
	Cause       error       // Underlying error
	JobName     string      // Jenkins job name (if applicable)
	BuildURL    string      // Jenkins build URL (if applicable)
	StatusCode  int         // HTTP status code (if applicable)
	Context     interface{} // Additional context data
}

// Error implements the error interface
func (e *JenkinsError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Type, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *JenkinsError) Unwrap() error {
	return e.Cause
}

// Is checks if this error matches a target error
func (e *JenkinsError) Is(target error) bool {
	if target == nil {
		return false
	}
	
	// Check against predefined error types
	switch target {
	case ErrAuthenticationFailed:
		return e.Type == "authentication_failed"
	case ErrJobNotFound:
		return e.Type == "job_not_found"
	case ErrInvalidParameters:
		return e.Type == "invalid_parameters"
	case ErrTimeout:
		return e.Type == "timeout"
	case ErrParsingFailed:
		return e.Type == "parsing_failed"
	case ErrConfigurationError:
		return e.Type == "configuration_error"
	case ErrNetworkError:
		return e.Type == "network_error"
	case ErrInvalidURL:
		return e.Type == "invalid_url"
	case ErrJobExecutionFailed:
		return e.Type == "job_execution_failed"
	default:
		return false
	}
}

// NewJenkinsError creates a new JenkinsError
func NewJenkinsError(errorType, message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
	}
}

// NewAuthenticationError creates an authentication error
func NewAuthenticationError(message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:    "authentication_failed",
		Code:    "JENKINS_AUTH_001",
		Message: message,
		Cause:   cause,
	}
}

// NewJobNotFoundError creates a job not found error
func NewJobNotFoundError(jobName, message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:    "job_not_found",
		Code:    "JENKINS_JOB_002",
		Message: message,
		Cause:   cause,
		JobName: jobName,
	}
}

// NewInvalidParametersError creates an invalid parameters error
func NewInvalidParametersError(jobName, message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:    "invalid_parameters",
		Code:    "JENKINS_PARAM_003",
		Message: message,
		Cause:   cause,
		JobName: jobName,
	}
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:    "timeout",
		Code:    "JENKINS_TIMEOUT_004",
		Message: message,
		Cause:   cause,
	}
}

// NewParsingError creates a parsing error
func NewParsingError(buildURL, message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:     "parsing_failed",
		Code:     "JENKINS_PARSE_005",
		Message:  message,
		Cause:    cause,
		BuildURL: buildURL,
	}
}

// NewConfigurationError creates a configuration error
func NewConfigurationError(message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:    "configuration_error",
		Code:    "JENKINS_CONFIG_006",
		Message: message,
		Cause:   cause,
	}
}

// NewNetworkError creates a network error
func NewNetworkError(message string, statusCode int, cause error) *JenkinsError {
	return &JenkinsError{
		Type:       "network_error",
		Code:       "JENKINS_NETWORK_007",
		Message:    message,
		Cause:      cause,
		StatusCode: statusCode,
	}
}

// NewInvalidURLError creates an invalid URL error
func NewInvalidURLError(url, message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:     "invalid_url",
		Code:     "JENKINS_URL_008",
		Message:  message,
		Cause:    cause,
		BuildURL: url,
	}
}

// NewJobExecutionError creates a job execution error
func NewJobExecutionError(jobName, message string, cause error) *JenkinsError {
	return &JenkinsError{
		Type:    "job_execution_failed",
		Code:    "JENKINS_EXEC_009",
		Message: message,
		Cause:   cause,
		JobName: jobName,
	}
}

// IsAuthenticationError checks if error is an authentication error
func IsAuthenticationError(err error) bool {
	return errors.Is(err, ErrAuthenticationFailed)
}

// IsJobNotFoundError checks if error is a job not found error
func IsJobNotFoundError(err error) bool {
	return errors.Is(err, ErrJobNotFound)
}

// IsTimeoutError checks if error is a timeout error
func IsTimeoutError(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsParsingError checks if error is a parsing error
func IsParsingError(err error) bool {
	return errors.Is(err, ErrParsingFailed)
}

// IsNetworkError checks if error is a network error
func IsNetworkError(err error) bool {
	return errors.Is(err, ErrNetworkError)
}

// GetJenkinsError extracts JenkinsError from error chain
func GetJenkinsError(err error) (*JenkinsError, bool) {
	var jenkinsErr *JenkinsError
	if errors.As(err, &jenkinsErr) {
		return jenkinsErr, true
	}
	return nil, false
}