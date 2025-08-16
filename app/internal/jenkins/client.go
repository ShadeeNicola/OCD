package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"app/internal/config"
	jenkinsconfig "app/internal/jenkins/config"
	"app/internal/jenkins/errors"
	"app/internal/jenkins/types"
)

// Client represents a Jenkins HTTP client with authentication
type Client struct {
	baseURL     string
	username    string
	token       string
	httpClient  *http.Client
	config      *jenkinsconfig.JobsConfig
	options     *types.ClientOptions
}

// ClientConfig represents configuration for creating a Jenkins client
type ClientConfig struct {
	URL       string
	Username  string
	Token     string
	Options   *types.ClientOptions
}

// NewClient creates a new Jenkins client from app config
func NewClient(cfg config.JenkinsConfig) (*Client, error) {
	return NewClientWithConfig(ClientConfig{
		URL:      cfg.URL,
		Username: cfg.Username,
		Token:    cfg.Token,
	})
}

// NewClientWithConfig creates a new Jenkins client with detailed configuration
func NewClientWithConfig(cfg ClientConfig) (*Client, error) {
	// Load Jenkins jobs configuration
	jobsConfig, err := jenkinsconfig.LoadConfig()
	if err != nil {
		return nil, errors.NewConfigurationError("failed to load Jenkins configuration", err)
	}

	// Set default client options if not provided
	options := cfg.Options
	if options == nil {
		options = &types.ClientOptions{
			Timeout:       time.Duration(jobsConfig.Global.DefaultTimeoutSeconds) * time.Second,
			RetryAttempts: jobsConfig.Global.RetryAttempts,
			RetryDelay:    time.Duration(jobsConfig.Global.RetryDelaySeconds) * time.Second,
			UserAgent:     jobsConfig.Global.UserAgent,
		}
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: options.Timeout,
	}

	client := &Client{
		baseURL:    strings.TrimSuffix(cfg.URL, "/"),
		username:   cfg.Username,
		token:      cfg.Token,
		httpClient: httpClient,
		config:     jobsConfig,
		options:    options,
	}

	return client, nil
}

// Get performs a GET request without authentication
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	return c.doRequest(ctx, "GET", url, nil, false)
}

// Post performs a POST request without authentication
func (c *Client) Post(ctx context.Context, url string, data map[string]string) ([]byte, error) {
	return c.doRequest(ctx, "POST", url, data, false)
}

// GetWithAuth performs a GET request with authentication
func (c *Client) GetWithAuth(ctx context.Context, url string) ([]byte, error) {
	return c.doRequest(ctx, "GET", url, nil, true)
}

// PostWithAuth performs a POST request with authentication  
func (c *Client) PostWithAuth(ctx context.Context, url string, data map[string]string) ([]byte, error) {
	return c.doRequest(ctx, "POST", url, data, true)
}

// IsConfigured returns true if the client has authentication credentials
func (c *Client) IsConfigured() bool {
	return c.username != "" && c.token != ""
}

// GetBaseURL returns the base Jenkins URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// GetConfig returns the Jenkins jobs configuration
func (c *Client) GetConfig() *jenkinsconfig.JobsConfig {
	return c.config
}

// GetJobURL constructs a full URL for a Jenkins job
func (c *Client) GetJobURL(jobName string) (string, error) {
	return c.config.GetJobURL(c.baseURL, jobName)
}

// ValidateJobParameters validates parameters for a specific job
func (c *Client) ValidateJobParameters(jobName string, params map[string]string) error {
	return c.config.ValidateJobParameters(jobName, params)
}

// doRequest performs the actual HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, method, requestURL string, data map[string]string, useAuth bool) ([]byte, error) {
	var lastErr error

	// Retry logic
	for attempt := 0; attempt <= c.options.RetryAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retrying
			select {
			case <-ctx.Done():
				return nil, errors.NewTimeoutError("request cancelled during retry", ctx.Err())
			case <-time.After(c.options.RetryDelay):
			}
		}

		body, err := c.executeRequest(ctx, method, requestURL, data, useAuth)
		if err == nil {
			return body, nil
		}

		lastErr = err

		// Check if error is retryable
		if !c.isRetryableError(err) {
			break
		}
	}

	return nil, lastErr
}

