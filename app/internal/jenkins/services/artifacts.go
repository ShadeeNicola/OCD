package services

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"app/internal/jenkins/errors"
	"app/internal/jenkins/types"
)

// ArtifactsServiceImpl implements the ArtifactsService interface
type ArtifactsServiceImpl struct {
	client JenkinsClient
}

// NewArtifactsService creates a new artifacts service instance
func NewArtifactsService(client JenkinsClient) ArtifactsService {
	return &ArtifactsServiceImpl{
		client: client,
	}
}

// ExtractArtifacts extracts deployed artifacts from a Jenkins build
func (a *ArtifactsServiceImpl) ExtractArtifacts(ctx context.Context, request *types.ArtifactExtractionRequest) (*types.ArtifactExtractionResponse, error) {
	// Validate the request
	validation, err := a.ValidateArtifactRequest(request)
	if err != nil {
		return nil, err
	}
	if !validation.Valid {
		return nil, errors.NewInvalidParametersError(
			"artifacts",
			fmt.Sprintf("validation failed: %v", validation.Errors),
			nil,
		)
	}

	// Normalize the build URL
	buildURL := a.normalizeBuildURL(request.BuildURL)

	// Fetch the build page HTML
	htmlContent, err := a.client.GetWithAuth(ctx, buildURL)
	if err != nil {
		return nil, errors.NewNetworkError(
			fmt.Sprintf("failed to fetch build page: %s", buildURL),
			0,
			err,
		)
	}

	// Parse artifacts from HTML content
	artifacts, err := a.parseArtifactsFromHTML(string(htmlContent))
	if err != nil {
		return nil, errors.NewParsingError(
			buildURL,
			"failed to parse artifacts from HTML",
			err,
		)
	}

	// Filter artifacts by type if specified
	if len(request.FilterTypes) > 0 {
		artifacts = a.filterArtifactsByType(artifacts, request.FilterTypes)
	}

	// Enhance artifacts with additional metadata
	a.enhanceArtifacts(artifacts)

	response := &types.ArtifactExtractionResponse{
		Artifacts:   artifacts,
		TotalCount:  len(artifacts),
		BuildURL:    buildURL,
		ExtractedAt: time.Now().UTC(),
		Metadata: map[string]string{
			"extraction_method": "html_parsing",
			"jenkins_build":     buildURL,
			"timestamp":         time.Now().UTC().Format(time.RFC3339),
		},
	}

	// Add filtering metadata if applicable
	if len(request.FilterTypes) > 0 {
		response.Metadata["filtered_types"] = strings.Join(request.FilterTypes, ",")
	}

	return response, nil
}

// GetBuildInfo retrieves detailed information about a Jenkins build
func (a *ArtifactsServiceImpl) GetBuildInfo(ctx context.Context, buildURL string) (*types.BuildInfo, error) {
	// Normalize URL and add API suffix
	apiURL := a.normalizeBuildURL(buildURL)
	if !strings.HasSuffix(apiURL, "/api/json") {
		apiURL = strings.TrimSuffix(apiURL, "/") + "/api/json"
	}

	// Fetch build information from Jenkins API
	responseBody, err := a.client.GetWithAuth(ctx, apiURL)
	if err != nil {
		return nil, errors.NewNetworkError(
			fmt.Sprintf("failed to fetch build info: %s", apiURL),
			0,
			err,
		)
	}

	// Parse the JSON response
	buildInfo, err := a.parseBuildInfoResponse(responseBody)
	if err != nil {
		return nil, errors.NewParsingError(
			apiURL,
			"failed to parse build info response",
			err,
		)
	}

	return buildInfo, nil
}

