package jenkins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"app/internal/config"
)

type Client struct {
	baseURL    string
	username   string
	token      string
	httpClient *http.Client
}

type ScaleRequest struct {
	ClusterName string `json:"cluster_name"`
	ScaleType   string `json:"scale_type"` // "up" or "down"
	Account     string `json:"account"`    // "ATT"
}

type JobStatus struct {
	Number      int    `json:"number"`
	Status      string `json:"status"` // "running", "success", "failed"
	URL         string `json:"url"`
	Duration    int64  `json:"duration"`
	Description string `json:"description"`
}

type ClientConfig struct {
	URL      string
	Username string
	Token    string
}

func NewClient(cfg config.JenkinsConfig) *Client {
	return NewClientWithConfig(ClientConfig{
		URL:      cfg.URL,
		Username: cfg.Username,
		Token:    cfg.Token,
	})
}

func NewClientWithConfig(cfg ClientConfig) *Client {
	return &Client{
		baseURL:  cfg.URL,
		username: cfg.Username,
		token:    cfg.Token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetURL() string {
	return c.baseURL
}

func (c *Client) TriggerScaleJob(req ScaleRequest) (*JobStatus, error) {
	if c.username == "" || c.token == "" {
		return nil, fmt.Errorf("Jenkins credentials not configured")
	}

	// Jenkins job path
	jobPath := "job/Utility/job/OpsUtil/job/scaleUpOrDown2/buildWithParameters"
	jobURL := fmt.Sprintf("%s/%s", c.baseURL, jobPath)

	// Prepare form data
	formData := url.Values{}
	formData.Set("eks_clustername", req.ClusterName)
	formData.Set("scale_type", req.ScaleType)
	formData.Set("account", req.Account)

	// Create request
	httpReq, err := http.NewRequest("POST", jobURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.SetBasicAuth(c.username, c.token)

	// Send request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Jenkins returned status %d", resp.StatusCode)
	}

	// Get queue item location from Location header
	location := resp.Header.Get("Location")
	if location == "" {
		return nil, fmt.Errorf("no Location header in response")
	}

	return &JobStatus{
		Status:      "queued",
		URL:         location, // Keep queue URL initially
		Description: fmt.Sprintf("Scaling %s cluster %s", req.ScaleType, req.ClusterName),
	}, nil
}

func (c *Client) GetJobStatus(jobNumber int) (*JobStatus, error) {
	if jobNumber <= 0 {
		return nil, fmt.Errorf("invalid job number: %d", jobNumber)
	}

	jobURL := fmt.Sprintf("%s/job/Utility/job/OpsUtil/job/scaleUpOrDown2/%d/api/json", c.baseURL, jobNumber)

	req, err := http.NewRequest("GET", jobURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jenkins returned status %d", resp.StatusCode)
	}

	var jenkinsResp struct {
		Number    int    `json:"number"`
		Result    string `json:"result"`
		Building  bool   `json:"building"`
		Duration  int64  `json:"duration"`
		URL       string `json:"url"`
		FullDisplayName string `json:"fullDisplayName"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jenkinsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	status := "unknown"
	if jenkinsResp.Building {
		status = "running"
	} else if jenkinsResp.Result == "SUCCESS" {
		status = "success"
	} else if jenkinsResp.Result == "FAILURE" {
		status = "failed"
	}

	// Create a user-friendly description
	description := fmt.Sprintf("Job #%d", jenkinsResp.Number)
	if status == "running" {
		description = fmt.Sprintf("Job #%d is running", jenkinsResp.Number)
	} else if status == "success" {
		description = fmt.Sprintf("Job #%d completed successfully", jenkinsResp.Number)
	} else if status == "failed" {
		description = fmt.Sprintf("Job #%d failed", jenkinsResp.Number)
	}

	return &JobStatus{
		Number:      jenkinsResp.Number,
		Status:      status,
		URL:         jenkinsResp.URL,
		Duration:    jenkinsResp.Duration,
		Description: description,
	}, nil
}

func (c *Client) GetQueueItemStatus(queueURL string) (*JobStatus, error) {
	if queueURL == "" {
		return nil, fmt.Errorf("empty queue URL")
	}

	// Convert queue URL to API URL
	queueAPIURL := queueURL + "api/json"
	
	req, err := http.NewRequest("GET", queueAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jenkins returned status %d", resp.StatusCode)
	}

	var queueItem struct {
		Executable struct {
			Number int    `json:"number"`
			URL    string `json:"url"`
		} `json:"executable"`
		Why       string `json:"why"`
		Cancelled bool   `json:"cancelled"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&queueItem); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If job is cancelled
	if queueItem.Cancelled {
		return &JobStatus{
			Status:      "failed",
			URL:         queueURL,
			Description: "Job was cancelled",
		}, nil
	}

	// If job is still queued (no executable yet)
	if queueItem.Executable.Number == 0 {
		reason := queueItem.Why
		if reason == "" {
			reason = "Job is queued"
		}
		return &JobStatus{
			Status:      "queued",
			URL:         queueURL,
			Description: reason,
		}, nil
	}

	// Job has started - return the actual job URL
	return &JobStatus{
		Number:      queueItem.Executable.Number,
		Status:      "running",
		URL:         queueItem.Executable.URL,
		Description: fmt.Sprintf("Job #%d is running", queueItem.Executable.Number),
	}, nil
}

func (c *Client) IsConfigured() bool {
	return c.username != "" && c.token != ""
}