# OCD - One Click Deployer

A GUI-based deployment tool that simplifies the deployment process with real-time progress tracking and an intuitive web interface.

## Quick Start - Download & Run

### For Windows Users
1. Download `executables/OCD.exe`
2. Double-click to run
3. Your browser will open automatically
4. **Requirements**: WSL (Windows Subsystem for Linux) must be installed

### For Linux Users
Choose the appropriate version for your system:

#### Linux x64 (Intel/AMD 64-bit) - Most Common
1. Download `executables/OCD-Tool-Linux-x64`
2. Make it executable: `chmod +x executables/OCD-Tool-Linux-x64`
3. Run the tool: `./executables/OCD-Tool-Linux-x64`

#### Linux x86 (32-bit)
1. Download `executables/OCD-Tool-Linux-x86`
2. Make it executable: `chmod +x executables/OCD-Tool-Linux-x86`
3. Run the tool: `./executables/OCD-Tool-Linux-x86`

#### Linux ARM64 (64-bit ARM)
1. Download `executables/OCD-Tool-Linux-ARM64`
2. Make it executable: `chmod +x executables/OCD-Tool-Linux-ARM64`
3. Run the tool: `./executables/OCD-Tool-Linux-ARM64`

#### Linux ARM (32-bit ARM)
1. Download `executables/OCD-Tool-Linux-ARM`
2. Make it executable: `chmod +x executables/OCD-Tool-Linux-ARM`
3. Run the tool: `./executables/OCD-Tool-Linux-ARM`

### For macOS Users

#### macOS Intel (x64)
1. Download `executables/OCD-Tool-macOS-Intel`
2. Make it executable: `chmod +x executables/OCD-Tool-macOS-Intel`
3. Run the tool: `./executables/OCD-Tool-macOS-Intel`

#### macOS Apple Silicon (M1/M2)
1. Download `executables/OCD-Tool-macOS-AppleSilicon`
2. Make it executable: `chmod +x executables/OCD-Tool-macOS-AppleSilicon`
3. Run the tool: `./executables/OCD-Tool-macOS-AppleSilicon`

**Note**: Your browser will open automatically on all platforms.

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

2. Build the application:
   go build -ldflags="-s -w" -o executables/OCD.exe .

## Usage

1. Launch the OCD tool (executable will vary by platform)
2. Select your Git repository folder using the Browse button
3. The tool will automatically detect changed microservices
4. Click "Deploy Changes" to start the deployment process
5. Monitor real-time progress in the web interface

## Development

### Building for All Platforms

Use the provided build script:
chmod +x build-all-executables.sh
./build-all-executables.sh

Or build manually for specific platforms:
# Windows
go build -ldflags="-s -w" -o executables/OCD.exe .

# Linux x64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o executables/OCD-Tool-Linux-x64 .

# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o OCD-Tool-macOS-AppleSilicon .

### Project Structure

- main.go - Application entry point
- handlers.go - HTTP request handlers
- websocket.go - WebSocket communication
- ocd_runner.go - Core deployment logic
- web/ - Frontend assets (HTML, CSS, JavaScript)
- OCD.sh - Shell script for deployment operations

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