package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Version is set during build time
var Version = "v1.1.0"

// Config holds the application configuration
type Config struct {
	FixedWorkers    int    `json:"fixed_workers"`
	PrefixName      string `json:"prefix_name"`
	TimeoutMs       int    `json:"timeout_ms"`
	Socket          string `json:"socket"`
	FailMode        string `json:"fail_mode"`
	MaxPayloadBytes int    `json:"max_payload_bytes"`
	EnableBenchmark bool   `json:"enable_benchmark"`
}

// TaskRequest represents an incoming task
type TaskRequest struct {
	Type            string          `json:"type,omitempty"` // "submit" or "await"
	TaskID          string          `json:"task_id"`
	Command         string          `json:"command,omitempty"`
	Cwd             string          `json:"cwd,omitempty"`
	Env             json.RawMessage `json:"env,omitempty"`
	Payload         json.RawMessage `json:"payload"`
	Context         json.RawMessage `json:"context,omitempty"`
	EnableBenchmark *bool           `json:"enable_benchmark,omitempty"`
}

// TaskResponse represents a task result
type TaskResponse struct {
	TaskID    string          `json:"task_id"`
	Ok        bool            `json:"ok"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Benchmark json.RawMessage `json:"benchmark,omitempty"`
}

// Worker represents a PHP worker process
type Worker struct {
	Name       string
	Cmd        *exec.Cmd
	Stdin      io.WriteCloser
	Stdout     io.ReadCloser
	Persistent bool
	Busy       bool
	mu         sync.Mutex
}

// IsAlive checks if the worker process is still running
func (w *Worker) IsAlive() bool {
	if w.Cmd == nil || w.Cmd.Process == nil {
		return false
	}

	// Check if process has exited
	if w.Cmd.ProcessState != nil && w.Cmd.ProcessState.Exited() {
		return false
	}

	// Try to signal the process (signal 0 doesn't actually send a signal)
	err := w.Cmd.Process.Signal(syscall.Signal(0))
	return err == nil
}

// TaskStatus represents the status of a task in memory
type TaskStatus struct {
	TaskID      string
	Status      string // "pending", "running", "completed", "failed"
	SubmittedAt time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Result      *TaskResponse
}

// Orchestrator manages the entire system
type Orchestrator struct {
	config        Config
	taskStatus    map[string]*TaskStatus   // In-memory task tracking
	resultCache   map[string]*TaskResponse // Cache completed results for await
	workers       []*Worker
	listener      net.Listener
	workerPool    chan *Worker
	taskQueue     chan *TaskRequest
	responseChans map[string]chan *TaskResponse
	mu            sync.RWMutex
	wg            sync.WaitGroup
	shutdown      chan struct{}
	stopAll       bool
}

func main() {
	// CLI flags
	version := flag.Bool("version", false, "Show version and exit")
	configFile := flag.String("config", "parallite.json", "Path to configuration file")
	fixedWorkers := flag.Int("fixed-workers", 0, "Number of persistent workers (0 = use config)")
	prefixName := flag.String("prefix-name", "", "Worker name prefix (empty = use config)")
	timeoutMs := flag.Int("timeout-ms", 0, "Task timeout in milliseconds (0 = use config)")
	socket := flag.String("socket", "", "IPC socket path (empty = use config)")
	failMode := flag.String("fail-mode", "", "Fail mode: stop_all or continue (empty = use config)")
	maxPayloadBytes := flag.Int("max-payload-bytes", 0, "Max payload size in bytes (0 = use config)")
	flag.Parse()

	// Show version and exit
	if *version {
		fmt.Printf("Parallite %s\n", Version)
		os.Exit(0)
	}

	// Load configuration
	config, err := loadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Apply CLI overrides
	if *fixedWorkers > 0 {
		config.FixedWorkers = *fixedWorkers
	}
	if *prefixName != "" {
		config.PrefixName = *prefixName
	}
	if *timeoutMs > 0 {
		config.TimeoutMs = *timeoutMs
	}
	if *socket != "" {
		config.Socket = *socket
	}
	if *failMode != "" {
		config.FailMode = *failMode
	}
	if *maxPayloadBytes > 0 {
		config.MaxPayloadBytes = *maxPayloadBytes
	}

	// Set default socket based on OS
	if config.Socket == "" {
		config.Socket = getDefaultSocket()
	}

	log.Printf("Starting Parallite orchestrator with config: %+v", config)

	// Initialize orchestrator
	orch, err := NewOrchestrator(config)
	if err != nil {
		log.Fatalf("Failed to initialize orchestrator: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal")
		orch.Shutdown()
	}()

	// Start orchestrator
	if err := orch.Start(); err != nil {
		log.Fatalf("Orchestrator failed: %v", err)
	}
}

func loadConfig(path string) (Config, error) {
	config := Config{
		FixedWorkers:    1,
		PrefixName:      "work",
		TimeoutMs:       60000,
		FailMode:        "continue",
		MaxPayloadBytes: 10 * 1024 * 1024,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Config file not found, using defaults")
			return config, nil
		}
		return config, err
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}

	return config, nil
}

func getDefaultSocket() string {
	if runtime.GOOS == "windows" {
		return `\\.\pipe\parallite`
	}
	return "/tmp/parallite.sock"
}

// NewOrchestrator creates a new orchestrator instance
func NewOrchestrator(config Config) (*Orchestrator, error) {
	orch := &Orchestrator{
		config:        config,
		taskStatus:    make(map[string]*TaskStatus),
		resultCache:   make(map[string]*TaskResponse),
		workerPool:    make(chan *Worker, config.FixedWorkers),
		taskQueue:     make(chan *TaskRequest, 100),
		responseChans: make(map[string]chan *TaskResponse),
		shutdown:      make(chan struct{}),
	}

	return orch, nil
}

// Start begins the orchestrator
func (o *Orchestrator) Start() error {
	// Start persistent workers
	workersStarted := 0
	for i := 1; i <= o.config.FixedWorkers; i++ {
		workerName := fmt.Sprintf("%s-%d", o.config.PrefixName, i)
		worker, err := o.startWorker(workerName, true)
		if err != nil {
			log.Printf("Warning: Could not start persistent worker %s: %v", workerName, err)
			continue
		}
		o.workers = append(o.workers, worker)
		o.workerPool <- worker
		workersStarted++
	}

	if workersStarted == 0 && o.config.FixedWorkers > 0 {
		log.Printf("Warning: No persistent workers started. Workers will be created on-demand when tasks arrive.")
	} else if workersStarted < o.config.FixedWorkers {
		log.Printf("Warning: Only %d of %d persistent workers started successfully.", workersStarted, o.config.FixedWorkers)
	}

	// Start IPC listener
	if err := o.startListener(); err != nil {
		return err
	}

	// Start cleanup routine
	o.wg.Add(1)
	go o.cleanupRoutine()

	// Start worker monitor routine
	o.wg.Add(1)
	go o.monitorWorkers()

	// Start task processor
	o.wg.Add(1)
	go o.processTaskQueue()

	// Start heartbeat to show daemon is alive
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-o.shutdown:
				return
			case <-ticker.C:
				log.Printf("Daemon is alive and waiting for connections on %s", o.config.Socket)
			}
		}
	}()

	// Wait for shutdown
	<-o.shutdown
	return nil
}

func (o *Orchestrator) startWorker(name string, persistent bool) (*Worker, error) {
	workerFileName := "parallite-worker.php"

	// Use test_worker.php ONLY if USE_TEST_WORKER env var is set
	if os.Getenv("USE_TEST_WORKER") == "1" {
		workerFileName = "test_worker.php"
	}

	// Search for worker file in multiple locations (priority order)
	searchPaths := []string{
		filepath.Join("src", "Support", workerFileName), // ./src/Support/parallite-worker.php (recommended)
		workerFileName,                       // ./parallite-worker.php (root)
		filepath.Join("php", workerFileName), // ./php/parallite-worker.php
		filepath.Join("..", "..", "src", "Support", workerFileName), // ../../src/Support/parallite-worker.php (from vendor/bin)
		filepath.Join("..", "..", workerFileName),                   // ../../parallite-worker.php (from vendor/bin)
		filepath.Join("..", "..", "php", workerFileName),            // ../../php/parallite-worker.php (from vendor/bin)
	}

	var phpWorkerPath string
	var found bool

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			phpWorkerPath = path
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("worker file not found. Searched in: %v", searchPaths)
	}

	if os.Getenv("USE_TEST_WORKER") == "1" {
		log.Printf("Using test worker: %s", phpWorkerPath)
	}

	// Get absolute path for better logging
	absPath, _ := filepath.Abs(phpWorkerPath)
	log.Printf("Starting worker %s with PHP script: %s", name, absPath)

	cmd := exec.Command("php", phpWorkerPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("WORKER_NAME=%s", name))

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	// Capture stderr for debugging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start worker %s: %v", name, err)
		return nil, err
	}

	log.Printf("Worker %s started successfully (PID: %d)", name, cmd.Process.Pid)

	// Give PHP a moment to initialize and check if it's still alive
	time.Sleep(101 * time.Millisecond)

	// Check if process died immediately
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		exitCode := cmd.ProcessState.ExitCode()
		log.Printf("WARNING: Worker %s died immediately after start (exit code: %d)", name, exitCode)
		return nil, fmt.Errorf("worker died immediately with exit code %d", exitCode)
	}

	log.Printf("Worker %s is alive and running", name)

	// Log stderr in background
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				log.Printf("[Worker %s STDERR] %s", name, string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	worker := &Worker{
		Name:       name,
		Cmd:        cmd,
		Stdin:      stdin,
		Stdout:     stdout,
		Persistent: persistent,
		Busy:       false,
	}

	// Monitor worker if persistent
	if persistent {
		go o.monitorWorker(worker)
	}

	log.Printf("Started worker: %s (persistent=%v, PID=%d)", name, persistent, cmd.Process.Pid)
	return worker, nil
}

func (o *Orchestrator) monitorWorker(worker *Worker) {
	err := worker.Cmd.Wait()

	if o.stopAll {
		return
	}

	// Log detailed exit information
	exitCode := -1
	if worker.Cmd.ProcessState != nil {
		exitCode = worker.Cmd.ProcessState.ExitCode()
	}

	if err != nil {
		log.Printf("Worker %s exited with error (exit code: %d): %v", worker.Name, exitCode, err)
	} else {
		log.Printf("Worker %s exited normally (exit code: %d)", worker.Name, exitCode)
	}

	// Restart persistent worker
	if worker.Persistent {
		time.Sleep(1 * time.Second)
		newWorker, err := o.startWorker(worker.Name, true)
		if err != nil {
			log.Printf("Failed to restart worker %s: %v", worker.Name, err)
			return
		}

		// Replace in workers list
		o.mu.Lock()
		for i, w := range o.workers {
			if w.Name == worker.Name {
				o.workers[i] = newWorker
				break
			}
		}
		o.mu.Unlock()

		// Return to pool if not busy
		if !worker.Busy {
			o.workerPool <- newWorker
		}
	}
}

func (o *Orchestrator) startListener() error {
	// Remove existing socket file on Unix
	if runtime.GOOS != "windows" {
		os.Remove(o.config.Socket)
	}

	var err error
	if runtime.GOOS == "windows" {
		o.listener, err = net.Listen("tcp", "127.0.0.1:9876")
	} else {
		o.listener, err = net.Listen("unix", o.config.Socket)
	}

	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	log.Printf("Listening on: %s", o.config.Socket)

	o.wg.Add(1)
	go o.acceptConnections()
	return nil
}

func (o *Orchestrator) acceptConnections() {
	defer o.wg.Done()
	for {
		conn, err := o.listener.Accept()
		if err != nil {
			select {
			case <-o.shutdown:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		go o.handleConnection(conn)
	}
}

func (o *Orchestrator) handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("New connection from: %s", conn.RemoteAddr())

	// Set read timeout to avoid hanging forever
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	for {
		// Read length header (4 bytes, big-endian)
		log.Printf("Waiting for data from connection...")
		var length uint32
		if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
			if err != io.EOF {
				log.Printf("Failed to read length: %v", err)
			} else {
				log.Printf("Connection closed by client (EOF)")
			}
			return
		}
		log.Printf("Received length header: %d bytes", length)

		// Validate payload size
		if length > uint32(o.config.MaxPayloadBytes) {
			log.Printf("Payload too large: %d bytes (max: %d)", length, o.config.MaxPayloadBytes)
			return
		}

		// Read payload
		log.Printf("Reading payload of %d bytes...", length)
		payload := make([]byte, length)
		if _, err := io.ReadFull(conn, payload); err != nil {
			log.Printf("Failed to read payload: %v", err)
			return
		}
		log.Printf("Payload read successfully")

		// Parse request
		var req TaskRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			log.Printf("Failed to parse request: %v", err)
			continue
		}

		benchmarkStatus := "default"
		if req.EnableBenchmark != nil {
			benchmarkStatus = fmt.Sprintf("%v", *req.EnableBenchmark)
		}
		log.Printf("Received %s for task %s (payload: %d bytes, enable_benchmark: %s)", req.Type, req.TaskID, length, benchmarkStatus)

		// Handle different message types
		var resp *TaskResponse

		if req.Type == "await" {
			// Client is waiting for an existing task result
			log.Printf("Client awaiting result for task %s", req.TaskID)

			o.mu.RLock()
			cachedResult, inCache := o.resultCache[req.TaskID]
			respChan, inProgress := o.responseChans[req.TaskID]
			o.mu.RUnlock()

			if inCache {
				// Result already completed and cached
				log.Printf("Task %s found in cache (ok=%v)", req.TaskID, cachedResult.Ok)
				resp = cachedResult
			} else if inProgress {
				// Task still in progress, wait for it
				log.Printf("Task %s still in progress, waiting...", req.TaskID)
				resp = <-respChan
				log.Printf("Task %s completed (ok=%v)", req.TaskID, resp.Ok)
			} else {
				// Task doesn't exist
				log.Printf("Task %s not found", req.TaskID)
				resp = &TaskResponse{
					TaskID: req.TaskID,
					Ok:     false,
					Error:  "Task not found",
				}
			}
		} else {
			// Default to "submit" - create new task
			if req.Type == "" {
				req.Type = "submit"
			}

			// Check if task is already being processed
			o.mu.Lock()
			if _, exists := o.responseChans[req.TaskID]; exists {
				o.mu.Unlock()
				log.Printf("Task %s is already being processed, ignoring duplicate submit", req.TaskID)
				continue
			}

			// Create response channel
			respChan := make(chan *TaskResponse, 1)
			o.responseChans[req.TaskID] = respChan
			o.mu.Unlock()

			// Queue task
			o.taskQueue <- &req

			// Wait for response
			resp = <-respChan
			log.Printf("Task %s completed (ok=%v)", req.TaskID, resp.Ok)
		}

		// Cache the result for future await requests
		o.mu.Lock()
		o.resultCache[req.TaskID] = resp
		o.mu.Unlock()

		// Send response
		respData, _ := json.Marshal(resp)
		respLength := uint32(len(respData))

		log.Printf("Sending response for task %s (%d bytes)", req.TaskID, respLength)

		if err := binary.Write(conn, binary.BigEndian, respLength); err != nil {
			// Check if it's a broken pipe (client disconnected)
			if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset") {
				log.Printf("Client disconnected before receiving response for task %s", req.TaskID)
			} else {
				log.Printf("Failed to write response length: %v", err)
			}
			return
		}

		if _, err := conn.Write(respData); err != nil {
			if strings.Contains(err.Error(), "broken pipe") || strings.Contains(err.Error(), "connection reset") {
				log.Printf("Client disconnected while sending response for task %s", req.TaskID)
			} else {
				log.Printf("Failed to write response: %v", err)
			}
			return
		}

		log.Printf("Response sent successfully for task %s", req.TaskID)
	}
}

func (o *Orchestrator) processTaskQueue() {
	defer o.wg.Done()
	for {
		select {
		case <-o.shutdown:
			return
		case task := <-o.taskQueue:
			go o.executeTask(task)
		}
	}
}

func (o *Orchestrator) executeTask(task *TaskRequest) {
	// Record task in memory
	now := time.Now()
	o.mu.Lock()
	o.taskStatus[task.TaskID] = &TaskStatus{
		TaskID:      task.TaskID,
		Status:      "pending",
		SubmittedAt: now,
	}
	o.mu.Unlock()

	// Get worker from pool or create on-demand
	var worker *Worker
	select {
	case worker = <-o.workerPool:
		log.Printf("Task %s assigned to persistent worker %s", task.TaskID, worker.Name)
		worker.mu.Lock()
		worker.Busy = true
		worker.mu.Unlock()
	default:
		// Create on-demand worker
		log.Printf("Task %s: creating on-demand worker", task.TaskID)
		workerName := fmt.Sprintf("%s-temp-%d", o.config.PrefixName, time.Now().UnixNano())
		var err error
		worker, err = o.startWorker(workerName, false)
		if err != nil {
			log.Printf("Task %s: failed to create on-demand worker: %v", task.TaskID, err)
			o.sendResponse(task.TaskID, &TaskResponse{
				TaskID: task.TaskID,
				Ok:     false,
				Error:  fmt.Sprintf("Failed to start worker: %v", err),
			})
			return
		}
		worker.Busy = true
	}

	// Update task status to running
	startTime := time.Now()
	o.mu.Lock()
	if status, exists := o.taskStatus[task.TaskID]; exists {
		status.Status = "running"
		status.StartedAt = &startTime
	}
	o.mu.Unlock()

	// Execute task with timeout
	result := make(chan *TaskResponse, 1)
	go func() {
		resp := o.runTask(worker, task)
		result <- resp
	}()

	timeout := time.Duration(o.config.TimeoutMs) * time.Millisecond
	var response *TaskResponse

	select {
	case response = <-result:
		// Task completed
	case <-time.After(timeout):
		// Timeout - kill worker
		log.Printf("Task %s timed out, killing worker %s", task.TaskID, worker.Name)
		worker.Cmd.Process.Kill()
		response = &TaskResponse{
			TaskID: task.TaskID,
			Ok:     false,
			Error:  "Task execution timed out",
		}
	}

	// Update task status in memory
	completedTime := time.Now()
	o.mu.Lock()
	if status, exists := o.taskStatus[task.TaskID]; exists {
		status.CompletedAt = &completedTime
		status.Result = response
		if response.Ok {
			status.Status = "completed"
		} else {
			status.Status = "failed"
		}
	}
	o.mu.Unlock()

	// Send response
	o.sendResponse(task.TaskID, response)

	// Return worker to pool or cleanup
	worker.mu.Lock()
	worker.Busy = false
	worker.mu.Unlock()

	if worker.Persistent {
		o.workerPool <- worker
	} else {
		// Cleanup temporary worker
		log.Printf("Cleaning up temporary worker %s", worker.Name)

		// Close stdin to signal worker to exit
		if worker.Stdin != nil {
			worker.Stdin.Close()
		}

		// Give worker 2 seconds to exit gracefully
		done := make(chan error, 1)
		go func() {
			done <- worker.Cmd.Wait()
		}()

		select {
		case <-done:
			log.Printf("Temporary worker %s exited gracefully", worker.Name)
		case <-time.After(2 * time.Second):
			// Force kill if not exited
			log.Printf("Force killing temporary worker %s", worker.Name)
			if worker.Cmd.Process != nil {
				worker.Cmd.Process.Kill()
				worker.Cmd.Wait() // Reap zombie
			}
		}
	}

	// Handle fail mode
	if !response.Ok && o.config.FailMode == "stop_all" {
		log.Printf("Task failed and fail_mode=stop_all, shutting down")
		o.stopAll = true
		o.Shutdown()
	}
}

func (o *Orchestrator) runTask(worker *Worker, task *TaskRequest) *TaskResponse {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in task %s: %v", task.TaskID, r)
		}
	}()

	// Check if worker process is still alive
	if !worker.IsAlive() {
		log.Printf("Worker %s is not alive, cannot execute task %s", worker.Name, task.TaskID)
		return &TaskResponse{
			TaskID: task.TaskID,
			Ok:     false,
			Error:  fmt.Sprintf("Worker %s is not alive", worker.Name),
		}
	}

	// Send task to worker
	taskData, err := json.Marshal(task)
	if err != nil {
		return &TaskResponse{
			TaskID: task.TaskID,
			Ok:     false,
			Error:  fmt.Sprintf("Failed to marshal task: %v", err),
		}
	}

	// Log benchmark setting being sent to worker
	benchmarkStatus := "default"
	if task.EnableBenchmark != nil {
		benchmarkStatus = fmt.Sprintf("%v", *task.EnableBenchmark)
	}
	log.Printf("Sending task %s to worker with enable_benchmark: %s", task.TaskID, benchmarkStatus)

	// Prepend 4-byte length header (big-endian)
	length := uint32(len(taskData))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)
	taskDataWithLength := append(lengthBytes, taskData...)

	// Protect against closed pipe
	worker.mu.Lock()
	if worker.Stdin == nil {
		worker.mu.Unlock()
		log.Printf("Worker %s stdin is nil for task %s", worker.Name, task.TaskID)
		return &TaskResponse{
			TaskID: task.TaskID,
			Ok:     false,
			Error:  "Worker stdin is closed",
		}
	}

	n, writeErr := worker.Stdin.Write(taskDataWithLength)
	worker.mu.Unlock()

	if writeErr != nil {
		log.Printf("Failed to write to worker %s stdin: %v (wrote %d bytes)", worker.Name, writeErr, n)
		return &TaskResponse{
			TaskID: task.TaskID,
			Ok:     false,
			Error:  fmt.Sprintf("Failed to send task to worker: %v", writeErr),
		}
	}

	log.Printf("Sent task %s to worker %s (%d bytes)", task.TaskID, worker.Name, n)

	// Read 4-byte length header from worker response
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(worker.Stdout, lengthBuf); err != nil {
		log.Printf("Failed to read response length from worker %s: %v", worker.Name, err)
		return &TaskResponse{
			TaskID: task.TaskID,
			Ok:     false,
			Error:  fmt.Sprintf("Failed to read worker response length: %v", err),
		}
	}

	responseLength := binary.BigEndian.Uint32(lengthBuf)
	log.Printf("Worker %s response length: %d bytes", worker.Name, responseLength)

	// Read response payload
	responseBuf := make([]byte, responseLength)
	if _, err := io.ReadFull(worker.Stdout, responseBuf); err != nil {
		log.Printf("Failed to read response payload from worker %s: %v", worker.Name, err)
		return &TaskResponse{
			TaskID: task.TaskID,
			Ok:     false,
			Error:  fmt.Sprintf("Failed to read worker response: %v", err),
		}
	}

	var response TaskResponse
	if err := json.Unmarshal(responseBuf, &response); err != nil {
		log.Printf("Failed to parse worker response: %v. Raw: %s", err, string(responseBuf))
		return &TaskResponse{
			TaskID: task.TaskID,
			Ok:     false,
			Error:  fmt.Sprintf("Failed to parse worker response: %v", err),
		}
	}

	return &response
}

func (o *Orchestrator) sendResponse(taskID string, response *TaskResponse) {
	o.mu.RLock()
	respChan, exists := o.responseChans[taskID]
	o.mu.RUnlock()

	if exists {
		respChan <- response
	}
}

// monitorWorkers periodically checks worker health and cleans up dead processes
func (o *Orchestrator) monitorWorkers() {
	defer o.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-o.shutdown:
			return
		case <-ticker.C:
			o.mu.Lock()
			for i, worker := range o.workers {
				if worker == nil {
					continue
				}

				// Check if worker process is still alive
				if !worker.IsAlive() {
					if worker.Persistent {
						log.Printf("Persistent worker %s died unexpectedly, restarting...", worker.Name)

						// Remove dead worker from list
						o.workers[i] = nil

						// Try to restart
						newWorker, err := o.startWorker(worker.Name, true)
						if err != nil {
							log.Printf("Failed to restart worker %s: %v", worker.Name, err)
						} else {
							o.workers[i] = newWorker
							o.workerPool <- newWorker
						}
					} else {
						// Temporary worker - should be dead, just ensure cleanup
						if worker.Cmd != nil && worker.Cmd.Process != nil {
							worker.Cmd.Process.Kill()
							worker.Cmd.Wait() // Reap zombie
						}
						log.Printf("Cleaned up dead temporary worker %s", worker.Name)
					}
				} else {
					// Worker is alive - check if it's a temporary worker that should be dead
					worker.mu.Lock()
					isIdle := !worker.Busy
					isPersistent := worker.Persistent
					worker.mu.Unlock()

					if !isPersistent && isIdle {
						// Temporary worker that finished its task but is still running
						log.Printf("Killing idle temporary worker %s", worker.Name)
						if worker.Stdin != nil {
							worker.Stdin.Close()
						}
						if worker.Cmd != nil && worker.Cmd.Process != nil {
							worker.Cmd.Process.Kill()
							worker.Cmd.Wait() // Reap zombie
						}
					}
				}
			}
			o.mu.Unlock()
		}
	}
}

// cleanupRoutine periodically cleans up old task records from memory
func (o *Orchestrator) cleanupRoutine() {
	defer o.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-o.shutdown:
			return
		case <-ticker.C:
			// Keep task results for timeout duration + 5 minutes buffer
			// This ensures results are available even if client is slightly delayed
			retentionMs := o.config.TimeoutMs + (5 * 60 * 1000) // timeout + 5 minutes
			cutoff := time.Now().Add(-time.Duration(retentionMs) * time.Millisecond)

			o.mu.Lock()
			cleaned := 0
			for taskID, status := range o.taskStatus {
				// Remove completed/failed tasks older than retention period
				if status.CompletedAt != nil && status.CompletedAt.Before(cutoff) {
					delete(o.taskStatus, taskID)
					delete(o.resultCache, taskID)
					delete(o.responseChans, taskID)
					cleaned++
				}
			}
			o.mu.Unlock()

			if cleaned > 0 {
				log.Printf("Cleaned up %d old task records from memory", cleaned)
			}
		}
	}
}

// Shutdown gracefully stops the orchestrator
func (o *Orchestrator) Shutdown() {
	log.Println("Shutting down orchestrator...")

	// Signal shutdown
	close(o.shutdown)

	// Close listener
	if o.listener != nil {
		o.listener.Close()
	}

	// Stop all workers
	for _, worker := range o.workers {
		if worker.Cmd != nil && worker.Cmd.Process != nil {
			worker.Stdin.Close()
			worker.Cmd.Process.Signal(syscall.SIGTERM)

			// Wait briefly for graceful shutdown
			done := make(chan struct{})
			go func() {
				worker.Cmd.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				worker.Cmd.Process.Kill()
			}
		}
	}

	// Wait for goroutines to finish
	o.wg.Wait()

	log.Println("Shutdown complete")
}
