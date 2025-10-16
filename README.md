<div align="center">

<img src="art/parallite-logo.webp" alt="Parallite Logo" width="128">
<h1>Parallite</h1>

</div>

> High-performance Go daemon for parallel execution of PHP closures

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](https://github.com/b7s/parallite)

Parallite is a robust orchestrator that manages persistent PHP worker processes, enabling true parallel execution of PHP closures with minimal overhead. Built in Go for maximum performance and reliability.

> **This repository contains the Go daemon only.** For PHP client integration, see [parallite-php](https://github.com/b7s/parallite-php).

## ✨ Features

- 🚀 **High Performance** - Go-powered orchestration with persistent worker pools
- 🔄 **True Parallelism** - Execute multiple PHP closures simultaneously
- 🛡️ **Fault Tolerant** - Auto-restart workers on crash, configurable error handling
- 💾 **In-Memory Task Cache** - Fast task status tracking with automatic cleanup
- ⏱️ **Timeout Control** - Per-task execution timeouts
- 🌐 **Cross-Platform** - Works on Linux, macOS, and Windows
- 🔌 **Efficient IPC** - Unix sockets (Linux/macOS) or TCP (Windows)
- 📊 **Resource Management** - Configurable worker pools and payload limits

## 🚀 Quick Start

```bash
# 1. Build the daemon
go build -o parallite main.go

# 2. Create configuration
cat > parallite.json << EOF
{
  "fixed_workers": 4,
  "prefix_name": "worker",
  "timeout_ms": 60000,
  "fail_mode": "continue",
  "max_payload_bytes": 10485760
}
EOF

# 3. Start the daemon
./parallite
```

The daemon will start 4 persistent PHP workers and listen for tasks on `/tmp/parallite.sock` (Linux/macOS) or TCP port 9876 (Windows).

## 📦 Installation

### From Source

```bash
git clone https://github.com/b7s/parallite.git
cd parallite
go build -o parallite main.go
```

### Using Make

```bash
make build          # Build for current platform
make install        # Install dependencies
make cross-compile  # Build for all platforms
```

See [INSTALL.md](INSTALL.md) for detailed installation instructions including systemd service setup.

## 🛠️ Development

### Creating a Release

Use the interactive release command to automate the entire release process:

```bash
make release
```

This command will:
1. ✅ Show current version
2. ✅ Ask for new version (format: `v0.0.0`)
3. ✅ Ask for release message
4. ✅ Build binary with version embedded
5. ✅ Commit changes to git
6. ✅ Create annotated git tag
7. ✅ Push to remote repository
8. ✅ Trigger GitHub Actions to build binaries for all platforms

**Example:**
```bash
$ make release
🚀 Parallite Release Process

Current version in code: Parallite v0.1.2

Enter new version (format: v0.0.0): v0.2.0

📝 Enter release message (press Ctrl+D when done):
Release v0.2.0 - New features

- Add worker monitoring
- Improve performance
^D

📋 Summary:
  Version: v0.2.0
  Message: Release v0.2.0 - New features...

Continue with release? [y/N]: y

🎉 Release v0.2.0 completed successfully!
```

## 🔗 PHP Integration

This repository contains only the Go daemon. For PHP client integration, worker implementation, and usage examples, see:

### **[parallite-php](https://github.com/b7s/parallite-php)** - PHP Client Library

The PHP package provides:

- ✅ **Ready-to-use Composer package** - Install via `composer require b7s/parallite-php`
- ✅ **Worker implementation** - Pre-built worker for executing closures
- ✅ **Client library** - Easy-to-use API for submitting tasks
- ✅ **Complete examples** - Standalone scripts and framework integration
- ✅ **Full documentation** - Setup guides and API reference

**Quick Start with PHP:**

```bash
# Install PHP package
composer require b7s/parallite-php

# Start the daemon
./parallite

# Use in your PHP code
use B7s\Parallite\Client;

$client = new Client();
$result = $client->run(fn() => heavy_computation());
```

See the [parallite-php repository](https://github.com/b7s/parallite-php) for complete documentation and examples.

## ⚙️ Configuration

Create a `parallite.json` file in the same directory as the binary:

```json
{
  "fixed_workers": 1,
  "prefix_name": "work",
  "timeout_ms": 60000,
  "socket": "",
  "fail_mode": "continue",
  "max_payload_bytes": 10485760
}
```

### Configuration Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fixed_workers` | int | 1 | Number of persistent PHP workers |
| `prefix_name` | string | "work" | Prefix for worker names (e.g., "work-1") |
| `timeout_ms` | int | 60000 | Task execution timeout in milliseconds |
| `socket` | string | OS-specific | IPC endpoint (auto-detected if empty) |
| `fail_mode` | string | "continue" | Error handling: "continue" or "stop_all" |
| `max_payload_bytes` | int | 10485760 | Maximum payload size in bytes (10MB) |

**Important Notes:**

- **Task Result Retention**: Completed task results are kept in memory for `timeout_ms + 5 minutes`. This ensures results remain available even if the client is slightly delayed in retrieving them. For example, with a 60-second timeout, results are retained for 6 minutes.

- **Automatic Cleanup**: The daemon runs a cleanup routine every 5 minutes to remove old task records from memory, keeping memory usage efficient.

**Socket defaults:**
- Linux/macOS: `/tmp/parallite.sock`
- Windows: `\\.\pipe\parallite` (currently uses TCP on port 9876)

## 🎯 Usage

### Start the Daemon

```bash
# Use default configuration
./parallite

# Use custom configuration file
./parallite --config /path/to/config.json

# Override socket path via CLI
./parallite --socket /tmp/my-custom.sock

# Override socket path via config (parallite.json)
{
  "socket": "/tmp/my-custom.sock",
  ...
}

# If socket is empty in config, uses OS default:
# - Linux/macOS: /tmp/parallite.sock
# - Windows: TCP on port 9876
./parallite

# Override with CLI flags
./parallite --fixed-workers 4 --timeout-ms 30000

# Run as background service (see INSTALL.md for systemd setup)
nohup ./parallite > /var/log/parallite.log 2>&1 &
```

### CLI Flags

All configuration options can be overridden:

```bash
--config string                Path to config file (default: parallite.json)
--fixed-workers int           Number of persistent workers
--prefix-name string          Worker name prefix
--timeout-ms int              Task timeout in milliseconds
--socket string               IPC socket path
--fail-mode string            Error handling: continue|stop_all
--max-payload-bytes int       Maximum payload size
```

## 🏗️ Architecture

### Worker Management

1. **Persistent Workers**: `fixed_workers` PHP processes start on launch and run continuously
   - Named using `prefix_name` + index (e.g., "work-1", "work-2")
   - Automatically restart if they crash
   - Receive `WORKER_NAME` environment variable

2. **On-Demand Workers**: Created when all persistent workers are busy
   - Execute a single task and exit
   - Named with timestamp suffix (e.g., "work-temp-1234567890")

### Connecting from PHP

Your PHP application should **connect as a client** to the daemon's socket:

```php
// Connect to the daemon socket
$socketPath = '/tmp/parallite.sock'; // Or your custom path
$socket = stream_socket_client(
    'unix://' . $socketPath,
    $errno,
    $errstr,
    30 // timeout
);

if (!$socket) {
    throw new Exception("Failed to connect to daemon: $errstr ($errno)");
}

// Send task (4-byte length + JSON payload)
$payload = json_encode([
    'task_id' => 'task-123',
    'command' => 'php artisan my:command',
    'cwd' => '/path/to/project',
]);

$length = pack('N', strlen($payload)); // Big-endian 32-bit
fwrite($socket, $length . $payload);

// Read response (4-byte length + JSON payload)
$lengthData = fread($socket, 4);
$responseLength = unpack('N', $lengthData)[1];
$response = json_decode(fread($socket, $responseLength), true);

fclose($socket);
```

**Important:** The PHP code should **connect** to the socket, not create a new one!

### IPC Protocol

Communication uses a simple binary protocol:

1. **Request Format**:
   - 4-byte length header (big-endian uint32)
   - JSON payload:
     ```json
     {
       "task_id": "unique-task-id",
       "payload": "serialized PHP closure",
       "context": {}
     }
     ```

2. **Response Format**:
   - 4-byte length header (big-endian uint32)
   - JSON payload:
     ```json
     {
       "task_id": "unique-task-id",
       "ok": true,
       "result": "execution result"
     }
     ```
   - Or on error:
     ```json
     {
       "task_id": "unique-task-id",
       "ok": false,
       "error": "error message"
     }
     ```

### Task Storage

The orchestrator maintains task status and results in memory:

- **Task Status**: Tracks pending, running, and completed tasks
- **Result Cache**: Stores task results for retrieval
- **Automatic Cleanup**: Old task records are removed every 5 minutes
- **Retention**: Results kept for `timeout_ms + 5 minutes`

**Important:** Task results are stored in memory only. Errors are returned directly in the response payload.

### Error Handling

1. **Fail Modes**:
   - `continue`: Log error and continue processing other tasks
   - `stop_all`: Stop all workers and shut down on first error

2. **Timeout Handling**:
   - Tasks exceeding `timeout_ms` are killed
   - Worker process is terminated
   - Persistent workers are automatically restarted

3. **Worker Crashes**:
   - Persistent workers restart automatically after 1 second
   - Tasks assigned to crashed workers fail with error response

## PHP Worker Protocol

The daemon communicates with PHP workers using a `php/parallite-worker.php` script:

- **Input**: 4-byte length prefix (big-endian) + JSON task request
- **Output**: 4-byte length prefix (big-endian) + JSON response
- **Communication**: stdin/stdout pipes
- **Lifecycle**: Workers run in continuous loop for persistent execution

For worker implementation, see the [parallite-php](https://github.com/b7s/parallite-php) package.

## 📋 Requirements

- **Go**: 1.21 or higher

## 🔧 Development

### Cross-Platform Builds

```bash
# Use the build script
./build.sh --cross-compile

# Or manually
GOOS=linux GOARCH=amd64 go build -o parallite-linux main.go
GOOS=darwin GOARCH=amd64 go build -o parallite-macos main.go
GOOS=windows GOARCH=amd64 go build -o parallite.exe main.go
```

## 🧪 Testing

Test the daemon without PHP integration:

```bash
# Quick automated test
chmod +x test.sh
./test.sh
```

Or manually:

```bash
# Terminal 1: Start daemon with test worker
go build -o parallite main.go
USE_TEST_WORKER=1 ./parallite

# Terminal 2: Run test client
cd test && go build -o test-client client.go
./test-client
```

**Expected output:**
```
=== Parallite Test Client ===

Connecting to: /tmp/parallite.sock
✓ Connected successfully

--- Test 1: Simple Task ---
Task ID: task-xxxxx
Status:  true
Result:  {"message":"Task processed successfully",...}

--- Test 2: Task with Context ---
Status:  true
Result:  {"operation":"sum","numbers":[1,2,3,4,5],"result":15}

--- Test 3: Multiple Concurrent Tasks ---
Sending task 1/5... ✓ Success
Sending task 2/5... ✓ Success
...
```

See [TESTING.md](TESTING.md) for comprehensive testing documentation.

## 🐛 Troubleshooting

| Issue | Solution |
|-------|----------|
| **Workers not starting** | Ensure `php/worker.php` exists and PHP is in PATH |
| **Socket connection errors** | Check socket permissions or try custom path with `--socket` |
| **Database locked** | Only one instance can use persistent DB at a time |
| **Tasks timing out** | Increase `timeout_ms` or check PHP worker logs |
| **Payload too large** | Increase `max_payload_bytes` (default: 10MB) |

## 📚 Documentation

- **[QUICKSTART.md](QUICKSTART.md)** - Get started in 5 minutes
- **[INSTALL.md](INSTALL.md)** - Detailed installation and deployment
- **[TESTING.md](TESTING.md)** - Testing without PHP integration
- **[PROJECT_SUMMARY.md](PROJECT_SUMMARY.md)** - Technical architecture
- **[CHANGELOG.md](CHANGELOG.md)** - Version history

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 📄 License

MIT License - see LICENSE file for details

## 🔗 Links

- **GitHub**: [https://github.com/b7s/parallite](https://github.com/b7s/parallite)
- **Issues**: [https://github.com/b7s/parallite/issues](https://github.com/b7s/parallite/issues)
