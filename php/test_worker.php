<?php

declare(strict_types=1);

/**
 * Simple Test Worker for Parallite
 * 
 * This is a simplified worker for testing without serialized closures.
 * It processes simple JSON payloads and returns results.
 */

$workerName = getenv('WORKER_NAME') ?: 'test-worker';
error_log("Test Worker {$workerName} started (PID: " . getmypid() . ")");

// Main loop
while (true) {
    $line = fgets(STDIN);
    
    if ($line === false) {
        error_log("Worker {$workerName}: stdin closed, exiting");
        break;
    }
    
    $line = trim($line);
    if (empty($line)) {
        continue;
    }
    
    try {
        // Parse task request
        $request = json_decode($line, true, 512, JSON_THROW_ON_ERROR);
        
        if (!isset($request['task_id']) || !isset($request['payload'])) {
            throw new Exception('Invalid task request');
        }
        
        $taskId = $request['task_id'];
        
        // Payload comes as JSON string, decode it
        $payload = is_string($request['payload']) 
            ? json_decode($request['payload'], true) 
            : $request['payload'];
            
        // Context comes as JSON string if present, decode it
        $context = null;
        if (isset($request['context'])) {
            $context = is_string($request['context']) 
                ? json_decode($request['context'], true) 
                : $request['context'];
        }
        
        error_log("Worker {$workerName}: processing task {$taskId}");
        
        // Process based on payload type
        $result = processTask($payload, $context);
        
        // Send success response
        $response = [
            'task_id' => $taskId,
            'ok' => true,
            'result' => json_encode($result),
        ];
        
        error_log("Worker {$workerName}: task {$taskId} completed");
        
    } catch (Throwable $e) {
        // Send error response
        $response = [
            'task_id' => $taskId ?? 'unknown',
            'ok' => false,
            'error' => $e->getMessage(),
        ];
        
        error_log("Worker {$workerName}: task failed - {$e->getMessage()}");
    }
    
    // Write response
    fwrite(STDOUT, json_encode($response, JSON_THROW_ON_ERROR) . "\n");
    fflush(STDOUT);
}

error_log("Worker {$workerName} exiting");

function processTask(array $payload, ?array $context): array
{
    $type = $payload['type'] ?? 'unknown';
    
    switch ($type) {
        case 'simple':
            return [
                'message' => 'Task processed successfully',
                'data' => $payload['data'] ?? null,
                'timestamp' => date('Y-m-d H:i:s'),
            ];
            
        case 'computation':
            if ($context && isset($context['numbers'])) {
                $sum = array_sum($context['numbers']);
                return [
                    'operation' => $context['operation'] ?? 'sum',
                    'numbers' => $context['numbers'],
                    'result' => $sum,
                ];
            }
            return ['result' => 'No computation data'];
            
        case 'batch':
            // Simulate some work
            usleep(100000); // 100ms
            return [
                'batch_index' => $payload['index'] ?? 0,
                'processed' => true,
                'data' => $payload['data'] ?? null,
            ];
            
        default:
            return [
                'message' => 'Unknown task type',
                'type' => $type,
            ];
    }
}
