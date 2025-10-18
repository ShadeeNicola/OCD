package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port           string
	WSLUser        string
	ScriptName     string
	AllowedOrigins []string
	MaxOutputLines int
	CommandTimeout int // seconds
	Jenkins        JenkinsConfig
	TLS            TLSConfig
	Endpoints      Endpoints
}

type JenkinsConfig struct {
	URL      string
	Username string
	Token    string
}

type TLSConfig struct {
	InsecureSkipVerify bool
}

type Endpoints struct {
	StorageJenkinsBaseURL       string
	StorageJenkinsJobPath       string
	CustomizationJenkinsBaseURL string
	ScalingJenkinsJobPath       string
	AutomationJenkinsJobPath    string
	BitbucketBaseURL            string
	BitbucketProjectKey         string
	BitbucketCustomizationRepo  string
	NexusSearchURL              string
	NexusRepositoryBaseURL      string
	NexusInternalProxyBaseURL   string
	HFParentRepoURL             string
}

func Load() *Config {
	return &Config{
		Port:           getEnvOrDefault("OCD_PORT", "2111"),
		WSLUser:        getEnvOrDefault("OCD_WSL_USER", "k8s"),
		ScriptName:     getEnvOrDefault("OCD_SCRIPT_NAME", "OCD.sh"),
		AllowedOrigins: getAllowedOrigins(),
		CommandTimeout: getEnvIntOrDefault("OCD_COMMAND_TIMEOUT", 1800),
		Jenkins: JenkinsConfig{
			URL:      getEnvOrDefault("OCD_JENKINS_URL", "https://jenkins-delivery.oss.corp.amdocs.aws"),
			Username: getEnvOrDefault("OCD_JENKINS_USERNAME", ""),
			Token:    getEnvOrDefault("OCD_JENKINS_TOKEN", ""),
		},
		TLS: TLSConfig{
			InsecureSkipVerify: getEnvBoolOrDefault("OCD_TLS_INSECURE_SKIP_VERIFY", false),
		},
		Endpoints: defaultEndpoints(),
	}
}

func DefaultEndpoints() Endpoints {
	return defaultEndpoints()
}

func defaultEndpoints() Endpoints {
	return Endpoints{
		StorageJenkinsBaseURL:       "http://ilososp030.corp.amdocs.com:7070",
		StorageJenkinsJobPath:       "/job/ATT_Storage_Creation",
		CustomizationJenkinsBaseURL: "https://jenkins-delivery.oss.corp.amdocs.aws",
		ScalingJenkinsJobPath:       "/job/Utility/job/OpsUtil/job/scaleUpOrDown",
		AutomationJenkinsJobPath:    "/job/Delivery/job/ATT_OSO/job/customization/job/develop",
		BitbucketBaseURL:            "https://ossbucket:7990",
		BitbucketProjectKey:         "ATTSVO",
		BitbucketCustomizationRepo:  "customization",
		NexusSearchURL:              "https://oss-nexus2.oss.corp.amdocs.aws/service/rest/v1/search?q=att-orchestration",
		NexusRepositoryBaseURL:      "https://oss-nexus2.oss.corp.amdocs.aws/repository/att.maven.snapshot/",
		NexusInternalProxyBaseURL:   "http://illin3613:8081/repository/maven.group/",
		HFParentRepoURL:             "https://ossbucket:7990/scm/attsvo/parent-pom.git",
	}
}

func getAllowedOrigins() []string {
	originsEnv := getEnvOrDefault("OCD_ALLOWED_ORIGINS", "localhost,127.0.0.1")
	if originsEnv == "*" {
		return []string{"*"}
	}

	origins := strings.Split(originsEnv, ",")
	var cleanOrigins []string
	for _, origin := range origins {
		cleanOrigins = append(cleanOrigins, strings.TrimSpace(origin))
	}
	return cleanOrigins
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		lowered := strings.ToLower(value)
		if lowered == "true" || lowered == "1" || lowered == "yes" {
			return true
		}
		if lowered == "false" || lowered == "0" || lowered == "no" {
			return false
		}
	}
	return defaultValue
}

func (e Endpoints) StorageJobRoot() string {
	base := strings.TrimRight(e.StorageJenkinsBaseURL, "/")
	jobPath := NormalizeJobPath(e.StorageJenkinsJobPath)
	return base + jobPath
}

func (e Endpoints) ScalingJobRoot(baseURL string) string {
	trimmedBase := strings.TrimRight(baseURL, "/")
	path := e.ScalingJenkinsJobPath
	if strings.TrimSpace(path) == "" {
		path = DefaultEndpoints().ScalingJenkinsJobPath
	}
	return trimmedBase + NormalizeJobPath(path)
}

func (e Endpoints) StorageJobURL(parts ...string) string {
	root := e.StorageJobRoot()
	if len(parts) == 0 {
		return root + "/"
	}

	builder := root
	for _, part := range parts {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "/") {
			builder += part
		} else {
			builder += "/" + part
		}
	}
	return builder
}

func (e Endpoints) BitbucketBranchesAPI() string {
	base := strings.TrimRight(e.BitbucketBaseURL, "/")
	return fmt.Sprintf("%s/rest/api/1.0/projects/%s/repos/%s/branches?limit=100&details=true", base, e.BitbucketProjectKey, e.BitbucketCustomizationRepo)
}

func (e Endpoints) ReplaceWithInternalNexus(url string) string {
	if e.NexusRepositoryBaseURL == "" || e.NexusInternalProxyBaseURL == "" {
		return url
	}
	return strings.Replace(url, e.NexusRepositoryBaseURL, e.NexusInternalProxyBaseURL, 1)
}

func NormalizeJobPath(path string) string {
	trimmed := strings.Trim(path, "/")
	return "/" + trimmed
}
