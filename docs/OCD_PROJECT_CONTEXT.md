# OCD (One Click Deployer) - Complete Project Context

## Project Overview

OCD (One Click Deployer) is a GUI-based deployment tool that simplifies the deployment process for microservices with real-time progress tracking and an intuitive web interface. It's designed to automatically detect changed microservices based on Git status and deploy only what's necessary.

## Architecture Overview

### Technology Stack
- **Backend**: Go (Golang) 1.24.5
- **Frontend**: Vanilla JavaScript (ES6 modules)
- **Communication**: Server-Sent Events (SSE) for real-time deployment progress
- **Deployment**: Shell scripts (`OCD.sh`, `OCD-customization.sh`) embedded via `deploy-scripts` module
- **Build & Packaging**: GitLab CI on tags builds cross-platform binaries and publishes Release assets; local `build/build-all-executables.sh` for contributors
- **Versioning**: Version injected at build from Git tag via ldflags; exposed by `GET /api/health`
- **Cross-platform**: Windows (WSL), Linux, macOS support

### Project Structure
```
OCD/
├── app/                            # Main application directory
│   ├── cmd/ocd-gui/               # Go entrypoint (main package)
│   │   └── main.go
│   ├── internal/                  # Non-exported application packages
│   │   ├── http/                  # HTTP and WebSocket layer
│   │   │   ├── handlers.go
│   │   │   └── ws.go
│   │   ├── executor/              # Script execution and orchestration
│   │   │   ├── command_executor.go
│   │   │   └── runner.go
│   │   ├── config/                # Configuration loading
│   │   │   └── config.go
│   │   ├── logging/               # Logger wrapper
│   │   │   └── logger.go
│   │   ├── security/              # Validation and path utilities
│   │   │   └── security.go
│   │   ├── progress/              # Progress parsing and DTOs
│   │   │   ├── parser.go
│   │   │   └── types.go
│   │   └── ui/                    # UI embedding and dialogs
│   │       ├── assets.go
│   │       └── dialog.go
│   ├── (no deployment scripts embedded here)
│   ├── web/                       # Frontend assets
│   │   ├── index.html
│   │   ├── styles.css
│   │   └── js/
│   │       ├── main.js
│   │       ├── constants.js
│   │       ├── utils.js
│   │       ├── history-manager.js
│   │       └── progress-manager.js
│   ├── go.mod                     # Go module
│   └── go.sum
├── deploy-scripts/                # Embedded deployment scripts module
│   ├── go.mod
│   ├── embed.go                   # Exposes ReadScript/ReadShared
│   └── scripts/
│       ├── OCD.sh
│       ├── OCD-customization.sh
│       └── shared/                     # Shared utilities (refactored)
│           ├── utils.sh               # Common utilities & PowerShell functions
│           ├── maven.sh               # Maven build functions
│           ├── kubernetes.sh          # Kubernetes deployment functions  
│           └── arguments.sh           # Common argument parsing
├── build/                         # Build utilities (root-level)
│   └── build-all-executables.sh   # Cross-platform build (outputs to repo dist/)
├── README.md                      # Project documentation
└── OCD_PROJECT_CONTEXT.md         # This context document
```

## Backend Architecture (Go)

### Core Components

#### 1. Main Application (`app/cmd/ocd-gui/main.go`)
- **Purpose**: Application entry point and server initialization
- **Key Features**:
  - Embedded web filesystem loading
  - HTTP server setup on port 2111 (configurable)
  - Automatic browser opening
  - Cross-platform path handling
  - Route configuration

#### 2. HTTP Handlers (`internal/http/`)
- **Endpoints**:
  - `GET /api/browse` - File dialog for folder selection
  - `POST /api/deploy` - Traditional deployment endpoint (synchronous)
  - `GET /api/health` - Health check endpoint with version info
  - `POST /api/deploy/start` - Start SSE deployment session
  - `GET /api/deploy/stream/{sessionId}` - SSE stream for real-time progress
  - `POST /api/deploy/cancel/{sessionId}` - Cancel deployment session

#### 3. SSE Communication (`internal/http/sse.go`)
- **Purpose**: Server-Sent Events for real-time deployment progress streaming
- **Features**:
  - Origin validation for security
  - Thread-safe message writing
  - Connection management
  - Error handling

