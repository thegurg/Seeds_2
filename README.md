# File Transfer Application

A high-speed, cross-platform file transfer application for local networks using Go, mDNS discovery, and HTTP streaming.

## Features

- **Automatic Discovery**: Uses mDNS (`_projectzero._tcp`) to automatically discover peers on the network
- **Memory Efficient**: Streams files using `io.CopyBuffer` with a 32KB buffer - handles 50GB+ files using only ~20MB RAM
- **Resume Support**: Downloads support HTTP range requests for resuming interrupted transfers
- **Cross-Platform**: Works on Windows, Linux, and Android
- **Web UI**: Modern drag-and-drop interface with progress tracking

## Directory Structure

```
./shared       - Files available for download
./downloads    - Incoming files are saved here
```

## Usage

```bash
# Run with auto-detected port
./filetransfer

# Run with specific port
./filetransfer -port 8080
```

Then open your browser to `http://localhost:<port>` (port will be displayed in console).

## Building

### Linux (AMD64)

```bash
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o filetransfer-linux-amd64 .
```

### Windows (AMD64)

```bash
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o filetransfer-windows-amd64.exe .
```

### Android

Building for Android requires the `gomobile` tool:

1. Install gomobile:
```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

2. Build the APK:
```bash
gomobile build -target=android -o filetransfer.apk .
```

**Note**: The application will request external storage permissions at runtime on Android. Downloads are saved to the standard Downloads folder (`/sdcard/Download/FileTransfer/`).

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web UI |
| `/peers` | GET | JSON list of discovered peers |
| `/files` | GET | JSON list of files in `./shared` |
| `/upload` | POST | Upload file (streamed to `./downloads`) |
| `/download/{filename}` | GET | Download file with range support |

## Architecture

- **mDNS Discovery**: Registers as `_projectzero._tcp` and scans for other nodes
- **HTTP Server**: Serves UI and handles file transfers
- **Memory Management**: Uses `io.CopyBuffer` for constant memory usage regardless of file size
- **Concurrency**: Supports multiple simultaneous uploads/downloads via goroutines

## Requirements

- Go 1.21 or later
- For Android: Android SDK and gomobile

## License

MIT
