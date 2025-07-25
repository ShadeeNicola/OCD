package main

import (
	"bufio"
	"embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/gorilla/websocket"
)

//go:embed OCD.sh
var ocdScript embed.FS

var commandExecutor *CommandExecutor

func initCommandExecutor() {
	commandExecutor = NewCommandExecutor(appConfig)
}

func runOCDScriptWithWebSocket(folderPath string, conn *websocket.Conn) {
	// Create a temporary script file
	tempScriptPath, err := createTempOCDScript()
	if err != nil {
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Error creating temporary script: %s", err.Error()),
			Success: false,
		})
		return
	}
	defer os.Remove(tempScriptPath) // Clean up temp file

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wsl"); err == nil {
			wslPath := convertToWSLPath(folderPath)
			tempScriptWSLPath := convertToWSLPath(tempScriptPath)

			cmd = exec.Command("wsl", "--user", "k8s", "bash", "-l", "-c",
				fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on && cp "%s" "%s/OCD.sh" && chmod +x "%s/OCD.sh" && cd "%s" && ./OCD.sh`,
					tempScriptWSLPath, wslPath, wslPath, wslPath))
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
			fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on && cp "%s" "%s/OCD.sh" && chmod +x "%s/OCD.sh" && cd "%s" && ./OCD.sh`,
				tempScriptPath, folderPath, folderPath, folderPath))

	default:
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Unsupported operating system: %s", runtime.GOOS),
			Success: false,
		})
		return
	}

	// Rest of your existing code remains the same...
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

			if strings.Contains(line, "screen size is bogus") {
				continue
			}

			writeToWebSocket(conn, OutputMessage{
				Type:    "output",
				Content: line,
			})

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

			if strings.Contains(line, "screen size is bogus") {
				continue
			}

			writeToWebSocket(conn, OutputMessage{
				Type:    "output",
				Content: line,
			})
		}
	}()

	err = cmd.Wait()
	success := err == nil

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

func createTempOCDScript() (string, error) {
	// Read the embedded script
	scriptContent, err := ocdScript.ReadFile("OCD.sh")
	if err != nil {
		return "", fmt.Errorf("failed to read embedded script: %w", err)
	}

	// Create a temporary file
	tempFile, err := ioutil.TempFile("", "OCD_*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Write the script content to the temp file
	if _, err := tempFile.Write(scriptContent); err != nil {
		return "", fmt.Errorf("failed to write script to temp file: %w", err)
	}

	// Make the file executable
	if err := os.Chmod(tempFile.Name(), 0755); err != nil {
		return "", fmt.Errorf("failed to make script executable: %w", err)
	}

	return tempFile.Name(), nil
}