#### 4. Deployment Runner (`internal/executor/runner.go`)
- **Purpose**: Thread-safe deployment orchestration with dependency injection
- **Architecture**: Uses `Runner` struct with injected `CommandExecutor` (no global variables)
- **Key Functions**:
  - `RunOCDScript()` - Synchronous deployment execution
  - `RunOCDScriptWithSSE()` - Real-time streaming deployment execution
  - Thread-safe message streaming via channels

#### 5. Command Executor (`internal/executor/command_executor.go`)
- **Purpose**: Secure command execution and cross-platform script management
- **Features**:
  - Enhanced path validation and sanitization (blocks injection characters)
  - Command timeout handling (30 minutes default)
  - Real-time stdout/stderr streaming via SSE
  - Proper shell escaping using `strconv.Quote()`
  - Cross-platform script embedding and temporary file management

#### 6. Configuration (`internal/config/config.go`)
- **Configuration Options**:
  - `OCD_PORT` - Server port (default: 2111)
  - `OCD_WSL_USER` - WSL username (default: k8s)
  - `OCD_SCRIPT_NAME` - Script name (default: OCD.sh)
  - `OCD_ALLOWED_ORIGINS` - CORS origins (default: localhost,127.0.0.1)
  - `OCD_COMMAND_TIMEOUT` - Command timeout in seconds (default: 1800)

### Data Structures (`internal/progress/types.go`)

```go
type Response struct {
    Message    string `json:"message"`
    Success    bool   `json:"success"`
    FolderPath string `json:"folderPath,omitempty"`
}

type DeployRequest struct {
    FolderPath string `json:"folderPath"`
}

type BrowseResponse struct {
    FolderPath string `json:"folderPath"`
    Success    bool   `json:"success"`
    Message    string `json:"message"`
}

type OutputMessage struct {
    Type    string `json:"type"` // "output", "progress", "complete"
    Content string `json:"content"`
    Success bool   `json:"success,omitempty"`
}

type ProgressUpdate struct {
    Type    string `json:"type"`    // "progress"
    Stage   string `json:"stage"`   // "settings", "build", "deploy", "patch"
    Service string `json:"service"` // microservice name
    Status  string `json:"status"`  // "pending", "running", "success", "error"
    Message string `json:"message"`
    Details string `json:"details,omitempty"`
}
```

## Frontend Architecture (JavaScript)

### Core Components

#### 1. Main Application (`main.js`)
- **Class**: `OCDApp`
- **Responsibilities**:
  - DOM element management
  - Event handling
  - WebSocket communication
  - State management
  - UI updates

#### 2. Progress Manager (`progress-manager.js`)
- **Class**: `ProgressManager`
- **Features**:
  - Real-time progress tracking
  - Stage-based progress updates
  - Service-specific progress items
  - Progress bar management

#### 3. History Manager (`history-manager.js`)
- **Class**: `HistoryManager`
- **Features**:
  - Folder path history storage
  - Dropdown interface
  - Local storage persistence
  - History item management

#### 4. Constants (`constants.js`)
```javascript
export const CONFIG = {
    HISTORY_KEY: 'ocd-folder-history',
    MAX_HISTORY_ITEMS: 10,
    WEBSOCKET_TIMEOUT: 30000,
    STATUS_DISPLAY_TIME: 5000
};

export const STAGE_LABELS = {
    prerequisites: 'Connection Checks & Prerequisites',
    settings: 'Maven Settings XML Update',
    build: 'Building Microservices',
    deploy: 'Docker Image Creation',
    patch: 'Kubernetes Deployment'
};
```

#### 5. Utilities (`utils.js`)
- **Functions**:
  - `createWebSocketUrl()` - WebSocket URL generation
  - `generateTimestamp()` - Timestamp formatting
  - `downloadTextFile()` - File download functionality
  - `ansiToHtml()` - ANSI color code conversion
  - `cleanAnsiEscapes()` - ANSI escape sequence cleaning

### UI Features
- **Theme Toggle**: Dark/light mode switching
- **Folder Selection**: Browse dialog with history
- **Real-time Progress**: WebSocket-based progress updates
- **Output Window**: Collapsible command output display
- **Save Functionality**: Download deployment logs
- **Responsive Design**: Modern CSS with animations

### UI Layout Reference
**Note**: For UI modifications, refer to screenshot: `OCD_UI_SCREENSHOT.png`
- **Main Interface**: Clean, modern web interface with dark/light theme toggle
- **Folder Input**: Text input with browse button and history dropdown
- **Deploy Button**: Primary action button (disabled until valid path selected)
- **Progress Section**: Real-time progress bar and stage-based progress overview
- **Output Window**: Collapsible terminal-style output with save functionality
- **Status Messages**: Temporary notifications for user feedback

