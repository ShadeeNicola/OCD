package main

import (
	"embed"
	"io/fs"
)

//go:embed web/*
var webAssets embed.FS

// GetWebFS returns the embedded web filesystem
func GetWebFS() fs.FS {
	webFS, err := fs.Sub(webAssets, "web")
	if err != nil {
		panic(err)
	}
	return webFS
}
