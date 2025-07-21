package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
)

func runOCDScriptWithWebSocket(folderPath string, conn *websocket.Conn) {
	// Validate folder path first
	if err := validateFolderPath(folderPath); err != nil {
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Invalid folder path: %s", err.Error()),
			Success: false,
		})
		return
	}

	// Sanitize the path
	safeFolderPath := sanitizePath(folderPath)

	currentDir, err := os.Getwd()
	if err != nil {
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Error getting current directory: %s", err.Error()),
			Success: false,
		})
		return
	}

	parentDir := filepath.Dir(currentDir)
	ocdScriptPath := filepath.Join(parentDir, "OCD.sh")

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wsl"); err == nil {
			wslPath := convertToWSLPath(safeFolderPath)
			ocdScriptWSLPath := convertToWSLPath(ocdScriptPath)

			// Use exec.Command with separate arguments for security
			cmd = exec.Command("wsl", "--user", appConfig.WSLUser, "bash", "-l", "-c",
				buildSecureWSLCommand(ocdScriptWSLPath, wslPath))
		} else {
			writeToWebSocket(conn, OutputMessage{
				Type:    "complete",
				Content: "WSL not available on Windows. Please install WSL to use OCD.",
				Success: false,
			})
			return
		}

	case "linux":
		cmd = exec.Command("bash", "-l", "-c",
			buildSecureLinuxCommand(ocdScriptPath, safeFolderPath))

	default:
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Unsupported operating system: %s", runtime.GOOS),
			Success: false,
		})
		return
	}

	// Set environment to avoid terminal size issues
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLUMNS=120",
		"LINES=30",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Error creating stdout pipe: %s", err.Error()),
			Success: false,
		})
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Error creating stderr pipe: %s", err.Error()),
			Success: false,
		})
		return
	}

	if err := cmd.Start(); err != nil {
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Error starting command: %s", err.Error()),
			Success: false,
		})
		return
	}

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip bogus screen size messages
			if strings.Contains(line, "screen size is bogus") {
				continue
			}

			// Send raw output
			writeToWebSocket(conn, OutputMessage{
				Type:    "output",
				Content: line,
			})

			// Parse and send progress updates
			if progress := parseProgressFromOutput(line); progress != nil {
				writeToWebSocket(conn, progress)
			}
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()

			// Skip bogus screen size messages
			if strings.Contains(line, "screen size is bogus") {
				continue
			}

			writeToWebSocket(conn, OutputMessage{
				Type:    "output",
				Content: line,
			})
		}
	}()

	// Wait for command to complete
	err = cmd.Wait()
	success := err == nil

	// Send better completion message
	var completionMessage string
	if success {
		completionMessage = "Deployment completed successfully"
	} else {
		completionMessage = "Check logs for more details"
	}

	writeToWebSocket(conn, OutputMessage{
		Type:    "complete",
		Content: completionMessage,
		Success: success,
	})
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
func runOCDScript(folderPath string) Response {
	// Validate folder path first
	if err := validateFolderPath(folderPath); err != nil {
		return Response{
			Message: fmt.Sprintf("Invalid folder path: %s", err.Error()),
			Success: false,
		}
	}

	// Sanitize the path
	safeFolderPath := sanitizePath(folderPath)

	currentDir, err := os.Getwd()
	if err != nil {
		return Response{
			Message: fmt.Sprintf("Error getting current directory: %s", err.Error()),
			Success: false,
		}
	}

	parentDir := filepath.Dir(currentDir)
	ocdScriptPath := filepath.Join(parentDir, "OCD.sh")

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// Check if WSL is available
		if _, err := exec.LookPath("wsl"); err == nil {
			wslPath := convertToWSLPath(safeFolderPath)
			ocdScriptWSLPath := convertToWSLPath(ocdScriptPath)

			cmd = exec.Command("wsl", "--user", appConfig.WSLUser, "bash", "-l", "-c",
				buildSecureWSLCommand(ocdScriptWSLPath, wslPath))
		} else {
			return Response{
				Message: "WSL not available on Windows. Please install WSL to use OCD.",
				Success: false,
			}
		}

	case "linux":
		cmd = exec.Command("bash", "-l", "-c",
			buildSecureLinuxCommand(ocdScriptPath, safeFolderPath))

	default:
		return Response{
			Message: fmt.Sprintf("Unsupported operating system: %s", runtime.GOOS),
			Success: false,
		}
	}

	output, err := cmd.CombinedOutput()

	if err != nil {
		return Response{
			Message: fmt.Sprintf("Error: %s\nOutput: %s", err.Error(), string(output)),
			Success: false,
		}
	}

	return Response{
		Message: fmt.Sprintf("OCD deployment completed!\n%s", string(output)),
		Success: true,
	}
}