// ValidateArtifactRequest validates an artifact extraction request
func (a *ArtifactsServiceImpl) ValidateArtifactRequest(request *types.ArtifactExtractionRequest) (*types.ValidationResult, error) {
	result := &types.ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Validate build URL
	if request.BuildURL == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "build_url is required")
	} else {
		// Basic URL validation
		if !strings.Contains(request.BuildURL, "/job/") {
			result.Valid = false
			result.Errors = append(result.Errors, "build_url must be a valid Jenkins build URL containing '/job/'")
		}

		if !strings.Contains(request.BuildURL, "://") {
			result.Valid = false
			result.Errors = append(result.Errors, "build_url must be a complete URL with protocol")
		}
	}

	// Validate filter types if provided
	if len(request.FilterTypes) > 0 {
		supportedTypes := a.GetSupportedArtifactTypes()
		supportedMap := make(map[string]bool)
		for _, t := range supportedTypes {
			supportedMap[t] = true
		}

		for _, filterType := range request.FilterTypes {
			if !supportedMap[filterType] {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("filter type '%s' is not in supported types list", filterType))
			}
		}
	}

	return result, nil
}

// GetSupportedArtifactTypes returns list of supported artifact types
func (a *ArtifactsServiceImpl) GetSupportedArtifactTypes() []string {
	// This would ideally come from configuration
	return []string{
		"jar", "war", "zip", "rpm", "tar.gz", "tar.bz2", "ear", "rar", "7z",
		"exe", "msi", "deb", "pkg", "dmg", "iso", "docker", "helm",
	}
}

// FilterArtifacts filters artifacts based on criteria
func (a *ArtifactsServiceImpl) FilterArtifacts(artifacts []types.DeployedArtifact, criteria map[string]interface{}) []types.DeployedArtifact {
	var filtered []types.DeployedArtifact

	for _, artifact := range artifacts {
		include := true

		// Filter by type
		if types, ok := criteria["types"].([]string); ok && len(types) > 0 {
			typeMatch := false
			for _, t := range types {
				if artifact.Type == t {
					typeMatch = true
					break
				}
			}
			if !typeMatch {
				include = false
			}
		}

		// Filter by name pattern
		if pattern, ok := criteria["name_pattern"].(string); ok && pattern != "" {
			matched, err := regexp.MatchString(pattern, artifact.Name)
			if err != nil || !matched {
				include = false
			}
		}

		// Filter by repository
		if repo, ok := criteria["repository"].(string); ok && repo != "" {
			if artifact.Repository != repo {
				include = false
			}
		}

		// Filter by minimum size
		if minSize, ok := criteria["min_size"].(int64); ok && minSize > 0 {
			if artifact.Size < minSize {
				include = false
			}
		}

		if include {
			filtered = append(filtered, artifact)
		}
	}

	return filtered
}

// Helper methods

// normalizeBuildURL normalizes a Jenkins build URL
func (a *ArtifactsServiceImpl) normalizeBuildURL(buildURL string) string {
	// Remove trailing slashes and API suffixes
	url := strings.TrimSuffix(buildURL, "/")
	url = strings.TrimSuffix(url, "/api/json")

	// Ensure it ends with just a slash for HTML requests
	return url + "/"
}

// parseArtifactsFromHTML parses deployed artifacts from Jenkins HTML content
func (a *ArtifactsServiceImpl) parseArtifactsFromHTML(htmlContent string) ([]types.DeployedArtifact, error) {
	var artifacts []types.DeployedArtifact

	// Regular expressions for parsing - these should come from configuration
	deployedArtifactsRegex := regexp.MustCompile(`(?s)Deployed Artifacts\s*<ul>(.*?)</ul>`)
	artifactRegex := regexp.MustCompile(`<li><a href="([^"]+)">([^<]+)</a>\s*\(type:\s*([^)]+)\)</li>`)

	// Find the deployed artifacts section
	matches := deployedArtifactsRegex.FindStringSubmatch(htmlContent)
	if len(matches) < 2 {
		// No deployed artifacts section found - this is not an error, just means no artifacts
		return artifacts, nil
	}

	artifactListHTML := matches[1]

	// Extract individual artifacts
	artifactMatches := artifactRegex.FindAllStringSubmatch(artifactListHTML, -1)

	for _, match := range artifactMatches {
		if len(match) >= 4 {
			artifact := types.DeployedArtifact{
				URL:  strings.TrimSpace(match[1]),
				Name: strings.TrimSpace(match[2]),
				Type: strings.TrimSpace(match[3]),
			}

			// Extract repository and path information from URL
			a.parseArtifactURL(&artifact)

			artifacts = append(artifacts, artifact)
		}
	}

	return artifacts, nil
}

