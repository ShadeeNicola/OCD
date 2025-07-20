# OCD - One Click Deployer

A GUI-based deployment tool that automatically detects changed microservices in Git repositories and deploys them to Kubernetes clusters with real-time progress tracking.

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
```bash
git clone <your-repo-url>
cd OCD