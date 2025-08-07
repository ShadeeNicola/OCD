package ui

import (
    "fmt"
    "os"
    "os/exec"
    "runtime"
    "strings"
)

func OpenFolderDialog() (string, error) {
    switch runtime.GOOS {
    case "windows":
        return openFolderDialogWindows()
    case "linux":
        return openFolderDialogLinux()
    default:
        return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
    }
}

func openFolderDialogWindows() (string, error) {
    psScript := `
Add-Type -AssemblyName System.Windows.Forms
$folderBrowser = New-Object System.Windows.Forms.FolderBrowserDialog
$folderBrowser.Description = "Select your Git repository folder"
$folderBrowser.ShowNewFolderButton = $false
$result = $folderBrowser.ShowDialog()
if ($result -eq [System.Windows.Forms.DialogResult]::OK) {
    Write-Output $folderBrowser.SelectedPath
} else {
    Write-Output "CANCELLED"
}`

    cmd := exec.Command("powershell", "-Command", psScript)
    output, err := cmd.Output()
    if err != nil { return "", err }

    selectedPath := strings.TrimSpace(string(output))
    if selectedPath == "CANCELLED" || selectedPath == "" { return "", nil }
    return selectedPath, nil
}

func openFolderDialogLinux() (string, error) {
    dialogTools := []struct{ cmd string; args []string }{
        {"zenity", []string{"--file-selection", "--directory", "--title=Select your Git repository folder"}},
        {"kdialog", []string{"--getexistingdirectory", ".", "Select your Git repository folder"}},
        {"yad", []string{"--file-selection", "--directory", "--title=Select your Git repository folder"}},
    }

    for _, tool := range dialogTools {
        if _, err := exec.LookPath(tool.cmd); err == nil {
            cmd := exec.Command(tool.cmd, tool.args...)
            output, err := cmd.Output()
            if err != nil { continue }
            selectedPath := strings.TrimSpace(string(output))
            if selectedPath == "" { return "", nil }
            return selectedPath, nil
        }
    }

    currentDir, err := os.Getwd()
    if err != nil { return "", fmt.Errorf("no GUI dialog available and cannot get current directory") }
    return currentDir, fmt.Errorf("no GUI dialog available, using current directory: %s", currentDir)
}


