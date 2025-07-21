package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type CommandExecutor struct {
	config *Config
}

func NewCommandExecutor(config *Config) *CommandExecutor {
	return &CommandExecutor{config: config}
}

func (ce *CommandExecutor) ExecuteWithWebSocket(folderPath string, conn *websocket.Conn) {
	// Validate folder path first
	if err := validateFolderPath(folderPath); err != nil {
		appLogger.Errorf("Invalid folder path: %s", err.Error())
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: fmt.Sprintf("Invalid folder path: %s", err.Error()),
			Success: false,
		})
		return
	}

	// Sanitize the path
	safeFolderPath := sanitizePath(folderPath)
	appLogger.Infof("Starting WebSocket deployment for path: %s", safeFolderPath)

	cmd, err := ce.buildCommand(safeFolderPath)
	if err != nil {
		appLogger.Errorf("Failed to build command: %s", err.Error())
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: err.Error(),
			Success: false,
		})
		return
	}

	ce.executeCommandWithStreaming(cmd, conn)
}

func (ce *CommandExecutor) Execute(folderPath string) Response {
	// Validate folder path first
	if err := validateFolderPath(folderPath); err != nil {
		appLogger.Errorf("Invalid folder path: %s", err.Error())
		return Response{
			Message: fmt.Sprintf("Invalid folder path: %s", err.Error()),
			Success: false,
		}
	}

	// Sanitize the path
	safeFolderPath := sanitizePath(folderPath)
	appLogger.Infof("Starting deployment for path: %s", safeFolderPath)

	cmd, err := ce.buildCommand(safeFolderPath)
	if err != nil {
		appLogger.Errorf("Failed to build command: %s", err.Error())
		return Response{
			Message: err.Error(),
			Success: false,
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		appLogger.Errorf("Command execution failed: %s", err.Error())
		return Response{
			Message: fmt.Sprintf("Error: %s\nOutput: %s", err.Error(), string(output)),
			Success: false,
		}
	}

	appLogger.Info("Deployment completed successfully")
	return Response{
		Message: fmt.Sprintf("OCD deployment completed!\n%s", string(output)),
		Success: true,
	}
}

func (ce *CommandExecutor) buildCommand(safeFolderPath string) (*exec.Cmd, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current directory: %s", err.Error())
	}

	parentDir := filepath.Dir(currentDir)
	ocdScriptPath := filepath.Join(parentDir, ce.config.ScriptName)

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		if _, err := exec.LookPath("wsl"); err == nil {
			wslPath := convertToWSLPath(safeFolderPath)
			ocdScriptWSLPath := convertToWSLPath(ocdScriptPath)

			cmd = exec.Command("wsl", "--user", ce.config.WSLUser, "bash", "-l", "-c",
				buildSecureWSLCommand(ocdScriptWSLPath, wslPath))
		} else {
			return nil, fmt.Errorf("WSL not available on Windows. Please install WSL to use OCD")
		}

	case "linux":
		cmd = exec.Command("bash", "-l", "-c",
			buildSecureLinuxCommand(ocdScriptPath, safeFolderPath))

	default:
		return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Set environment to avoid terminal size issues
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLUMNS=120",
		"LINES=30",
	)

	return cmd, nil
}

func (ce *CommandExecutor) executeCommandWithStreaming(cmd *exec.Cmd, conn *websocket.Conn) {
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

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ce.config.CommandTimeout)*time.Second)
	defer cancel()

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
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
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
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
		}
	}()

	// Wait for command to complete or timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		cmd.Process.Kill()
		writeToWebSocket(conn, OutputMessage{
			Type:    "complete",
			Content: "Deployment timed out",
			Success: false,
		})
	case err := <-done:
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
}