// executeRequest performs a single HTTP request
func (c *Client) executeRequest(ctx context.Context, method, requestURL string, data map[string]string, useAuth bool) ([]byte, error) {
	// Prepare request body for POST requests
	var requestBody io.Reader
	if method == "POST" && data != nil {
		formData := url.Values{}
		for key, value := range data {
			formData.Set(key, value)
		}
		requestBody = strings.NewReader(formData.Encode())
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if err != nil {
		return nil, errors.NewNetworkError("failed to create HTTP request", 0, err)
	}

	// Set headers
	if method == "POST" && data != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if c.options.UserAgent != "" {
		req.Header.Set("User-Agent", c.options.UserAgent)
	}

	// Add custom headers if any
	for key, value := range c.options.Headers {
		req.Header.Set(key, value)
	}

	// Set authentication if required and configured
	if useAuth {
		if !c.IsConfigured() {
			return nil, errors.NewAuthenticationError("Jenkins credentials not configured", nil)
		}
		req.SetBasicAuth(c.username, c.token)
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Check if it's a timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return nil, errors.NewTimeoutError("request timeout", err)
		}
		return nil, errors.NewNetworkError("HTTP request failed", 0, err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewNetworkError("failed to read response body", resp.StatusCode, err)
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return nil, c.handleHTTPError(resp.StatusCode, string(body), requestURL)
	}

	return body, nil
}

// handleHTTPError creates appropriate errors based on HTTP status codes
func (c *Client) handleHTTPError(statusCode int, body, url string) error {
	switch statusCode {
	case 401, 403:
		return errors.NewAuthenticationError(
			fmt.Sprintf("authentication failed (status: %d)", statusCode), 
			nil,
		)
	case 404:
		return errors.NewJobNotFoundError(
			"", 
			fmt.Sprintf("resource not found: %s (status: %d)", url, statusCode), 
			nil,
		)
	case 408, 504:
		return errors.NewTimeoutError(
			fmt.Sprintf("request timeout (status: %d)", statusCode), 
			nil,
		)
	case 422:
		return errors.NewInvalidParametersError(
			"",
			fmt.Sprintf("invalid parameters (status: %d): %s", statusCode, body),
			nil,
		)
	case 500, 502, 503:
		return errors.NewNetworkError(
			fmt.Sprintf("Jenkins server error (status: %d): %s", statusCode, body),
			statusCode,
			nil,
		)
	default:
		return errors.NewNetworkError(
			fmt.Sprintf("HTTP error (status: %d): %s", statusCode, body),
			statusCode,
			nil,
		)
	}
}

// isRetryableError determines if an error should trigger a retry
func (c *Client) isRetryableError(err error) bool {
	if jenkinsErr, ok := errors.GetJenkinsError(err); ok {
		switch jenkinsErr.Type {
		case "timeout", "network_error":
			// Retry timeout and network errors
			return true
		case "authentication_failed", "invalid_parameters", "parsing_failed":
			// Don't retry authentication, parameter, or parsing errors
			return false
		default:
			// For server errors (5xx), retry
			return jenkinsErr.StatusCode >= 500 && jenkinsErr.StatusCode < 600
		}
	}
	return false
}

// GetJobTimeout returns the configured timeout for a specific job
func (c *Client) GetJobTimeout(jobName string) time.Duration {
	return c.config.GetJobTimeout(jobName)
}

// CreateContextWithTimeout creates a context with job-specific timeout
func (c *Client) CreateContextWithTimeout(ctx context.Context, jobName string) (context.Context, context.CancelFunc) {
	timeout := c.GetJobTimeout(jobName)
	return context.WithTimeout(ctx, timeout)
}

// Health performs a basic health check on the Jenkins instance
func (c *Client) Health(ctx context.Context) error {
	healthURL := c.baseURL + "/api/json"
	_, err := c.GetWithAuth(ctx, healthURL)
	return err
}