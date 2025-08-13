package ocdgui

import (
    "embed"
    "io/fs"
)

// Embed web UI only from the module root
//go:embed web/*
var rootAssets embed.FS

// WebFS provides the embedded filesystem for serving the web UI
func WebFS() (fs.FS, error) {
    webFS, err := fs.Sub(rootAssets, "web")
    if err != nil {
        return nil, err
    }
    return webFS, nil
}

