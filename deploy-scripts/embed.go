package ocdscripts

import (
    "embed"
    "io/fs"
)

//go:embed scripts/*.sh scripts/shared/*.sh
var scriptsFS embed.FS

// ReadScript returns the contents of a top-level script like OCD.sh
func ReadScript(name string) ([]byte, error) {
    return scriptsFS.ReadFile("scripts/" + name)
}

// ReadShared returns the contents of a shared script like utils.sh or maven.sh
func ReadShared(name string) ([]byte, error) {
    return scriptsFS.ReadFile("scripts/shared/" + name)
}

// ReadDir returns directory entries for dynamic discovery
func ReadDir(path string) ([]fs.DirEntry, error) {
    return scriptsFS.ReadDir(path)
}


