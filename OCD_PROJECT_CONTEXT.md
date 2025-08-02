# OCD (One Click Deployer) - Complete Project Context

## Project Overview

OCD (One Click Deployer) is a GUI-based deployment tool that simplifies the deployment process for microservices with real-time progress tracking and an intuitive web interface. It's designed to automatically detect changed microservices based on Git status and deploy only what's necessary.

## Architecture Overview

### Technology Stack
- **Backend**: Go (Golang) 1.24.5
- **Frontend**: Vanilla JavaScript (ES6 modules)
- **Communication**: WebSocket for real-time updates
- **Deployment**: Shell script (OCD.sh) with Maven, Docker, and Kubernetes integration
- **Cross-platform**: Windows (WSL), Linux, macOS support

### Project Structure
```
OCD/
├── ocd-gui/                    # Main application directory
│   ├── main.go                # Application entry point
│   ├── handlers.go            # HTTP request handlers
│   ├── websocket.go           # WebSocket communication
│   ├── ocd_runner.go          # Core deployment logic
│   ├── command_executor.go    # Command execution and streaming
│   ├── config.go              # Configuration management
│   ├── types.go               # Data structures and types
│   ├── file_dialog.go         # File dialog handling
│   ├── logger.go              # Logging functionality
│   ├── security.go            # Security utilities
│   ├── progress_parser.go     # Progress parsing from output
│   ├── assets.go              # Embedded web assets
│   ├── OCD.sh                 # Main deployment shell script
│   ├── go.mod                 # Go dependencies
│   ├── go.sum                 # Go dependency checksums
│   └── web/                   # Frontend assets
│       ├── index.html         # Main HTML interface
│       ├── styles.css         # Styling
│       └── js/                # JavaScript modules
│           ├── main.js        # Main application logic
│           ├── constants.js   # Application constants
│           ├── utils.js       # Utility functions
│           ├── history-manager.js    # Folder history management
│           └── progress-manager.js   # Progress tracking
└── README.md                  # Project documentation
```

## Backend Architecture (Go)

### Core Components

#### 1. Main Application (`main.go`)
- **Purpose**: Application entry point and server initialization
- **Key Features**:
  - Embedded web filesystem loading
  - HTTP server setup on port 2111 (configurable)
  - Automatic browser opening
  - Cross-platform path handling
  - Route configuration

#### 2. HTTP Handlers (`handlers.go`)
- **Endpoints**:
  - `GET /api/browse` - File dialog for folder selection
  - `POST /api/deploy` - Traditional deployment endpoint
  - `GET /api/health` - Health check endpoint
  - `GET /ws/deploy` - WebSocket deployment endpoint

#### 3. WebSocket Communication (`websocket.go`)
- **Purpose**: Real-time communication for deployment progress
- **Features**:
  - Origin validation for security
  - Thread-safe message writing
  - Connection management
  - Error handling

#### 4. Deployment Runner (`ocd_runner.go`)
- **Purpose**: Core deployment orchestration
- **Key Functions**:
  - Cross-platform command building (Windows/WSL, Linux, macOS)
  - Temporary script file creation
  - Real-time output streaming
  - Progress parsing and WebSocket communication
  - Path conversion for WSL

#### 5. Command Executor (`command_executor.go`)
- **Purpose**: Secure command execution and streaming
- **Features**:
  - Path validation and sanitization
  - Command timeout handling (30 minutes default)
  - Real-time stdout/stderr streaming
  - Error handling and logging

#### 6. Configuration (`config.go`)
- **Configuration Options**:
  - `OCD_PORT` - Server port (default: 2111)
  - `OCD_WSL_USER` - WSL username (default: k8s)
  - `OCD_SCRIPT_NAME` - Script name (default: OCD.sh)
  - `OCD_ALLOWED_ORIGINS` - CORS origins (default: localhost,127.0.0.1)
  - `OCD_COMMAND_TIMEOUT` - Command timeout in seconds (default: 1800)

### Data Structures (`types.go`)

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

## Deployment Script (OCD.sh)

### Overview
The `OCD.sh` script is the core deployment engine that:
1. Detects changed microservices using Git
2. Updates Maven settings
3. Builds changed services with Maven
4. Creates Docker images
5. Deploys to Kubernetes

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
- `-n, --namespace` - Kubernetes namespace (default: dop)
- `--skip-build` - Skip build phase
- `--skip-deploy` - Skip deploy phase
- `--force` - Run even if no changes detected
- `--confirm` - Prompt for confirmation
- `-v, --verbose` - Show detailed output
- `-h, --help` - Show help

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

### Single Platform
```bash
# Windows
go build -ldflags="-s -w" -o OCD.exe .

# Linux x64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o OCD-Tool-Linux-x64 .

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o OCD-Tool-macOS-AppleSilicon .
```

### All Platforms
```bash
chmod +x build-all-executables.sh
./build-all-executables.sh
```

## Usage Workflow

1. **Launch Application**: Run the platform-specific executable
2. **Select Repository**: Use browse button or type path to Git repository
3. **Validation**: System validates Git repository and detects changes
4. **Deployment**: Click "Deploy Changes" to start deployment
5. **Monitoring**: Real-time progress tracking via WebSocket
6. **Completion**: View results and optionally save logs

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

### Technical Debt
- **Code Documentation**: Additional inline documentation
- **Test Coverage**: Unit and integration tests
- **Error Handling**: More specific error types
- **Performance**: Additional optimization opportunities

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

This context file provides a comprehensive overview of the OCD project, enabling any AI to understand the complete architecture, functionality, and implementation details without needing to explore the codebase further. 