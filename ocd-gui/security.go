package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// validateFolderPath ensures the path is safe and exists
func validateFolderPath(folderPath string) error {
	if folderPath == "" {
		return fmt.Errorf("folder path cannot be empty")
	}

	// Check for dangerous characters
	dangerousChars := regexp.MustCompile(`[;&|$\x60<>]`)
	if dangerousChars.MatchString(folderPath) {
		return fmt.Errorf("folder path contains invalid characters")
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(folderPath)
	if err != nil {
		return fmt.Errorf("invalid folder path: %v", err)
	}

	// Check if directory exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("folder does not exist: %s", absPath)
	}

	return nil
}

// sanitizePath removes dangerous characters and normalizes the path
func sanitizePath(path string) string {
	// Remove any null bytes
	path = strings.ReplaceAll(path, "\x00", "")

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return absPath
}
