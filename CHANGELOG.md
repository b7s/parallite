# Changelog

All notable changes to the Parallite Go Daemon will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-10-15

### Added
- Initial release of Parallite Go Daemon
- Cross-platform support (Windows, Linux, macOS)
- Persistent worker pool management
- On-demand worker spawning for overflow tasks
- IPC protocol using Unix domain sockets and TCP
- SQLite task registry with configurable persistence
- Automatic task cleanup based on retention policy
- Per-task timeout enforcement
- Automatic worker restart on crash
- Two fail modes: continue or stop_all
- JSON configuration file with CLI overrides
- Comprehensive documentation and examples
- PHP worker script for closure execution
- Test client for validation
- systemd service file example
- Build script with cross-compilation support
- Makefile for common tasks

### Features
- **Worker Management**
  - Configurable number of persistent workers
  - Custom worker name prefixes
  - Environment variable passing (WORKER_NAME)
  - Automatic crash recovery
  - On-demand worker creation

- **IPC Protocol**
  - Binary length-prefixed JSON messages
  - Request/response correlation via task_id
  - Support for serialized PHP closures
  - Optional context passing
  - Payload size validation (10MB max)

- **Database**
  - Pure Go SQLite implementation (no CGo)
  - Persistent or in-memory modes
  - Task status tracking (pending, running, done)
  - Automatic cleanup routine
  - Indexed for performance
  - Errors NOT stored (returned in response only)

- **Error Handling**
  - Panic recovery with logging
  - Worker crash detection
  - Timeout enforcement with process termination
  - Configurable fail modes
  - Detailed error messages

- **Configuration**
  - JSON configuration file
  - CLI flag overrides for all options
  - OS-specific socket detection
  - Sensible defaults

### Technical Details
- Go 1.21+ required
- Pure Go dependencies only (no CGo)
- Concurrent goroutine-based architecture
- Channel-based worker pool
- Mutex-protected shared state
- Graceful shutdown handling

### Documentation
- README.md: Comprehensive documentation
- QUICKSTART.md: Quick start guide
- PROJECT_SUMMARY.md: Technical overview
- Examples: Test client, configurations, systemd service

[1.0.0]: https://github.com/b7s/parallite/releases/tag/v1.0.0
