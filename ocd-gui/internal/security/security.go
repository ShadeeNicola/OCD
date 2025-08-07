package security

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

// ValidateFolderPath ensures the path is safe and exists
func ValidateFolderPath(folderPath string) error {
    if folderPath == "" {
        return fmt.Errorf("folder path cannot be empty")
    }

    dangerousChars := regexp.MustCompile(`[;&|$\x60<>]`)
    if dangerousChars.MatchString(folderPath) {
        return fmt.Errorf("folder path contains invalid characters")
    }

    absPath, err := filepath.Abs(folderPath)
    if err != nil {
        return fmt.Errorf("invalid folder path: %v", err)
    }

    if _, err := os.Stat(absPath); os.IsNotExist(err) {
        return fmt.Errorf("folder does not exist: %s", absPath)
    }

    return nil
}

// SanitizePath removes dangerous characters and normalizes the path
func SanitizePath(path string) string {
    path = strings.ReplaceAll(path, "\x00", "")
    absPath, err := filepath.Abs(path)
    if err != nil {
        return path
    }
    return absPath
}


