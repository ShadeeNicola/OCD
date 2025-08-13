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

## Recent Architectural Improvements (2025-08-13)

### Security Enhancements
- **Path Validation**: Enhanced to block command injection characters (`; & | $ \` < > ' "`)
- **Shell Escaping**: Proper escaping using `strconv.Quote()` instead of basic string formatting
- **Panic Elimination**: Replaced `panic()` calls with graceful error handling

### Architecture Improvements  
- **Dependency Injection**: Eliminated global variables, implemented proper DI with `Runner` struct
- **Thread Safety**: Removed race conditions and improved concurrent access patterns
- **Simplified Command Execution**: Reduced 30+ lines of over-engineered code to 3 lines
- **SSE Communication**: Migrated from WebSockets to Server-Sent Events for better browser compatibility

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
- **Backend**: Go 1.24.5 with dependency injection architecture
- **Frontend**: Vanilla JavaScript (ES6 modules), no frameworks  
- **Communication**: Server-Sent Events (SSE) for real-time deployment progress
- **Cross-platform**: Windows (WSL), Linux, macOS support
- **Security**: Enhanced path validation and shell escaping

### Project Type Detection
The system automatically detects project types:
- **ATT projects**: Uses `OCD.sh` script (default)
- **Customization projects**: Uses `OCD-customization.sh` (path contains "customization")

### Configuration
Environment variables:
- `OCD_PORT`: Server port (default: 2111)
- `OCD_WSL_USER`: WSL username for Windows (default: k8s)
- `OCD_COMMAND_TIMEOUT`: Command timeout in seconds (default: 1800)

### SSE Communication
- **Real-time progress updates** via `/api/deploy/stream/{sessionId}`
- **Session management**: `/api/deploy/start` creates session, `/api/deploy/cancel/{id}` cancels
- **Message types**: "output", "progress", "complete"
- **Progress stages**: prerequisites, settings, build, deploy, patch

### Cross-Platform Execution
- **Windows**: Requires WSL, executes via `wsl --user k8s bash -l -c`
- **Linux/macOS**: Direct bash execution with login shell
- **Path conversion**: Windows paths automatically converted to WSL paths

### Deployment Scripts Architecture (Refactored)
The deployment scripts have been refactored for better maintainability:

#### Shared Modules (`deploy-scripts/scripts/shared/`)
- **`arguments.sh`**: Common command-line argument parsing for both scripts
- **`utils.sh`**: Shared utilities including PowerShell execution helpers and environment detection
- **`maven.sh`**: Maven build functions and settings management
- **`kubernetes.sh`**: Kubernetes deployment and microservice management

#### Script Structure
- **`OCD.sh`** (489 lines, was 625): ATT project deployment
- **`OCD-customization.sh`** (333 lines, was 524): Customization project deployment
- **Total reduction**: 328 lines (28.5% smaller), improved maintainability

#### Key Improvements
- **Consolidated build functions**: Eliminated 3 duplicate functions in customization script
- **Shared PowerShell utilities**: Common pattern for cross-platform Maven execution
- **Unified argument parsing**: Single source of truth for command-line options
- **Enhanced maintainability**: Changes to shared logic only need to be made once

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