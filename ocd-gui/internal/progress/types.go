package progress

type Response struct {
    Message    string `json:"message"`
    Success    bool   `json:"success"`
    FolderPath string `json:"folderPath,omitempty"`
}

type DeployRequest struct {
    FolderPath string `json:"folderPath"`
}

type BrowseResponse struct {
    FolderPath string `json:"folderPath"`
    Success    bool   `json:"success"`
    Message    string `json:"message"`
}

type OutputMessage struct {
    Type    string `json:"type"` // "output", "progress", "complete"
    Content string `json:"content"`
    Success bool   `json:"success,omitempty"`
}

type ProgressUpdate struct {
    Type    string `json:"type"`    // "progress"
    Stage   string `json:"stage"`   // "settings", "build", "deploy", "patch"
    Service string `json:"service"` // microservice name
    Status  string `json:"status"`  // "pending", "running", "success", "error"
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}


