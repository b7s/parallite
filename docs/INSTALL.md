# Installation Guide

## Prerequisites

### Required

- **Go 1.21 or higher**: [Download Go](https://golang.org/dl/)
- **PHP 8.3 or higher**: With CLI support

### Verify Prerequisites

```bash
# Check Go version
go version

# Check PHP version
php --version
```

## Installation Steps

### 1. Clone or Extract the Repository

```bash
cd /path/to/parallite-go-daemon
```

### 2. Install Go Dependencies

```bash
go mod download
```

This will download all required Go dependencies (if any).

### 3. Build the Binary

#### Option A: Using go build
```bash
go build -o parallite main.go
```

#### Option B: Using the build script
```bash
chmod +x build.sh
./build.sh
```

#### Option C: Using Make
```bash
make build
```

### 4. Verify the Build

```bash
./parallite --version
```

You should see the version information.

### 5. Set Up PHP Integration

For PHP client and worker setup, install the PHP package:

```bash
composer require b7s/parallite-php
```

The PHP package includes:
- Worker implementation for executing closures
- Client library for submitting tasks
- Complete examples and documentation

See the [parallite-php repository](https://github.com/b7s/parallite-php) for detailed setup instructions.

## Configuration

### 1. Review Default Configuration

The default `parallite.json` is already configured with sensible defaults:

```json
{
  "fixed_workers": 1,
  "prefix_name": "work",
  "timeout_ms": 60000,
  "socket": "",
  "fail_mode": "continue"
}
```

### 2. Customize for Your Environment

Copy an example configuration:

```bash
# For development
cp examples/parallite.development.json parallite.json

# For production
cp examples/parallite.production.json parallite.json
```

Edit `parallite.json` to match your requirements.

## Running

### Development Mode

```bash
./parallite
```

The daemon will:
- Start 1 persistent worker (default)
- Listen on `/tmp/parallite.sock` (Linux/macOS) or TCP port 9876 (Windows)
- Use persistent SQLite database
- Log to stderr

### Production Mode

```bash
./parallite --config parallite.json 2>&1 | tee parallite.log
```

Or use the systemd service (Linux):

```bash
# Copy service file
sudo cp examples/parallite.service /etc/systemd/system/

# Edit paths in service file
sudo nano /etc/systemd/system/parallite.service

# Reload systemd
sudo systemctl daemon-reload

# Start service
sudo systemctl start parallite

# Enable on boot
sudo systemctl enable parallite

# Check status
sudo systemctl status parallite
```

## Testing

### 1. Start the Daemon

```bash
./parallite
```

### 2. Run the Test Client

In another terminal:

```bash
php examples/test_client.php
```

You should see output like:

```
Connected to Parallite daemon at /tmp/parallite.sock
Sent task task-xxxxx
Received response for task task-xxxxx
Array
(
    [task_id] => task-xxxxx
    [ok] => 1
    [result] => Hello from Parallite!
)
...
```

## Troubleshooting

### "go: command not found"

Install Go from https://golang.org/dl/

### "php: command not found"

Install PHP:
```bash
# Ubuntu/Debian
sudo apt-get install php-cli

# macOS
brew install php

# Windows
# Download from https://windows.php.net/download/
```

### "Failed to connect to socket"

1. Check if daemon is running: `ps aux | grep parallite`
2. Check socket exists: `ls -l /tmp/parallite.sock`
3. Check socket permissions
4. Try specifying a different socket: `./parallite --socket /tmp/test.sock`

### "Address already in use"

Another instance is running:
```bash
# Find the process
pgrep parallite

# Stop it
kill $(pgrep parallite)
```

### "Worker not starting"

1. Check PHP is in PATH: `which php`
2. Check `php/worker.php` exists
3. Check PHP error logs
4. Run worker manually: `php php/worker.php`

### Build Errors

If you get build errors:

```bash
# Clean and rebuild
go clean
go mod tidy
go build -o parallite main.go
```

## Cross-Platform Builds

### Build for All Platforms

```bash
./build.sh --cross-compile
```

This creates:
- `parallite-linux-amd64`
- `parallite-linux-arm64`
- `parallite-darwin-amd64` (macOS Intel)
- `parallite-darwin-arm64` (macOS Apple Silicon)
- `parallite-windows-amd64.exe`

### Manual Cross-Compilation

```bash
# Linux
GOOS=linux GOARCH=amd64 go build -o parallite-linux main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o parallite-macos main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o parallite.exe main.go
```

## Deployment

### Linux (systemd)

1. Copy binary to `/opt/parallite/`
2. Copy config to `/opt/parallite/parallite.json`
3. Copy PHP worker to `/opt/parallite/php/worker.php`
4. Install systemd service (see above)

### Docker (Optional)

Create a `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o parallite main.go

FROM php:8.2-cli-alpine
WORKDIR /app
COPY --from=builder /app/parallite .
COPY php/ ./php/
COPY parallite.json .
CMD ["./parallite"]
```

Build and run:
```bash
docker build -t parallite .
docker run -v /tmp:/tmp parallite
```

## Upgrading

1. Stop the daemon
2. Backup your configuration and database
3. Replace the binary
4. Restart the daemon

```bash
# Stop
kill $(pgrep parallite)

# Backup
cp parallite.json parallite.json.bak
cp parallite.sqlite parallite.sqlite.bak

# Replace binary
cp /path/to/new/parallite .

# Start
./parallite
```

## Uninstallation

```bash
# Stop daemon
kill $(pgrep parallite)

# Remove systemd service (if installed)
sudo systemctl stop parallite
sudo systemctl disable parallite
sudo rm /etc/systemd/system/parallite.service
sudo systemctl daemon-reload

# Remove files
rm -rf /opt/parallite
rm /tmp/parallite.sock
```

## Next Steps

- Read [QUICKSTART.md](QUICKSTART.md) for usage examples
- Review [README.md](README.md) for detailed documentation
- Check [PROJECT_SUMMARY.md](PROJECT_SUMMARY.md) for technical details
- Integrate with your PHP package

## Support

For issues and questions:
- Check the troubleshooting section above
- Review the documentation
- Check the examples directory
- Open an issue on the project repository
