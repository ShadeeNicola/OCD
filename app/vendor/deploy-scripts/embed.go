package ocdscripts

import (
    "embed"
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