// parseArtifactURL extracts additional information from artifact URL
func (a *ArtifactsServiceImpl) parseArtifactURL(artifact *types.DeployedArtifact) {
	url := artifact.URL

	// Extract repository information
	if strings.Contains(url, "/repository/") {
		parts := strings.Split(url, "/repository/")
		if len(parts) > 1 {
			repoParts := strings.Split(parts[1], "/")
			if len(repoParts) > 0 {
				artifact.Repository = repoParts[0]
			}

			// Extract path (everything after repository name)
			if len(repoParts) > 1 {
				artifact.Path = strings.Join(repoParts[1:], "/")
			}
		}
	}

	// Add metadata about the artifact source
	if artifact.Metadata == nil {
		artifact.Metadata = make(map[string]string)
	}

	if strings.Contains(url, "nexus") {
		artifact.Metadata["source"] = "nexus"
	} else if strings.Contains(url, "artifactory") {
		artifact.Metadata["source"] = "artifactory"
	}

	artifact.Metadata["url_parsed"] = "true"
}

// enhanceArtifacts adds additional metadata to artifacts
func (a *ArtifactsServiceImpl) enhanceArtifacts(artifacts []types.DeployedArtifact) {
	for i := range artifacts {
		artifact := &artifacts[i]

		if artifact.Metadata == nil {
			artifact.Metadata = make(map[string]string)
		}

		// Add classification based on type
		artifact.Metadata["classification"] = a.classifyArtifact(artifact.Type)

		// Add size category if size is available
		if artifact.Size > 0 {
			artifact.Metadata["size_category"] = a.getSizeCategory(artifact.Size)
		}
	}
}

// classifyArtifact classifies an artifact based on its type
func (a *ArtifactsServiceImpl) classifyArtifact(artifactType string) string {
	switch strings.ToLower(artifactType) {
	case "jar", "war", "ear":
		return "java_artifact"
	case "zip", "tar.gz", "tar.bz2", "rar", "7z":
		return "archive"
	case "rpm", "deb", "msi", "pkg":
		return "package"
	case "exe", "dmg":
		return "executable"
	case "docker":
		return "container"
	case "helm":
		return "chart"
	default:
		return "other"
	}
}

// getSizeCategory returns a size category for an artifact
func (a *ArtifactsServiceImpl) getSizeCategory(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size < KB:
		return "tiny"
	case size < MB:
		return "small"
	case size < 10*MB:
		return "medium"
	case size < 100*MB:
		return "large"
	case size < GB:
		return "very_large"
	default:
		return "huge"
	}
}

// filterArtifactsByType filters artifacts by their types
func (a *ArtifactsServiceImpl) filterArtifactsByType(artifacts []types.DeployedArtifact, filterTypes []string) []types.DeployedArtifact {
	typeMap := make(map[string]bool)
	for _, t := range filterTypes {
		typeMap[strings.ToLower(t)] = true
	}

	var filtered []types.DeployedArtifact
	for _, artifact := range artifacts {
		if typeMap[strings.ToLower(artifact.Type)] {
			filtered = append(filtered, artifact)
		}
	}

	return filtered
}

