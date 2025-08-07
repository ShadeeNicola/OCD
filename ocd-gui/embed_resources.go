package ocdgui

import (
    "embed"
    "io/fs"
)

// Embed web UI and scripts from the module root
//go:embed web/* scripts/*.sh scripts/shared/*.sh
var rootAssets embed.FS

// WebFS provides the embedded filesystem for serving the web UI
func WebFS() fs.FS {
    webFS, err := fs.Sub(rootAssets, "web")
    if err != nil {
        panic(err)
    }
    return webFS
}

// ReadScript reads a script file by name from the embedded scripts directory
func ReadScript(name string) ([]byte, error) {
    return rootAssets.ReadFile("scripts/" + name)
}

// ReadShared reads a shared helper script by name from the embedded scripts/shared directory
func ReadShared(name string) ([]byte, error) {
    return rootAssets.ReadFile("scripts/shared/" + name)
}


