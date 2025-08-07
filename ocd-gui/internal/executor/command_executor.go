package executor

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

    "ocd-gui/internal/config"
    "ocd-gui/internal/progress"
    "ocd-gui/internal/security"
)

type CommandExecutor struct { config *config.Config }

func NewCommandExecutor(cfg *config.Config) *CommandExecutor { return &CommandExecutor{config: cfg} }

func (ce *CommandExecutor) Execute(folderPath string) progress.Response {
    if err := security.ValidateFolderPath(folderPath); err != nil {
        return progress.Response{Message: fmt.Sprintf("Invalid folder path: %s", err.Error()), Success: false}
    }
    safeFolderPath := security.SanitizePath(folderPath)

    cmd, err := ce.buildCommand(safeFolderPath)
    if err != nil { return progress.Response{Message: err.Error(), Success: false} }

    output, err := cmd.CombinedOutput()
    if err != nil {
        return progress.Response{Message: fmt.Sprintf("Error: %s\nOutput: %s", err.Error(), string(output)), Success: false}
    }
    return progress.Response{Message: fmt.Sprintf("OCD deployment completed!\n%s", string(output)), Success: true}
}

func (ce *CommandExecutor) ExecuteWithWebSocket(folderPath string, conn *websocket.Conn, writeJSON func(*websocket.Conn, interface{}) error) {
    if err := security.ValidateFolderPath(folderPath); err != nil {
        writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Invalid folder path: %s", err.Error()), Success: false})
        return
    }
    safeFolderPath := security.SanitizePath(folderPath)

    cmd, err := ce.buildCommand(safeFolderPath)
    if err != nil {
        writeJSON(conn, progress.OutputMessage{Type: "complete", Content: err.Error(), Success: false})
        return
    }

    stdout, err := cmd.StdoutPipe()
    if err != nil { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Error creating stdout pipe: %s", err.Error()), Success: false}); return }
    stderr, err := cmd.StderrPipe()
    if err != nil { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Error creating stderr pipe: %s", err.Error()), Success: false}); return }

    if err := cmd.Start(); err != nil { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Error starting command: %s", err.Error()), Success: false}); return }

    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ce.config.CommandTimeout)*time.Second)
    defer cancel()

    go func() {
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            select { case <-ctx.Done(): return; default: }
            line := scanner.Text()
            if strings.Contains(line, "screen size is bogus") { continue }
            writeJSON(conn, progress.OutputMessage{Type: "output", Content: line})
            if pu := progress.ParseProgressFromOutput(line); pu != nil { writeJSON(conn, pu) }
        }
    }()

    go func() {
        scanner := bufio.NewScanner(stderr)
        for scanner.Scan() {
            select { case <-ctx.Done(): return; default: }
            line := scanner.Text()
            if strings.Contains(line, "screen size is bogus") { continue }
            writeJSON(conn, progress.OutputMessage{Type: "output", Content: line})
        }
    }()

    done := make(chan error, 1)
    go func() { done <- cmd.Wait() }()

    select {
    case <-ctx.Done():
        _ = cmd.Process.Kill()
        writeJSON(conn, progress.OutputMessage{Type: "complete", Content: "Deployment timed out", Success: false})
    case err := <-done:
        success := err == nil
        msg := "Check logs for more details"
        if success { msg = "Deployment completed successfully" }
        writeJSON(conn, progress.OutputMessage{Type: "complete", Content: msg, Success: success})
    }
}

func (ce *CommandExecutor) buildCommand(safeFolderPath string) (*exec.Cmd, error) {
    currentDir, err := os.Getwd()
    if err != nil { return nil, fmt.Errorf("error getting current directory: %s", err.Error()) }

    ocdScriptPath := filepath.Join(currentDir, "scripts", ce.config.ScriptName)
    sharedDirPath := filepath.Join(currentDir, "scripts", "shared")

    var cmd *exec.Cmd
    switch runtime.GOOS {
    case "windows":
        if _, err := exec.LookPath("wsl"); err == nil {
            wslPath := convertToWSLPath(safeFolderPath)
            ocdScriptWSLPath := convertToWSLPath(ocdScriptPath)
            sharedDirWSLPath := convertToWSLPath(sharedDirPath)
            cmd = exec.Command("wsl", "--user", ce.config.WSLUser, "bash", "-l", "-c",
                buildSecureWSLCommand(ocdScriptWSLPath, sharedDirWSLPath, wslPath))
        } else {
            return nil, fmt.Errorf("WSL not available on Windows. Please install WSL to use OCD")
        }
    case "linux":
        cmd = exec.Command("bash", "-l", "-c", buildSecureLinuxCommand(ocdScriptPath, sharedDirPath, safeFolderPath))
    default:
        return nil, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
    }

    cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLUMNS=120", "LINES=30")
    return cmd, nil
}

func buildSecureWSLCommand(scriptPath, sharedDirPath, folderPath string) string {
    return fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on && mkdir -p '%s/shared' && cp '%s' '%s/OCD.sh' && cp '%s'/* '%s/shared/' && chmod +x '%s/OCD.sh' && chmod +x '%s/shared/'* && cd '%s' && ./OCD.sh`, folderPath, scriptPath, folderPath, sharedDirPath, folderPath, folderPath, folderPath, folderPath)
}

func buildSecureLinuxCommand(scriptPath, sharedDirPath, folderPath string) string {
    return fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on && mkdir -p '%s/shared' && cp '%s' '%s/OCD.sh' && cp '%s'/* '%s/shared/' && chmod +x '%s/OCD.sh' && chmod +x '%s/shared/'* && cd '%s' && ./OCD.sh`, folderPath, scriptPath, folderPath, sharedDirPath, folderPath, folderPath, folderPath, folderPath)
}

func convertToWSLPath(windowsPath string) string {
    if runtime.GOOS != "windows" { return windowsPath }
    wslPath := strings.ReplaceAll(windowsPath, "\\", "/")
    if strings.HasPrefix(wslPath, "C:") { wslPath = "/mnt/c" + wslPath[2:] } else if len(wslPath) >= 2 && wslPath[1] == ':' { drive := strings.ToLower(string(wslPath[0])); wslPath = "/mnt/" + drive + wslPath[2:] }
    return wslPath
}


