package executor

import (
    "context"
    "encoding/json"
    "strings"

    "app/internal/progress"
)

var commandExecutor *CommandExecutor

func InitExecutor(ce *CommandExecutor) { commandExecutor = ce }

// SSE-based execution for deployment streaming
func RunOCDScriptWithSSE(ctx context.Context, folderPath string, writer chan []byte) {
    commandExecutor.ExecuteWithSSE(ctx, folderPath, writer)
}

func RunOCDScript(folderPath string) progress.Response { return commandExecutor.Execute(folderPath) }

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



