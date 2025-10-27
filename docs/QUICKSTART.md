# Quick Start Guide

## Installation

1. **Install dependencies:**

   ```bash
   go mod download
   ```

2. **Build the binary:**

   ```bash
   go build -o bin/parallite .
   ```

   Or use the Makefile:

   ```bash
   make build
   ```

## Basic Usage

1. **Start the daemon with default settings:**

   ```bash
   ./bin/parallite
   ```

2. **Start with custom configuration:**

   ```bash
   ./bin/parallite --fixed-workers 4 --timeout-ms 30000 --worker-script /path/to/parallite-worker.php
   ```

3. **What happens internally:**

   - The orchestrator boots an event loop that drives IPC, worker IO, and scheduled maintenance.
   - A managed blocking pool runs filesystem checks and process waits without stalling the loop.
   - Persistent workers are registered immediately; temporary workers spin up when demand exceeds capacity.


## Testing the Daemon

You can test the daemon using a simple PHP client. Create `test_client.php`:

```php
<?php

require 'vendor/autoload.php';

use MessagePack\MessagePack;

// Connect to the socket
$socket = socket_create(AF_UNIX, SOCK_STREAM, 0);
socket_connect($socket, '/tmp/parallite.sock');

// Create a test task
$task = [
    'type' => 'submit',
    'task_id' => uniqid('task_'),
    'payload' => \Opis\Closure\serialize(function() {
        return 'Hello from Parallite!';
    }),
    'context' => [],
];

// Encode task with MessagePack
$taskPacked = MessagePack::pack($task);
$length = pack('N', strlen($taskPacked));

// Send task
socket_write($socket, $length . $taskPacked);

// Read response length
$lengthData = socket_read($socket, 4);
$responseLength = unpack('N', $lengthData)[1];

// Read response
$response = socket_read($socket, $responseLength);
$result = MessagePack::unpack($response);

print_r($result);

socket_close($socket);
```

Run the test:

```bash
php test_client.php
```

## Configuration Examples

### High-Performance Setup

```json
{
  "fixed_workers": 8,
  "prefix_name": "worker",
  "timeout_ms": 30000,
  "fail_mode": "continue",
  "max_payload_bytes": 10485760,
  "worker_script": "/var/www/app/vendor/parallite/parallite-php/src/Support/parallite-worker.php"
}
```

### Development Setup

```json
{
  "fixed_workers": 1,
  "prefix_name": "dev",
  "timeout_ms": 120000,
  "fail_mode": "stop_all",
  "max_payload_bytes": 10485760,
  "debug_logs": true,
  "worker_script": ""
}
```

### Production Setup

```json
{
  "fixed_workers": 16,
  "prefix_name": "prod",
  "timeout_ms": 60000,
  "fail_mode": "continue",
  "max_payload_bytes": 10485760,
  "worker_script": "/opt/parallite/parallite-worker.php"
}
```

## Monitoring

### Check Running Workers
```bash
ps aux | grep "php.*worker.php"
```

### Monitor Logs
The daemon logs to stderr. Redirect to a file:

```bash
./bin/parallite 2>&1 | tee parallite.log
```

## Stopping the Daemon

Send SIGTERM or SIGINT:
```bash
kill -TERM $(pgrep parallite)
```

Or use Ctrl+C if running in foreground.

## Troubleshooting

### "Address already in use"
Another instance is running. Find and stop it:
```bash
pgrep parallite
kill $(pgrep parallite)
```

### "Permission denied" on socket
Check socket permissions:
```bash
ls -l /tmp/parallite.sock
```

### Workers not responding
Check PHP error logs and ensure `php/worker.php` is executable:
```bash
chmod +x php/worker.php
```
