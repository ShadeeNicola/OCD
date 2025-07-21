package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
)

var commandExecutor *CommandExecutor

func initCommandExecutor() {
	commandExecutor = NewCommandExecutor(appConfig)
}

func runOCDScriptWithWebSocket(folderPath string, conn *websocket.Conn) {
	commandExecutor.ExecuteWithWebSocket(folderPath, conn)
}

func runOCDScript(folderPath string) Response {
	return commandExecutor.Execute(folderPath)
}

// buildSecureWSLCommand creates a secure command string for WSL
func buildSecureWSLCommand(scriptPath, folderPath string) string {
	// Use single quotes to prevent shell interpretation
	return fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on && cp '%s' '%s/OCD.sh' && chmod +x '%s/OCD.sh' && cd '%s' && ./OCD.sh`,
		scriptPath, folderPath, folderPath, folderPath)
}

// buildSecureLinuxCommand creates a secure command string for Linux
func buildSecureLinuxCommand(scriptPath, folderPath string) string {
	// Use single quotes to prevent shell interpretation
	return fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on && cp '%s' '%s/OCD.sh' && chmod +x '%s/OCD.sh' && cd '%s' && ./OCD.sh`,
		scriptPath, folderPath, folderPath, folderPath)
}

func convertToWSLPath(windowsPath string) string {
	if runtime.GOOS != "windows" {
		return windowsPath // Return as-is for Linux
	}

	wslPath := strings.ReplaceAll(windowsPath, "\\", "/")
	if strings.HasPrefix(wslPath, "C:") {
		wslPath = "/mnt/c" + wslPath[2:]
	} else if len(wslPath) >= 2 && wslPath[1] == ':' {
		drive := strings.ToLower(string(wslPath[0]))
		wslPath = "/mnt/" + drive + wslPath[2:]
	}
	return wslPath
}