// parseBuildInfoResponse parses build information from Jenkins API response
func (a *ArtifactsServiceImpl) parseBuildInfoResponse(data []byte) (*types.BuildInfo, error) {
	var jenkinsResp struct {
		Number          int               `json:"number"`
		URL             string            `json:"url"`
		Result          string            `json:"result"`
		Duration        int64             `json:"duration"`
		Timestamp       int64             `json:"timestamp"`
		Building        bool              `json:"building"`
		Description     string            `json:"description"`
		DisplayName     string            `json:"displayName"`
		FullDisplayName string            `json:"fullDisplayName"`
		Actions         []json.RawMessage `json:"actions"`
		ChangeSets      []json.RawMessage `json:"changeSets"`
	}

	if err := json.Unmarshal(data, &jenkinsResp); err != nil {
		return nil, err
	}

	buildInfo := &types.BuildInfo{
		Number:          jenkinsResp.Number,
		URL:             jenkinsResp.URL,
		Result:          jenkinsResp.Result,
		Duration:        time.Duration(jenkinsResp.Duration) * time.Millisecond,
		Building:        jenkinsResp.Building,
		Description:     jenkinsResp.Description,
		DisplayName:     jenkinsResp.DisplayName,
		FullDisplayName: jenkinsResp.FullDisplayName,
		Metadata:        make(map[string]interface{}),
	}

	if jenkinsResp.Timestamp > 0 {
		buildInfo.Timestamp = time.Unix(jenkinsResp.Timestamp/1000, 0)
	}

	// Parse parameters from actions (if available)
	buildInfo.Parameters = a.parseParametersFromActions(jenkinsResp.Actions)

	// Parse changes from changeSets (if available)
	buildInfo.Changes = a.parseChangesFromChangeSets(jenkinsResp.ChangeSets)

	return buildInfo, nil
}

// parseParametersFromActions extracts job parameters from Jenkins actions
func (a *ArtifactsServiceImpl) parseParametersFromActions(actions []json.RawMessage) []types.JobParameter {
	var parameters []types.JobParameter

	for _, action := range actions {
		var actionData struct {
			Class      string `json:"_class"`
			Parameters []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
				Class string `json:"_class"`
			} `json:"parameters"`
		}

		if err := json.Unmarshal(action, &actionData); err != nil {
			continue
		}

		if actionData.Class == "hudson.model.ParametersAction" {
			for _, param := range actionData.Parameters {
				parameters = append(parameters, types.JobParameter{
					Name:  param.Name,
					Value: param.Value,
					Type:  a.extractParameterType(param.Class),
				})
			}
		}
	}

	return parameters
}

// parseChangesFromChangeSets extracts code changes from Jenkins changeSets
func (a *ArtifactsServiceImpl) parseChangesFromChangeSets(changeSets []json.RawMessage) []types.ChangeSet {
	var changes []types.ChangeSet

	for _, changeSet := range changeSets {
		var changeSetData struct {
			Items []struct {
				CommitID string `json:"commitId"`
				Author   struct {
					FullName string `json:"fullName"`
				} `json:"author"`
				Comment       string   `json:"comment"`
				Date          string   `json:"date"`
				AffectedPaths []string `json:"affectedPaths"`
				Timestamp     int64    `json:"timestamp"`
			} `json:"items"`
		}

		if err := json.Unmarshal(changeSet, &changeSetData); err != nil {
			continue
		}

		for _, item := range changeSetData.Items {
			change := types.ChangeSet{
				CommitID:      item.CommitID,
				Author:        item.Author.FullName,
				Message:       item.Comment,
				AffectedFiles: item.AffectedPaths,
			}

			if item.Timestamp > 0 {
				change.Timestamp = time.Unix(item.Timestamp/1000, 0)
			}

			changes = append(changes, change)
		}
	}

	return changes
}

// extractParameterType extracts parameter type from Jenkins class name
func (a *ArtifactsServiceImpl) extractParameterType(className string) string {
	switch className {
	case "hudson.model.StringParameterValue":
		return "string"
	case "hudson.model.BooleanParameterValue":
		return "boolean"
	case "hudson.model.PasswordParameterValue":
		return "password"
	case "hudson.model.ChoiceParameterValue":
		return "choice"
	default:
		return "unknown"
	}
}
