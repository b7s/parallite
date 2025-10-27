# Testing Parallite Without PHP

Você pode testar o daemon Parallite sem precisar do PHP usando as ferramentas de teste incluídas.

## Opção 1: Test Client em Go (Recomendado)

### 1. Compile o test client

```bash
make test-client
```

Ou manualmente:

```bash
cd test
go build -o test-client client.go
```

### 2. Inicie o daemon

Em um terminal:

```bash
# Com test worker (recomendado para testes)
USE_TEST_WORKER=1 ./parallite

# Ou com worker normal (espera closures serializadas)
./parallite
```

### 3. Execute o test client

Em outro terminal:

```bash
./test/test-client
```

**Saída esperada:**

```
=== Parallite Test Client ===

Connecting to: /tmp/parallite.sock
✓ Connected successfully

--- Test 1: Simple Task ---
Task ID: task-1729026123456789
Status:  false
Error:   Failed to read worker response: EOF

--- Test 2: Task with Context ---
Task ID: task-1729026123456790
Status:  false
Error:   Failed to read worker response: EOF

--- Test 3: Multiple Concurrent Tasks ---
Sending task 1/5... ✗ Failed: Failed to read worker response: EOF
...

✓ All tests completed!
```

**Nota:** Os erros "EOF" são esperados porque o worker PHP não está respondendo. Isso é normal para testar apenas a comunicação IPC.

## Opção 2: Test Worker em PHP

Se você tiver PHP instalado, pode usar o worker de teste simplificado.

### Usando a variável de ambiente (Recomendado)

```bash
# Força o uso do test_worker.php
USE_TEST_WORKER=1 ./parallite
```

### Ou renomeando o worker

```bash
mv php/worker.php php/worker.php.bak
mv php/test_worker.php php/worker.php
```

**Nota:** O daemon detecta automaticamente `test_worker.php` se `worker.php` não existir.

### 2. Inicie o daemon

```bash
./parallite
```

Você verá no log:

```
Using test worker: php/test_worker.php
```

### 3. Execute o test client

```bash
./test/test-client
```

**Saída esperada (com sucesso):**

```
=== Parallite Test Client ===

Connecting to: /tmp/parallite.sock
✓ Connected successfully

--- Test 1: Simple Task ---
Task ID: task-1729026123456789
Status:  true
Result:  {"message":"Task processed successfully","data":"Hello from test client!","timestamp":"2025-10-15 20:15:23"}

--- Test 2: Task with Context ---
Task ID: task-1729026123456790
Status:  true
Result:  {"operation":"sum","numbers":[1,2,3,4,5],"result":15}

--- Test 3: Multiple Concurrent Tasks ---
Sending task 1/5... ✓ Success
Sending task 2/5... ✓ Success
Sending task 3/5... ✓ Success
Sending task 4/5... ✓ Success
Sending task 5/5... ✓ Success

✓ All tests completed!
```

## Opção 3: Teste Manual com netcat

Você também pode testar manualmente usando `netcat`:

```bash
# Inicie o daemon
./parallite

# Em outro terminal, conecte ao socket
nc -U /tmp/parallite.sock

# Envie uma mensagem (você precisará calcular o tamanho manualmente)
# Isso é mais complexo e não recomendado
```

## Verificando o Funcionamento

### Logs do Daemon

O daemon mostra logs úteis:

```
2025/10/15 20:15:20 Starting Parallite orchestrator with config: ...
2025/10/15 20:15:20 Started worker: work-1 (persistent=true, PID=12345)
2025/10/15 20:15:20 Listening on: /tmp/parallite.sock
```

### Verificar Workers Rodando

```bash
ps aux | grep "php.*worker"
```

### Verificar Database

Se `db_persistent=true`:

```bash
sqlite3 parallite.sqlite "SELECT * FROM tasks ORDER BY created_at DESC LIMIT 10;"
```

## Troubleshooting

### "Failed to connect"

- Certifique-se que o daemon está rodando: `ps aux | grep parallite`
- Verifique se o socket existe: `ls -l /tmp/parallite.sock`

### "Failed to read worker response: EOF"

- Normal se não houver worker PHP respondendo
- Instale PHP ou use o test_worker.php

### "Worker not starting"

- Verifique se PHP está instalado: `php --version`
- Verifique se o arquivo existe: `ls -l php/test_worker.php`

## Próximos Passos

Depois de testar com sucesso:

1. Restaure o worker original: `mv php/worker.php.bak php/worker.php`
2. Integre com seu pacote PHP
3. Implemente closures serializadas reais

## Estrutura de Teste

```
parallite-go-daemon/
├── parallite              # Daemon principal
├── test/
│   ├── client.go         # Código do test client
│   └── test-client       # Binário compilado
└── php/
    ├── worker.php        # Worker real (com closures)
    └── test_worker.php   # Worker de teste (simplificado)
```
