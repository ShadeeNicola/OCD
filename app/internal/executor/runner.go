package executor

import (
    "context"
    "encoding/json"
    "strings"

    "app/internal/progress"
)

type Runner struct {
    executor *CommandExecutor
}

func NewRunner(ce *CommandExecutor) *Runner {
    return &Runner{executor: ce}
}

// SSE-based execution for deployment streaming
func (r *Runner) RunOCDScriptWithSSE(ctx context.Context, folderPath string, writer chan []byte) {
    r.executor.ExecuteWithSSE(ctx, folderPath, writer)
}

func (r *Runner) RunOCDScript(folderPath string) progress.Response { 
    return r.executor.Execute(folderPath) 
}

func detectProjectType(folderPath string) string { if strings.Contains(folderPath, "customization") { return "customization" }; return "att" }

// Helper function to send JSON messages via SSE
func sendSSEMessage(writer chan []byte, message interface{}) {
    if data, err := json.Marshal(message); err == nil {
        select {
        case writer <- data:
        default:
            // Channel full or closed, skip message
        }
    }
}



