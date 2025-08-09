# OCD Application Icons

This directory contains custom icons for the OCD application binaries.

## Required Files

- `ocd.ico` - Windows icon (ICO format with multiple sizes: 16, 32, 48, 64, 128, 256px)
- `ocd.icns` - macOS icon (ICNS format with Apple's standard icon sizes)
- `ocd.png` - Linux icon (PNG format, 256x256 or 512x512 pixels)

## Tools to Create Icons

### Windows ICO
- Use online converters like favicon.io or convertio.co
- Or use ImageMagick: `convert ocd.png -define icon:auto-resize=256,128,64,48,32,16 ocd.ico`

### macOS ICNS
- Use `iconutil` on macOS: Create iconset folder with multiple PNG sizes, then `iconutil -c icns icon.iconset`
- Or use online converters

### Linux PNG
- Simply use a high-quality PNG file (256x256 or 512x512 pixels)

## Implementation

Icons are embedded during build using platform-specific tools:
- Windows: Uses `goversioninfo` or `rsrc` to embed ICO files
- macOS: Icons are included in app bundle metadata
- Linux: Icons are embedded as binary data or used with desktop files
