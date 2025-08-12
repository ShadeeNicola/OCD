# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

### Development Build (Single Platform)
```bash
# Windows executable
(cd app && go build -ldflags="-s -w" -o ../dist/OCD.exe ./cmd/ocd-gui)

# Linux x64
(cd app && GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../dist/OCD-Tool-Linux-x64 ./cmd/ocd-gui)

# macOS Apple Silicon
(cd app && GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../dist/OCD-Tool-macOS-AppleSilicon ./cmd/ocd-gui)
```

### Cross-Platform Build (All Platforms)
```bash
chmod +x build/build-all-executables.sh
DIST_DIR=dist ./build/build-all-executables.sh
```

## Project Architecture

### Module Structure
- **Main Go module**: `app/` (contains the web server and business logic)
- **Deployment scripts module**: `deploy-scripts/` (embedded shell scripts)
- **Build output**: `dist/` (generated executables)

### Key Components
- **Entry point**: `app/cmd/ocd-gui/main.go` - HTTP server with embedded web assets
- **HTTP handlers**: `app/internal/http/` - REST API and WebSocket endpoints
- **Deployment execution**: `app/internal/executor/` - Script orchestration and real-time output streaming
- **Frontend**: `app/web/` - Vanilla JavaScript with WebSocket communication
- **Embedded scripts**: `deploy-scripts/scripts/` - Shell scripts for ATT and customization projects

### Technology Stack
- **Backend**: Go 1.24.5 with Gorilla WebSockets
- **Frontend**: Vanilla JavaScript (ES6 modules), no frameworks
- **Communication**: WebSocket for real-time deployment progress
- **Cross-platform**: Windows (WSL), Linux, macOS support

### Project Type Detection
The system automatically detects project types:
- **ATT projects**: Uses `OCD.sh` script (default)
- **Customization projects**: Uses `OCD-customization.sh` (path contains "customization")

### Configuration
Environment variables:
- `OCD_PORT`: Server port (default: 2111)
- `OCD_WSL_USER`: WSL username for Windows (default: k8s)
- `OCD_COMMAND_TIMEOUT`: Command timeout in seconds (default: 1800)

### WebSocket Communication
- **Real-time progress updates** via `/ws/deploy`
- **Message types**: "output", "progress", "complete"
- **Progress stages**: prerequisites, settings, build, deploy, patch

### Cross-Platform Execution
- **Windows**: Requires WSL, executes via `wsl --user k8s bash -l -c`
- **Linux/macOS**: Direct bash execution with login shell
- **Path conversion**: Windows paths automatically converted to WSL paths

## Development Principles

### Core Rules
- **NO TEMPORARY SOLUTIONS**: Never implement temporary fixes, workarounds, or "for testing" changes
- **Production-Ready Code**: All implementations must be permanent, maintainable, and production-ready
- **Proper Investigation**: If a solution requires more investigation, do the investigation first before implementing
- **No Bypass Logic**: Don't add conditional logic to bypass issues - fix the root cause
- **Clean Architecture**: Maintain clean, readable, and well-structured code at all times

### Code Conventions
- **Go**: Standard Go project layout with `internal/` packages
- **JavaScript**: ES6 modules, no build system required
- **Embedded assets**: Web files embedded via Go's embed package
- **Scripts**: Shell scripts embedded via separate module

### Security Features
- Origin validation for WebSocket connections
- Path sanitization and Git repository validation
- Temporary file management with proper cleanup

### Testing
No formal test framework is currently configured. Manual testing across platforms is required.