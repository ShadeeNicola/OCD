# OCD - One Click Deployer

A GUI-based deployment tool that simplifies the deployment process with real-time progress tracking and an intuitive web interface.

## Quick Start - Download & Run

### For Windows Users
1. Download `OCD.exe`
2. Double-click to run
3. Your browser will open automatically
4. **Requirements**: WSL (Windows Subsystem for Linux) must be installed

### For Linux Users
1. Download `OCD-Tool-Linux`
2. Make it executable:
chmod +x OCD-Tool-Linux
3. Run the tool:
./OCD-Tool-Linux
4. Your browser will open automatically

## Features

- **Smart Change Detection**: Automatically detects which microservices have changed based on Git status
- **One-Click Deployment**: Deploy multiple microservices with a single click
- **Real-time Progress**: WebSocket-based progress updates with detailed logging
- **Cross-Platform**: Works on Windows (with WSL) and Linux
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
