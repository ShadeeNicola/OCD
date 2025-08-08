package executor

import (
    "bufio"
    "context"
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "path/filepath"
    "runtime"
    "strings"

    "github.com/gorilla/websocket"

    "app/internal/progress"
    ocdscripts "deploy-scripts"
)

var commandExecutor *CommandExecutor

func InitExecutor(ce *CommandExecutor) { commandExecutor = ce }

func RunOCDScriptWithWebSocket(ctx context.Context, folderPath string, conn *websocket.Conn, writeJSON func(*websocket.Conn, interface{}) error) {
    tempScriptPath, err := createTempOCDScript(folderPath)
    if err != nil { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Error creating temporary script: %s", err.Error()), Success: false}); return }
    defer os.Remove(tempScriptPath)

    tempSharedDir, err := createTempSharedFiles()
    if err != nil { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Error creating temporary shared files: %s", err.Error()), Success: false}); return }
    defer os.RemoveAll(tempSharedDir)

    var cmd *exec.Cmd
    projectType := detectProjectType(folderPath)
    scriptName := "OCD.sh"
    if projectType == "customization" { scriptName = "OCD-customization.sh" }

    switch runtime.GOOS {
    case "windows":
        if _, err := exec.LookPath("wsl"); err == nil {
            wslPath := convertToWSLPath(folderPath)
            tempScriptWSLPath := convertToWSLPath(tempScriptPath)
            tempSharedDirWSLPath := convertToWSLPath(tempSharedDir)
            cmd = exec.Command("wsl", "--user", "k8s", "bash", "-l", "-c",
                fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on && mkdir -p "%s/shared" && cp "%s" "%s/%s" && cp "%s"/* "%s/shared/" && chmod +x "%s/%s" && chmod +x "%s/shared/"* && cd "%s" && ./%s`,
                    wslPath, tempScriptWSLPath, wslPath, scriptName, tempSharedDirWSLPath, wslPath, wslPath, scriptName, wslPath, wslPath, scriptName))
        } else { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: "WSL not available on Windows. Please install WSL to use OCD.", Success: false}); return }
    case "linux":
        cmd = exec.Command("bash", "-l", "-c",
            fmt.Sprintf(`export MAVEN_OPTS="-Dorg.slf4j.simpleLogger.showDateTime=true -Dorg.slf4j.simpleLogger.dateTimeFormat=HH:mm:ss" && export OCD_VERBOSE=true && proxy on && mkdir -p "%s/shared" && cp "%s" "%s/%s" && cp "%s"/* "%s/shared/" && chmod +x "%s/%s" && chmod +x "%s/shared/"* && cd "%s" && ./%s`,
                folderPath, tempScriptPath, folderPath, scriptName, tempSharedDir, folderPath, folderPath, scriptName, folderPath, folderPath, scriptName))
    default:
        writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Unsupported operating system: %s", runtime.GOOS), Success: false}); return
    }

    cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLUMNS=120", "LINES=30")

    stdout, err := cmd.StdoutPipe(); if err != nil { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Error creating stdout pipe: %s", err.Error()), Success: false}); return }
    stderr, err := cmd.StderrPipe(); if err != nil { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Error creating stderr pipe: %s", err.Error()), Success: false}); return }
    if err := cmd.Start(); err != nil { writeJSON(conn, progress.OutputMessage{Type: "complete", Content: fmt.Sprintf("Error starting command: %s", err.Error()), Success: false}); return }

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
        writeJSON(conn, progress.OutputMessage{Type: "complete", Content: "Deployment aborted by user", Success: false})
    case err := <-done:
        success := err == nil
        msg := "Check logs for more details"
        if success { msg = "Deployment completed successfully" }
        writeJSON(conn, progress.OutputMessage{Type: "complete", Content: msg, Success: success})
    }
}

func RunOCDScript(folderPath string) progress.Response { return commandExecutor.Execute(folderPath) }

func detectProjectType(folderPath string) string { if strings.Contains(folderPath, "customization") { return "customization" }; return "att" }

func createTempOCDScript(folderPath string) (string, error) {
    projectType := detectProjectType(folderPath)
    scriptName := "OCD.sh"
    if projectType == "customization" { scriptName = "OCD-customization.sh" }
    scriptContent, err := ocdscripts.ReadScript(scriptName)
    if err != nil { return "", fmt.Errorf("failed to read embedded script %s: %w", scriptName, err) }
    tempFile, err := ioutil.TempFile("", "OCD_*.sh"); if err != nil { return "", fmt.Errorf("failed to create temp file: %w", err) }
    defer tempFile.Close()
    if _, err := tempFile.Write(scriptContent); err != nil { return "", fmt.Errorf("failed to write script to temp file: %w", err) }
    if err := os.Chmod(tempFile.Name(), 0755); err != nil { return "", fmt.Errorf("failed to make script executable: %w", err) }
    return tempFile.Name(), nil
}

func createTempSharedFiles() (string, error) {
    tempDir, err := ioutil.TempDir("", "OCD_shared_*"); if err != nil { return "", fmt.Errorf("failed to create temp directory: %w", err) }
    sharedFiles := []string{"utils.sh", "maven.sh"}
    for _, sharedFile := range sharedFiles {
        content, err := ocdscripts.ReadShared(sharedFile)
        if err != nil { os.RemoveAll(tempDir); return "", fmt.Errorf("failed to read embedded shared file %s: %w", sharedFile, err) }
        filePath := filepath.Join(tempDir, filepath.Base(sharedFile))
        if err := ioutil.WriteFile(filePath, content, 0644); err != nil { os.RemoveAll(tempDir); return "", fmt.Errorf("failed to write shared file %s: %w", sharedFile, err) }
    }
    return tempDir, nil
}


