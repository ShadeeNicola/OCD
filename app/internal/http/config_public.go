package httpapi

import (
	"net/http"

	"app/internal/config"
)

type publicConfigResponse struct {
	Endpoints publicEndpoints `json:"endpoints"`
}

type publicEndpoints struct {
	StorageJenkinsBaseURL       string `json:"storageJenkinsBaseUrl"`
	StorageJenkinsJobPath       string `json:"storageJenkinsJobPath"`
	CustomizationJenkinsBaseURL string `json:"customizationJenkinsBaseUrl"`
	ScalingJenkinsJobPath       string `json:"scalingJenkinsJobPath"`
	BitbucketBaseURL            string `json:"bitbucketBaseUrl"`
	BitbucketProjectKey         string `json:"bitbucketProjectKey"`
	BitbucketCustomizationRepo  string `json:"bitbucketCustomizationRepo"`
	NexusSearchURL              string `json:"nexusSearchUrl"`
	NexusRepositoryBaseURL      string `json:"nexusRepositoryBaseUrl"`
	NexusInternalProxyBaseURL   string `json:"nexusInternalProxyBaseUrl"`
	HFParentRepoURL             string `json:"hfParentRepoUrl"`
}

func HandlePublicConfig(configuration *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		resp := publicConfigResponse{
			Endpoints: publicEndpoints{
				StorageJenkinsBaseURL:       configuration.Endpoints.StorageJenkinsBaseURL,
				StorageJenkinsJobPath:       configuration.Endpoints.StorageJenkinsJobPath,
				CustomizationJenkinsBaseURL: configuration.Endpoints.CustomizationJenkinsBaseURL,
				ScalingJenkinsJobPath:       configuration.Endpoints.ScalingJenkinsJobPath,
				BitbucketBaseURL:            configuration.Endpoints.BitbucketBaseURL,
				BitbucketProjectKey:         configuration.Endpoints.BitbucketProjectKey,
				BitbucketCustomizationRepo:  configuration.Endpoints.BitbucketCustomizationRepo,
				NexusSearchURL:              configuration.Endpoints.NexusSearchURL,
				NexusRepositoryBaseURL:      configuration.Endpoints.NexusRepositoryBaseURL,
				NexusInternalProxyBaseURL:   configuration.Endpoints.NexusInternalProxyBaseURL,
				HFParentRepoURL:             configuration.Endpoints.HFParentRepoURL,
			},
		}

		writeJSON(w, http.StatusOK, resp)
	}
}
