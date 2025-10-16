package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"runtime"
	"time"
)

type TestTaskRequest struct {
	TaskID          string          `json:"task_id"`
	Payload         json.RawMessage `json:"payload"`
	Context         json.RawMessage `json:"context,omitempty"`
	EnableBenchmark *bool           `json:"enable_benchmark,omitempty"`
}

type TestTaskResponse struct {
	TaskID    string          `json:"task_id"`
	Ok        bool            `json:"ok"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Benchmark json.RawMessage `json:"benchmark,omitempty"`
}

func main() {
	fmt.Println("=== Parallite Test Client ===\n")

	// Determine socket path based on OS
	socketPath := "/tmp/parallite.sock"
	if runtime.GOOS == "windows" {
		socketPath = "127.0.0.1:9876"
	}

	fmt.Printf("Connecting to: %s\n", socketPath)

	// Connect to daemon
	var conn net.Conn
	var err error

	if runtime.GOOS == "windows" {
		conn, err = net.Dial("tcp", socketPath)
	} else {
		conn, err = net.Dial("unix", socketPath)
	}

	if err != nil {
		log.Fatalf("Failed to connect: %v\n\nMake sure the daemon is running:\n  ./parallite\n", err)
	}
	defer conn.Close()

	fmt.Println("✓ Connected successfully\n")

	// Test 1: Simple task (no benchmark flag - uses config default)
	fmt.Println("--- Test 1: Simple Task (no benchmark flag) ---")
	testSimpleTask(conn)
	time.Sleep(500 * time.Millisecond)

	// Test 2: Task with benchmark enabled
	fmt.Println("\n--- Test 2: Task with Benchmark ENABLED ---")
	testTaskWithBenchmark(conn, true)
	time.Sleep(500 * time.Millisecond)

	// Test 3: Task with benchmark explicitly disabled
	fmt.Println("\n--- Test 3: Task with Benchmark DISABLED ---")
	testTaskWithBenchmark(conn, false)
	time.Sleep(500 * time.Millisecond)

	// Test 3: Multiple concurrent tasks
	fmt.Println("\n--- Test 3: Multiple Concurrent Tasks ---")
	testMultipleTasks(conn)

	fmt.Println("\n✓ All tests completed!")
}

func testSimpleTask(conn net.Conn) {
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

	// Create a simple payload (simulating serialized PHP closure)
	payload := map[string]interface{}{
		"type": "simple",
		"data": "Hello from test client!",
	}
	payloadJSON, _ := json.Marshal(payload)

	request := TestTaskRequest{
		TaskID:  taskID,
		Payload: payloadJSON,
	}

	response := sendTask(conn, request)
	printResponse(response)
}

func testTaskWithBenchmark(conn net.Conn, enableBenchmark bool) {
	taskID := fmt.Sprintf("task-%d", time.Now().UnixNano())

	payload := map[string]interface{}{
		"type": "benchmark_test",
		"data": fmt.Sprintf("Testing with benchmark=%v", enableBenchmark),
	}
	payloadJSON, _ := json.Marshal(payload)

	request := TestTaskRequest{
		TaskID:          taskID,
		Payload:         payloadJSON,
		EnableBenchmark: &enableBenchmark,
	}

	response := sendTask(conn, request)
	printResponse(response)
}

func testMultipleTasks(conn net.Conn) {
	for i := 1; i <= 5; i++ {
		taskID := fmt.Sprintf("task-batch-%d-%d", time.Now().UnixNano(), i)

		payload := map[string]interface{}{
			"type":  "batch",
			"index": i,
			"data":  fmt.Sprintf("Batch task #%d", i),
		}
		payloadJSON, _ := json.Marshal(payload)

		request := TestTaskRequest{
			TaskID:  taskID,
			Payload: payloadJSON,
		}

		fmt.Printf("Sending task %d/%d... ", i, 5)
		response := sendTask(conn, request)

		if response.Ok {
			fmt.Println("✓ Success")
		} else {
			fmt.Printf("✗ Failed: %s\n", response.Error)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func sendTask(conn net.Conn, request TestTaskRequest) TestTaskResponse {
	// Encode request
	requestJSON, err := json.Marshal(request)
	if err != nil {
		log.Fatalf("Failed to marshal request: %v", err)
	}

	// Send length header (4 bytes, big-endian)
	length := uint32(len(requestJSON))
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		log.Fatalf("Failed to write length: %v", err)
	}

	// Send payload
	if _, err := conn.Write(requestJSON); err != nil {
		log.Fatalf("Failed to write payload: %v", err)
	}

	// Read response length
	var responseLength uint32
	if err := binary.Read(conn, binary.BigEndian, &responseLength); err != nil {
		log.Fatalf("Failed to read response length: %v", err)
	}

	// Read response payload
	responseData := make([]byte, responseLength)
	n := 0
	for n < int(responseLength) {
		read, err := conn.Read(responseData[n:])
		if err != nil {
			log.Fatalf("Failed to read response: %v", err)
		}
		n += read
	}

	// Parse response
	var response TestTaskResponse
	if err := json.Unmarshal(responseData, &response); err != nil {
		log.Fatalf("Failed to unmarshal response: %v", err)
	}

	return response
}

func printResponse(response TestTaskResponse) {
	fmt.Printf("Task ID: %s\n", response.TaskID)
	fmt.Printf("Status:  %v\n", response.Ok)

	if response.Ok {
		fmt.Printf("Result:  %s\n", string(response.Result))
		if len(response.Benchmark) > 0 {
			fmt.Printf("Benchmark: %s\n", string(response.Benchmark))
		}
	} else {
		fmt.Printf("Error:   %s\n", response.Error)
	}
}
