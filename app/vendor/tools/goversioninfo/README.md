# Vendored goversioninfo Tool

This directory contains a pre-built `goversioninfo.exe` binary to avoid network dependencies during CI/CD builds.

## Purpose
- Embeds Windows icons and version information into Go executables
- Eliminates need for `go install` commands during CI builds
- Ensures consistent builds across environments

## Source
- Original tool: https://github.com/josephspurrier/goversioninfo
- Binary version: Latest stable release
- Platform: Windows x64

## Usage
The build scripts automatically use this vendored version when available:
- Local builds: `build/build-all-executables.sh`
- CI builds: `.gitlab-ci.yml`

## Updating
To update the tool:
1. Download latest release from GitHub
2. Replace `goversioninfo.exe` in this directory
3. Test local builds to ensure compatibility
