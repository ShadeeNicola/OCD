# Vendored Tools

This directory contains pre-built tools to avoid network dependencies during CI builds.

## goversioninfo

Used for embedding Windows icons and version information into Go executables.

### Binaries:
- `goversioninfo-linux` - Linux binary for GitLab CI
- `goversioninfo.exe` - Windows binary for local builds

### Updating:
To update goversioninfo, disconnect VPN and run:

```bash
# Clone source
git clone https://github.com/josephspurrier/goversioninfo.git
cd goversioninfo/cmd/goversioninfo

# Build for CI (Linux)
GOOS=linux GOARCH=amd64 go build -o ../../tools/goversioninfo-linux

# Build for local (Windows) 
GOOS=windows GOARCH=amd64 go build -o ../../tools/goversioninfo.exe

# Clean up
cd ../../
rm -rf goversioninfo/
```

### Source:
- Repository: https://github.com/josephspurrier/goversioninfo
- License: MIT
