package types

import (
	"time"
)

// JobStatus represents the status of a Jenkins job
type JobStatus struct {
	Number      int           `json:"number"`
	Status      string        `json:"status"` // "queued", "running", "success", "failed", "aborted", "unstable"
	URL         string        `json:"url"`
	Duration    time.Duration `json:"duration"`
	Description string        `json:"description"`
	QueueURL    string        `json:"queue_url,omitempty"` // URL for queued jobs
	StartTime   *time.Time    `json:"start_time,omitempty"`
	EndTime     *time.Time    `json:"end_time,omitempty"`
	Result      string        `json:"result,omitempty"` // Jenkins result: SUCCESS, FAILURE, UNSTABLE, etc.
	Building    bool          `json:"building"`
}

// DeployedArtifact represents a deployed artifact from Jenkins
type DeployedArtifact struct {
	Name         string            `json:"name"`
	URL          string            `json:"url"`
	Type         string            `json:"type"`
	Size         int64             `json:"size,omitempty"`
	LastModified *time.Time        `json:"last_modified,omitempty"`
	Checksum     string            `json:"checksum,omitempty"`
	Repository   string            `json:"repository,omitempty"`
	Path         string            `json:"path,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ScaleRequest represents a request to scale an EKS cluster
type ScaleRequest struct {
	ClusterName string            `json:"cluster_name"`
	ScaleType   string            `json:"scale_type"` // "up" or "down"
	Account     string            `json:"account"`
	Options     map[string]string `json:"options,omitempty"` // Additional scaling options
}

// ScaleResponse represents the response from a scaling operation
type ScaleResponse struct {
	JobStatus *JobStatus        `json:"job_status"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ArtifactExtractionRequest represents a request to extract artifacts
type ArtifactExtractionRequest struct {
	BuildURL    string            `json:"build_url"`
	FilterTypes []string          `json:"filter_types,omitempty"` // Filter by artifact types
	Options     map[string]string `json:"options,omitempty"`
}

// ArtifactExtractionResponse represents the response from artifact extraction
type ArtifactExtractionResponse struct {
	Artifacts   []DeployedArtifact `json:"artifacts"`
	TotalCount  int                `json:"total_count"`
	BuildURL    string             `json:"build_url"`
	ExtractedAt time.Time          `json:"extracted_at"`
	Metadata    map[string]string  `json:"metadata,omitempty"`
}

// AuthCredentials represents Jenkins authentication credentials
type AuthCredentials struct {
	Username string `json:"username"`
	Token    string `json:"token"`
	BaseURL  string `json:"base_url"`
}

// ClientOptions represents options for Jenkins client configuration
type ClientOptions struct {
	Timeout       time.Duration     `json:"timeout"`
	RetryAttempts int               `json:"retry_attempts"`
	RetryDelay    time.Duration     `json:"retry_delay"`
	UserAgent     string            `json:"user_agent"`
	Headers       map[string]string `json:"headers,omitempty"`
}

// RequestContext represents context for Jenkins requests
type RequestContext struct {
	RequestID string            `json:"request_id"`
	UserID    string            `json:"user_id,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// JobParameter represents a parameter for a Jenkins job
type JobParameter struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

// QueueItem represents an item in Jenkins build queue
type QueueItem struct {
	ID         int        `json:"id"`
	URL        string     `json:"url"`
	Why        string     `json:"why"` // Reason for being queued
	Cancelled  bool       `json:"cancelled"`
	Executable *JobStatus `json:"executable,omitempty"` // Set when job starts executing
	Task       struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"task"`
	InQueueSince time.Time `json:"in_queue_since"`
}

// BuildInfo represents information about a Jenkins build
type BuildInfo struct {
	Number          int                    `json:"number"`
	URL             string                 `json:"url"`
	Result          string                 `json:"result"`
	Duration        time.Duration          `json:"duration"`
	Timestamp       time.Time              `json:"timestamp"`
	Building        bool                   `json:"building"`
	Description     string                 `json:"description"`
	DisplayName     string                 `json:"display_name"`
	FullDisplayName string                 `json:"full_display_name"`
	Parameters      []JobParameter         `json:"parameters,omitempty"`
	Artifacts       []DeployedArtifact     `json:"artifacts,omitempty"`
	Changes         []ChangeSet            `json:"changes,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ChangeSet represents a code change in a Jenkins build
type ChangeSet struct {
	CommitID      string    `json:"commit_id"`
	Author        string    `json:"author"`
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"timestamp"`
	AffectedFiles []string  `json:"affected_files,omitempty"`
}

// JobHealth represents the health status of a Jenkins job
type JobHealth struct {
	Score       int    `json:"score"` // 0-100
	Description string `json:"description"`
	IconURL     string `json:"icon_url,omitempty"`
}

// ServiceStatus represents the overall status of a Jenkins service
type ServiceStatus struct {
	Service     string                 `json:"service"`
	Status      string                 `json:"status"` // "healthy", "degraded", "unhealthy"
	LastChecked time.Time              `json:"last_checked"`
	Details     string                 `json:"details,omitempty"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
}

// ValidationResult represents the result of parameter validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// GetStatusLevel returns a numeric level for status comparison
func (js *JobStatus) GetStatusLevel() int {
	switch js.Status {
	case "success":
		return 5
	case "unstable":
		return 4
	case "running":
		return 3
	case "queued":
		return 2
	case "failed", "aborted":
		return 1
	default:
		return 0
	}
}

// IsComplete returns true if the job has finished execution
func (js *JobStatus) IsComplete() bool {
	return js.Status == "success" || js.Status == "failed" || js.Status == "aborted" || js.Status == "unstable"
}

// IsSuccessful returns true if the job completed successfully
func (js *JobStatus) IsSuccessful() bool {
	return js.Status == "success"
}

// IsFailed returns true if the job failed
func (js *JobStatus) IsFailed() bool {
	return js.Status == "failed" || js.Status == "aborted"
}

// IsRunning returns true if the job is currently running
func (js *JobStatus) IsRunning() bool {
	return js.Status == "running" || js.Building
}

// FilterArtifactsByType filters artifacts by their type
func (aer *ArtifactExtractionResponse) FilterArtifactsByType(types []string) []DeployedArtifact {
	if len(types) == 0 {
		return aer.Artifacts
	}

	typeMap := make(map[string]bool)
	for _, t := range types {
		typeMap[t] = true
	}

	var filtered []DeployedArtifact
	for _, artifact := range aer.Artifacts {
		if typeMap[artifact.Type] {
			filtered = append(filtered, artifact)
		}
	}

	return filtered
}

// GetArtifactsByRepository groups artifacts by repository
func (aer *ArtifactExtractionResponse) GetArtifactsByRepository() map[string][]DeployedArtifact {
	result := make(map[string][]DeployedArtifact)
	for _, artifact := range aer.Artifacts {
		repo := artifact.Repository
		if repo == "" {
			repo = "unknown"
		}
		result[repo] = append(result[repo], artifact)
	}
	return result
}

// RNCreationRequest represents a request to create release notes via storage job
type RNCreationRequest struct {
	Branch              string            `json:"branch"`
	Product             string            `json:"product"`
	CoreVersion         string            `json:"core_version"`
	EnvLogin            string            `json:"env_login"`
	BuildChartVersion   string            `json:"build_chart_version"`
	BranchName          string            `json:"branch_name"`
	CustomOrchZipURL    string            `json:"custom_orch_zip_url"`
	OniImage            string            `json:"oni_image"`
	Email               string            `json:"email"`
	Layering            string            `json:"layering"`
	CustomizationJobURL string            `json:"customization_job_url,omitempty"`
	Options             map[string]string `json:"options,omitempty"`
}

// RNCreationResponse represents the response from storage job trigger
type RNCreationResponse struct {
	JobStatus *JobStatus        `json:"job_status"`
	Message   string            `json:"message"`
	JobURL    string            `json:"job_url,omitempty"`
	RequestID string            `json:"request_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// CustomizationJob represents a Jenkins customization job
type CustomizationJob struct {
	Number     int                `json:"number"`
	URL        string             `json:"url"`
	Status     string             `json:"status"`
	Result     string             `json:"result"`
	Timestamp  time.Time          `json:"timestamp"`
	Branch     string             `json:"branch"`
	TLCVersion string             `json:"tlc_version,omitempty"`
	Artifacts  []DeployedArtifact `json:"artifacts,omitempty"`
	Building   bool               `json:"building"`
}

// BitbucketCommit represents a commit from Bitbucket API
type BitbucketCommit struct {
	ID        string    `json:"id"`
	DisplayID string    `json:"displayId"`
	Message   string    `json:"message"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
}

// JenkinsBuildInfo represents the raw structure from Jenkins build API
type JenkinsBuildInfo struct {
	Description string `json:"description"`
	Actions     []struct {
		Class      string `json:"_class"`
		Parameters []struct {
			Class string      `json:"_class"`
			Name  string      `json:"name"`
			Value interface{} `json:"value"`
		} `json:"parameters"`
	} `json:"actions"`
}

// CorePatchInfo represents information about core patches/charts
type CorePatchInfo struct {
	Namespace string   `json:"namespace"`
	Charts    []string `json:"charts"`
}

// RNTableRequest represents the input parameters for RN table data generation
type RNTableRequest struct {
	CustomizationJobURL string `json:"customization_job_url"`
	CustomOrchZipURL    string `json:"custom_orch_zip_url"`
	OniImage            string `json:"oni_image"`
	AttImage            string `json:"att_image"`
	GuidedTaskImage     string `json:"guided_task_image"`
	CustomizationImage  string `json:"customization_image"`
	StorageJobURL       string `json:"storage_job_url"`
}

// RNTableData represents the data structure for RN table population
type RNTableData struct {
	Application            string `json:"application"`
	DefectNumber           string `json:"defect_number"`
	CorePatchCharts        string `json:"core_patch_charts"`
	CustomOrchestrationZip string `json:"custom_orchestration_zip"`
	CommitID               string `json:"commit_id"`
	CommentsInstructions   string `json:"comments_instructions"`
}
