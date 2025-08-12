package executor

import (
    "context"
    "strings"

    "github.com/gorilla/websocket"

    "app/internal/progress"
)

var commandExecutor *CommandExecutor

func InitExecutor(ce *CommandExecutor) { commandExecutor = ce }

func RunOCDScriptWithWebSocket(ctx context.Context, folderPath string, conn *websocket.Conn, writeJSON func(*websocket.Conn, interface{}) error) {
    commandExecutor.ExecuteWithWebSocket(folderPath, conn, writeJSON)
}

func RunOCDScript(folderPath string) progress.Response { return commandExecutor.Execute(folderPath) }

func detectProjectType(folderPath string) string { if strings.Contains(folderPath, "customization") { return "customization" }; return "att" }