## Deployment Scripts

### Overview
The OCD tool now supports multiple project types with dedicated deployment scripts:

#### 1. ATT Projects (`OCD.sh`)
The original deployment script for ATT projects that:
1. Detects changed microservices using Git
2. Updates Maven settings
3. Builds changed services with Maven
4. Creates Docker images
5. Deploys to Kubernetes

#### 2. Customization Projects (`OCD-customization.sh`)
A specialized deployment script for customization projects that:
1. Detects changed services in `app/backend/` directories
2. Builds changed services with Maven
3. Always builds `app/metadata`
4. Always builds `dockers/customization-jars`
5. Deploys using project-specific patch commands (TBD)

### Shared Functions (`deploy-scripts/scripts/shared/`)
Both scripts use shared utility functions to eliminate code duplication:

#### `deploy-scripts/scripts/shared/utils.sh`
- Environment detection (WSL/Windows/Linux)
- Colored output and logging
- Git utilities (changed file detection)
- Prerequisites checking
- Connection validation

#### `deploy-scripts/scripts/shared/maven.sh`
- Maven settings management
- Cross-platform Maven build functions
- Path conversion utilities

### Key Features
- **Smart Change Detection**: Uses Git status to identify modified services
- **Maven Integration**: Automatic Maven settings.xml updates
- **Docker Support**: Image building and registry pushing
- **Kubernetes Deployment**: Direct kubectl deployment
- **Cross-platform**: Works on Windows (WSL), Linux, and macOS
- **Verbose Logging**: Detailed output for debugging
- **Error Handling**: Comprehensive error checking and reporting

### Deployment Stages
1. **Prerequisites**: Connection checks and environment validation
2. **Settings**: Maven settings.xml configuration
3. **Build**: Maven compilation of changed services
4. **Deploy**: Docker image creation and pushing
5. **Patch**: Kubernetes deployment and rollout

### Command Line Options
Both scripts support the same command-line interface:
- `-n, --namespace` - Kubernetes namespace (default: dop)
- `--skip-build` - Skip build phase
- `--skip-deploy` - Skip deploy phase
- `--force` - Run even if no changes detected
- `--confirm` - Prompt for confirmation
- `-v, --verbose` - Show detailed output
- `-h, --help` - Show help

## Multi-Project Support

### Project Type Detection
The OCD tool automatically detects project type based on the repository path:
- **ATT Projects**: Standard microservice projects (default)
- **Customization Projects**: Projects containing "customization" in the path

### Script Selection
- **ATT Projects**: Uses `OCD.sh` with microservice detection and deployment
- **Customization Projects**: Uses `OCD-customization.sh` with service-specific build/deploy flow

### Backward Compatibility
- Existing ATT projects continue to work exactly as before
- No changes required for current deployments
- New customization projects use the dedicated script

## Cross-Platform Support

### Windows
- **Requirement**: WSL (Windows Subsystem for Linux)
- **Path Conversion**: Windows paths converted to WSL paths
- **User Configuration**: Configurable WSL user (default: k8s)
- **Command Execution**: Uses `wsl` command with bash

### Linux
- **Native Support**: Direct bash execution
- **Environment**: Full Linux environment access
- **Permissions**: Automatic script permission setting

### macOS
- **Native Support**: Direct bash execution
- **Environment**: Unix-like environment
- **Compatibility**: Full compatibility with Linux commands

## Security Features

### Input Validation
- **Path Sanitization**: Prevents path traversal attacks
- **Git Repository Validation**: Ensures selected folder is a Git repo
- **Command Injection Prevention**: Secure command building

### WebSocket Security
- **Origin Validation**: Configurable CORS origins
- **Connection Management**: Proper connection cleanup
- **Error Handling**: Graceful error responses

### File System Security
- **Temporary File Management**: Secure temp file creation and cleanup
- **Permission Handling**: Proper file permissions
- **Path Validation**: Comprehensive path checking

## Dependencies

### Go Dependencies
```go
require github.com/gorilla/websocket v1.5.3
```

### System Dependencies
- **Git**: Version control system
- **Maven**: Java build tool
- **kubectl**: Kubernetes command-line tool
- **Docker**: Container platform
- **WSL**: Windows Subsystem for Linux (Windows only)

## Build Process

