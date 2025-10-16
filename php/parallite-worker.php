<?php

declare(strict_types=1);

/**
 * Parallite PHP Worker
 * 
 * This worker receives serialized PHP closures from the Go orchestrator,
 * executes them, and returns the results.
 */

$workerName = getenv('WORKER_NAME') ?: 'unknown';
error_log("Worker {$workerName} started (PID: " . getmypid() . ")");

// Set error handler to catch all errors
set_error_handler(function ($severity, $message, $file, $line) {
    throw new ErrorException($message, 0, $severity, $file, $line);
});

// Main loop - read from stdin, execute, write to stdout
while (true) {
    // Read 4-byte length prefix (endian int-32)
    $lengthData = fread(STDIN, 4);

    if ($lengthData === false || strlen($lengthData) !== 4) {
        error_log("Worker {$workerName}: failed to read length, exiting");
        break;
    }

    $length = unpack('N', $lengthData)[1];

    // Read payload
    $payload = '';
    $remaining = $length;

    while ($remaining > 0) {
        $chunk = fread(STDIN, $remaining);

        if ($chunk === false) {
            error_log("Worker {$workerName}: failed to read payload, exiting");
            break 2;
        }

        $payload .= $chunk;
        $remaining -= strlen($chunk);
    }

    try {
        // Parse task request
        $request = json_decode($payload, true, 512, JSON_THROW_ON_ERROR);

        if (empty($request['task_id'])) {
            throw new Exception('Invalid task request: missing task_id');
        }

        $taskId = $request['task_id'];

        error_log("Worker {$workerName}: executing task {$taskId}");

        // Check if it's a closure (has 'payload') or a shell command (has 'command')
        if (isset($request['payload'])) {
            // Execute closure
            $payload = $request['payload'];
            $context = $request['context'] ?? null;
            $result = executeTask($payload, $context);
        } elseif (isset($request['command'])) {
            // Execute shell command
            $command = $request['command'];
            $cwd = $request['cwd'] ?? getcwd();
            $env = $request['env'] ?? null;

            error_log("Worker {$workerName}: executing command: {$command}");

            $result = executeCommand($command, $cwd, $env);
        } else {
            throw new Exception('Invalid task request: missing payload or command');
        }

        // Send success response
        $response = [
            'task_id' => $taskId,
            'ok' => true,
            'result' => $result,
        ];

        error_log("Worker {$workerName}: task {$taskId} completed successfully");

    } catch (Throwable $e) {
        // Send error response
        $response = [
            'task_id' => $taskId ?? 'unknown',
            'ok' => false,
            'error' => sprintf(
                '%s: %s in %s:%d',
                get_class($e),
                $e->getMessage(),
                $e->getFile(),
                $e->getLine()
            ),
        ];

        error_log("Worker {$workerName}: task failed - {$response['error']}");
    }

    // Write response to stdout with 4-byte length prefix
    $responseJson = json_encode($response, JSON_THROW_ON_ERROR);
    $responseLength = pack('N', strlen($responseJson));

    fwrite(STDOUT, $responseLength . $responseJson);
    fflush(STDOUT);
}

error_log("Worker {$workerName} exiting");

/**
 * Execute a shell command
 *
 * @param string $command The command to execute
 * @param string $cwd Working directory
 * @param array|null $env Environment variables
 * @return array Command output and exit code
 */
function executeCommand(string $command, string $cwd, ?array $env = null): array
{
    $descriptors = [
        0 => ['pipe', 'r'],  // stdin
        1 => ['pipe', 'w'],  // stdout
        2 => ['pipe', 'w'],  // stderr
    ];

    $process = proc_open($command, $descriptors, $pipes, $cwd, $env);

    if (!is_resource($process)) {
        throw new Exception('Failed to start process');
    }

    // Close stdin
    fclose($pipes[0]);

    // Read stdout and stderr
    $stdout = stream_get_contents($pipes[1]);
    $stderr = stream_get_contents($pipes[2]);

    fclose($pipes[1]);
    fclose($pipes[2]);

    // Get exit code
    $exitCode = proc_close($process);

    return [
        'stdout' => $stdout,
        'stderr' => $stderr,
        'exit_code' => $exitCode,
    ];
}

/**
 * Execute a serialized PHP closure
 *
 * @param mixed $payload The serialized closure or callable
 * @param mixed $context Optional context data
 * @return mixed The result of the closure execution
 */
function executeTask($payload, $context = null)
{
    // If payload is a string, try to unserialize it
    if (is_string($payload)) {
        $closure = unserialize($payload);
    } else {
        $closure = $payload;
    }

    if (!is_callable($closure)) {
        throw new Exception('Payload is not callable');
    }

    // Execute the closure with context if provided
    if ($context !== null) {
        return $closure($context);
    }

    return $closure();
}
