# OCD - One Click Deployer

A GUI-based deployment tool that simplifies the deployment process with real-time progress tracking and an intuitive web interface.

## Quick Start - Download & Run

### For Windows Users
1. Download from Releases: `/-/releases/latest` (OCD.exe)
2. Double-click to run
3. Your browser will open automatically
4. **Requirements**: WSL (Windows Subsystem for Linux) must be installed

### For macOS Users
1. Download from Releases: `/-/releases/latest` (choose `OCD-Tool-macOS-Intel` or `OCD-Tool-macOS-AppleSilicon`).
2. Make it executable: `chmod +x dist/<downloaded-file>`
3. Run: `./dist/<downloaded-file>`

### For Linux Users
1. Download from Releases: `/-/releases/latest` (choose your architecture: `OCD-Tool-Linux-x64`, `-x86`, `-ARM64`, `-ARM`).
2. Make it executable: `chmod +x dist/<downloaded-file>`
3. Run: `./dist/<downloaded-file>`

**Note**: Your browser will open automatically on all platforms.

Verify downloads: each release also includes a `SHA256SUMS.txt`. On Linux/macOS, you can verify with:
```bash
sha256sum -c SHA256SUMS.txt | grep OCD-Tool-Linux-x64
```

## Features

- **Smart Change Detection**: Automatically detects which microservices have changed based on Git status
- **One-Click Deployment**: Deploy multiple microservices with a single click
- **Real-time Progress**: WebSocket-based progress updates with detailed logging
- **Cross-Platform**: Works on Windows (with WSL), Linux, and macOS
- **Maven Integration**: Automatically updates Docker settings and builds with Maven
- **Kubernetes Integration**: Direct deployment to Kubernetes clusters via kubectl
- **Web-based GUI**: Clean, modern web interface for easy interaction

## Prerequisites

- **Git** - Version control system
- **Maven** - Java build tool
- **kubectl** - Kubernetes command-line tool
- **Docker** - Container platform
- **WSL** (Windows only) - Windows Subsystem for Linux

## Installation

1. Clone the repository:
   git clone <your-repo-url>
   cd OCD

2. Build the application (outputs to repo-level `dist/`):
   (cd app && go build -ldflags="-s -w" -o ../dist/OCD.exe ./cmd/ocd-gui)

## Usage

1. Launch the OCD tool (executable will vary by platform)
2. Select your Git repository folder using the Browse button
3. The tool will automatically detect changed microservices
4. Click "Deploy Changes" to start the deployment process
5. Monitor real-time progress in the web interface

## Development

### Building for All Platforms

Use the provided root build script (for contributors):
chmod +x build/build-all-executables.sh
DIST_DIR=dist ./build/build-all-executables.sh

Or build manually for specific platforms (outputs to repo-level `dist/`):
# Windows
(cd app && go build -ldflags="-s -w" -o ../dist/OCD.exe ./cmd/ocd-gui)

# Linux x64
(cd app && GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../dist/OCD-Tool-Linux-x64 ./cmd/ocd-gui)

# macOS Apple Silicon
(cd app && GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../dist/OCD-Tool-macOS-AppleSilicon ./cmd/ocd-gui)

### Project Structure (Go module lives in `app/`)

- cmd/ocd-gui/main.go - Application entry point
- internal/http - HTTP and WebSocket handlers
- internal/executor - Script execution and orchestration
- internal/config - Configuration loading
- internal/logging - Logger wrapper
- internal/security - Validation and path utilities
- internal/progress - Progress parsing and DTOs
- internal/ui - Embedded web assets and folder dialog
- web/ - Frontend assets (HTML, CSS, JavaScript)
- deploy-scripts/ - Embedded deployment scripts module
  - scripts/OCD.sh, scripts/OCD-customization.sh
  - scripts/shared/ (utils.sh, maven.sh)
- scripts/ - Build utilities (root)
  - build/build-all-executables.sh

## Troubleshooting

### Windows Issues
- Ensure WSL is installed and configured
- Check that WSL can access your project folder
- Verify Maven settings.xml is properly configured

### Linux/macOS Issues
- Ensure all prerequisites are installed
- Check file permissions for the executable
- Verify kubectl context is properly configured

### Common Issues
- "No changes detected": Use --force flag or ensure files are properly staged in Git
- Maven build failures: Check Maven settings and network connectivity
- Kubernetes connection issues: Verify kubectl configuration and cluster access


## Contributing
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test thoroughly
5. Submit a pull request

## Support

For issues and questions, please create an issue in the repository or contact the developer directly.