### Single Platform (outputs to repo-level dist/)
```bash
# Windows
(cd app && go build -ldflags="-s -w -X app/internal/version.Version=$(git describe --tags --abbrev=0 2>/dev/null || echo dev)" -o ../dist/OCD.exe ./cmd/ocd-gui)

# Linux x64
(cd app && GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../dist/OCD-Tool-Linux-x64 ./cmd/ocd-gui)

# macOS Apple Silicon
(cd app && GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../dist/OCD-Tool-macOS-AppleSilicon ./cmd/ocd-gui)
```

### All Platforms
```bash
chmod +x build/build-all-executables.sh
DIST_DIR=dist ./build/build-all-executables.sh
```
### CI Release Flow
- Push a tag (e.g., `v2.0.1`)
- Pipeline builds all targets with `-ldflags` injecting version/commit/date
- A Release is created with binaries and `SHA256SUMS.txt`

## Usage Workflow

1. **Launch Application**: Run the platform-specific executable
2. **Select Repository**: Use browse button or type path to Git repository
3. **Project Detection**: System automatically detects project type (ATT vs Customization)
4. **Script Selection**: Appropriate deployment script is automatically chosen
5. **Validation**: System validates Git repository and detects changes
6. **Deployment**: Click "Deploy Changes" to start deployment
7. **Monitoring**: Real-time progress tracking via WebSocket
8. **Completion**: View results and optionally save logs

## Error Handling

### Common Issues
- **WSL Not Available**: Windows users must install WSL
- **No Changes Detected**: Use --force flag or ensure files are staged
- **Maven Build Failures**: Check Maven settings and network connectivity
- **Kubernetes Connection**: Verify kubectl configuration and cluster access

### Error Recovery
- **Automatic Retry**: Built-in retry mechanisms for network issues
- **Graceful Degradation**: Partial deployment support
- **Detailed Logging**: Comprehensive error reporting
- **User Feedback**: Clear error messages in UI

## Performance Considerations

### Optimization Features
- **Embedded Assets**: Web files embedded in binary for faster startup
- **WebSocket Streaming**: Real-time updates without polling
- **Command Timeout**: Configurable timeout to prevent hanging
- **Memory Management**: Proper cleanup of temporary resources

### Scalability
- **Single Instance**: Designed for single-user deployment
- **Resource Usage**: Minimal memory and CPU footprint
- **Network Efficiency**: WebSocket reduces HTTP overhead

## Development Guidelines

### Code Organization
- **Modular Design**: Clear separation of concerns
- **Error Handling**: Comprehensive error checking
- **Logging**: Structured logging throughout
- **Documentation**: Inline comments and clear naming

### Testing Considerations
- **Cross-platform Testing**: Test on all supported platforms
- **WebSocket Testing**: Verify real-time communication
- **Error Scenarios**: Test various failure conditions
- **UI Testing**: Verify responsive design and interactions

## Future Enhancements

### Potential Improvements
- **Multi-user Support**: Multiple concurrent deployments
- **Plugin System**: Extensible deployment strategies
- **Advanced Monitoring**: Integration with monitoring tools
- **Configuration UI**: Web-based configuration management
- **Deployment Templates**: Predefined deployment patterns
- **Additional Project Types**: Support for more project structures
- **Customization Deployment**: Complete implementation of customization patch commands

### Technical Debt
- **Code Documentation**: Additional inline documentation
- **Test Coverage**: Unit and integration tests
- **Error Handling**: More specific error types
- **Performance**: Additional optimization opportunities
- **Shared Function Testing**: Comprehensive testing of shared utilities

## Troubleshooting Guide

### Windows Issues
- Ensure WSL is installed and configured
- Check WSL can access project folder
- Verify Maven settings.xml configuration

### Linux/macOS Issues
- Ensure all prerequisites are installed
- Check file permissions for executable
- Verify kubectl context configuration

### General Issues
- Check network connectivity for Maven and Docker
- Verify Kubernetes cluster accessibility
- Review detailed logs in output window
- Ensure Git repository is properly configured

## Related Context Files

### Project-Specific Contexts
- **`_att_context.md`**: Documents the ATT project structure and current OCD tool support
- **`_customization_context.md`**: Documents the customization project structure and planned OCD tool integration

### Usage
- For ATT project deployments, refer to `_att_context.md`
- For customization project development, refer to `_customization_context.md`
- For general OCD tool architecture, refer to this main context file

---

This context file provides a comprehensive overview of the OCD project, enabling any AI to understand the complete architecture, functionality, and implementation details without needing to explore the codebase further. 