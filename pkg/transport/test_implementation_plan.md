# Transport Package Test Implementation Plan

## Phase 1: BaseTransport Tests (transport_test.go additions)

### 1. TestSetLogger
```go
func TestSetLogger(t *testing.T) {
    bt := NewBaseTransport()

    // Create custom logger
    var buf bytes.Buffer
    customLogger := log.New(&buf, "TEST: ", log.LstdFlags)

    // Set logger
    bt.SetLogger(customLogger)

    // Use Logf to verify logger is used
    bt.Logf("test message %d", 123)

    // Check buffer contains the message
    assert.Contains(t, buf.String(), "TEST:")
    assert.Contains(t, buf.String(), "test message 123")
}
```

### 2. TestHandleRequestPanic
```go
func TestHandleRequestPanic(t *testing.T) {
    bt := NewBaseTransport()

    // Register handler that panics
    bt.RegisterRequestHandler("panic", func(ctx context.Context, params interface{}) (interface{}, error) {
        panic("test panic")
    })

    req := &protocol.Request{
        ID:     "test-id",
        Method: "panic",
        Params: json.RawMessage(`{}`),
    }

    // Should recover from panic and return error response
    resp, err := bt.HandleRequest(context.Background(), req)
    require.NoError(t, err)
    require.NotNil(t, resp)
    require.NotNil(t, resp.Error)
    assert.Equal(t, protocol.InternalError, resp.Error.Code)
    assert.Contains(t, resp.Error.Message, "Internal server error")
}
```

### 3. TestHandleNotificationPanic
```go
func TestHandleNotificationPanic(t *testing.T) {
    bt := NewBaseTransport()

    // Register handler that panics
    bt.RegisterNotificationHandler("panic", func(ctx context.Context, params interface{}) error {
        panic("notification panic")
    })

    notif := &protocol.Notification{
        Method: "panic",
        Params: json.RawMessage(`{}`),
    }

    // Should recover from panic and return error
    err := bt.HandleNotification(context.Background(), notif)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "internal error processing notification")
}
```

### 4. TestSafeGoPanic
```go
func TestSafeGoPanic(t *testing.T) {
    var logBuf bytes.Buffer
    logger := func(format string, args ...interface{}) {
        fmt.Fprintf(&logBuf, format, args...)
    }

    done := make(chan bool)

    SafeGo(logger, "test-goroutine", func() {
        defer close(done)
        panic("test panic in goroutine")
    })

    <-done

    assert.Contains(t, logBuf.String(), "Panic in goroutine test-goroutine")
    assert.Contains(t, logBuf.String(), "test panic in goroutine")
}
```

### 5. TestConcurrentHandlers
```go
func TestConcurrentHandlers(t *testing.T) {
    bt := NewBaseTransport()

    // Test concurrent registration and handling
    var wg sync.WaitGroup

    // Register handlers concurrently
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            method := fmt.Sprintf("method-%d", idx)
            bt.RegisterRequestHandler(method, func(ctx context.Context, params interface{}) (interface{}, error) {
                return fmt.Sprintf("response-%d", idx), nil
            })
        }(i)
    }

    wg.Wait()

    // Call handlers concurrently
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            req := &protocol.Request{
                ID:     fmt.Sprintf("id-%d", idx),
                Method: fmt.Sprintf("method-%d", idx),
                Params: json.RawMessage(`{}`),
            }
            resp, err := bt.HandleRequest(context.Background(), req)
            assert.NoError(t, err)
            assert.NotNil(t, resp)
        }(i)
    }

    wg.Wait()
}
```

## Phase 2: StdioTransport Tests (stdio_test.go additions)

### 1. TestNewStdioTransportWithStdInOut
```go
func TestNewStdioTransportWithStdInOut(t *testing.T) {
    // Save original stdin/stdout
    oldStdin := os.Stdin
    oldStdout := os.Stdout
    defer func() {
        os.Stdin = oldStdin
        os.Stdout = oldStdout
    }()

    // Create pipes for testing
    inR, inW, _ := os.Pipe()
    outR, outW, _ := os.Pipe()

    os.Stdin = inR
    os.Stdout = outW

    tr := NewStdioTransportWithStdInOut()

    assert.NotNil(t, tr)
    assert.Equal(t, os.Stdin, tr.reader)
    assert.Equal(t, os.Stdout, tr.writer)

    inW.Close()
    outR.Close()
}
```

### 2. TestStdioTransportInitialize
```go
func TestStdioTransportInitialize(t *testing.T) {
    tr := NewStdioTransport(strings.NewReader(""), &bytes.Buffer{})

    err := tr.Initialize(context.Background())
    assert.NoError(t, err)
}
```

### 3. TestStdioTransportGenerateID
```go
func TestStdioTransportGenerateID(t *testing.T) {
    tr := NewStdioTransport(strings.NewReader(""), &bytes.Buffer{})

    id1 := tr.GenerateID()
    id2 := tr.GenerateID()

    assert.NotEmpty(t, id1)
    assert.NotEmpty(t, id2)
    assert.NotEqual(t, id1, id2)
}
```

### 4. TestStdioTransportSendNotification
```go
func TestStdioTransportSendNotification(t *testing.T) {
    outBuf := &bytes.Buffer{}
    tr := NewStdioTransport(strings.NewReader(""), outBuf)

    params := map[string]string{"key": "value"}
    err := tr.SendNotification(context.Background(), "test.notification", params)

    assert.NoError(t, err)

    // Check output
    output := outBuf.String()
    assert.Contains(t, output, `"method":"test.notification"`)
    assert.Contains(t, output, `"params":{"key":"value"}`)
    assert.NotContains(t, output, `"id"`) // Notifications don't have IDs
}
```

### 5. TestStdioTransportProgressHandlers
```go
func TestStdioTransportProgressHandlers(t *testing.T) {
    tr := NewStdioTransport(strings.NewReader(""), &bytes.Buffer{})

    handlerCalled := false
    handler := func(params interface{}) error {
        handlerCalled = true
        return nil
    }

    // Test registration
    tr.RegisterProgressHandler("test-id", handler)

    // Test unregistration
    tr.UnregisterProgressHandler("test-id")

    // Currently these are no-op implementations
    // When implemented, add assertions for handler invocation
}
```

### 6. TestStdioTransportConcurrentOperations
```go
func TestStdioTransportConcurrentOperations(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    inR, inW := io.Pipe()
    outR, outW := io.Pipe()

    tr := NewStdioTransport(inR, outW)

    // Start transport
    go tr.Start(ctx)

    var wg sync.WaitGroup

    // Send multiple messages concurrently
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()
            data := []byte(fmt.Sprintf("message %d", idx))
            err := tr.Send(data)
            assert.NoError(t, err)
        }(i)
    }

    // Read messages
    go func() {
        scanner := bufio.NewScanner(outR)
        count := 0
        for scanner.Scan() && count < 5 {
            t.Logf("Received: %s", scanner.Text())
            count++
        }
    }()

    wg.Wait()
    time.Sleep(100 * time.Millisecond)

    inW.Close()
    outR.Close()
}
```

### 7. TestStdioTransportLargeMessages
```go
func TestStdioTransportLargeMessages(t *testing.T) {
    outBuf := &bytes.Buffer{}
    tr := NewStdioTransport(strings.NewReader(""), outBuf)

    // Create large message (1MB)
    largeData := make([]byte, 1024*1024)
    for i := range largeData {
        largeData[i] = byte('A' + (i % 26))
    }

    err := tr.Send(largeData)
    assert.NoError(t, err)

    // Verify output
    output := outBuf.Bytes()
    assert.Equal(t, len(largeData)+1, len(output)) // +1 for newline
    assert.Equal(t, largeData, output[:len(largeData)])
    assert.Equal(t, byte('\n'), output[len(output)-1])
}
```

## Phase 3: HTTPTransport Tests (http_test.go - new file)

### Test Helper Functions
```go
// Mock HTTP server with SSE support
func newMockHTTPServer(t *testing.T) *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/events" {
            // SSE endpoint
            w.Header().Set("Content-Type", "text/event-stream")
            w.Header().Set("Cache-Control", "no-cache")
            w.Header().Set("Connection", "keep-alive")

            flusher, ok := w.(http.Flusher)
            if !ok {
                http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
                return
            }

            // Send test event
            fmt.Fprintf(w, "data: {\"test\": \"event\"}\n\n")
            flusher.Flush()

            // Keep connection open
            <-r.Context().Done()
        } else {
            // Regular endpoint
            var req protocol.Request
            json.NewDecoder(r.Body).Decode(&req)

            resp := protocol.Response{
                ID:     req.ID,
                Result: json.RawMessage(`{"status": "ok"}`),
            }

            json.NewEncoder(w).Encode(resp)
        }
    }))
}
```

### Core HTTPTransport Tests
```go
func TestNewHTTPTransport(t *testing.T) {
    server := newMockHTTPServer(t)
    defer server.Close()

    tr := NewHTTPTransport(server.URL, WithRequestTimeout(5*time.Second))

    assert.NotNil(t, tr)
    assert.Equal(t, server.URL, tr.serverURL)
    assert.Equal(t, 5*time.Second, tr.options.RequestTimeout)
}

func TestHTTPTransportInitialize(t *testing.T) {
    server := newMockHTTPServer(t)
    defer server.Close()

    tr := NewHTTPTransport(server.URL)

    err := tr.Initialize(context.Background())
    assert.NoError(t, err)
    assert.NotNil(t, tr.eventSource)
}

func TestHTTPTransportSendRequest(t *testing.T) {
    server := newMockHTTPServer(t)
    defer server.Close()

    tr := NewHTTPTransport(server.URL)
    tr.Initialize(context.Background())

    // Start event processing in background
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    go tr.Start(ctx)
    time.Sleep(100 * time.Millisecond) // Let it connect

    // Send request
    result, err := tr.SendRequest(context.Background(), "test.method", map[string]string{"param": "value"})

    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

## Phase 4: StreamableHTTPTransport Tests (streamable_http_test.go - new file)

### Mock Streamable Server
```go
func newMockStreamableServer(t *testing.T) *httptest.Server {
    sessionID := "test-session-123"

    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Handle different content types
        accept := r.Header.Get("Accept")

        if strings.Contains(accept, "text/event-stream") {
            // SSE response
            w.Header().Set("Content-Type", "text/event-stream")
            w.Header().Set("MCP-Session-ID", sessionID)

            flusher, ok := w.(http.Flusher)
            if !ok {
                return
            }

            // Send connection event
            fmt.Fprintf(w, "event: connected\n")
            fmt.Fprintf(w, "data: {\"connectionId\": \"conn-123\"}\n\n")
            flusher.Flush()

            // Keep alive
            <-r.Context().Done()
        } else {
            // JSON response
            w.Header().Set("Content-Type", "application/json")
            w.Header().Set("MCP-Session-ID", sessionID)

            var req protocol.Request
            json.NewDecoder(r.Body).Decode(&req)

            if req.Method == "initialize" {
                resp := protocol.Response{
                    ID:     req.ID,
                    Result: json.RawMessage(`{"capabilities": {}}`),
                }
                json.NewEncoder(w).Encode(resp)
            } else {
                resp := protocol.Response{
                    ID:     req.ID,
                    Result: json.RawMessage(`{"status": "ok"}`),
                }
                json.NewEncoder(w).Encode(resp)
            }
        }
    }))
}
```

### Core StreamableHTTPTransport Tests
```go
func TestNewStreamableHTTPTransport(t *testing.T) {
    tr := NewStreamableHTTPTransport("http://localhost:8080")

    assert.NotNil(t, tr)
    assert.Equal(t, "http://localhost:8080", tr.endpoint)
    assert.Equal(t, "streamable-http", tr.requestIDPrefix)
}

func TestStreamableHTTPTransportSessionManagement(t *testing.T) {
    server := newMockStreamableServer(t)
    defer server.Close()

    tr := NewStreamableHTTPTransport(server.URL)

    // Initialize should set session ID
    err := tr.Initialize(context.Background())
    assert.NoError(t, err)

    sessionID := tr.GetSessionID()
    assert.NotEmpty(t, sessionID)
    assert.Equal(t, "test-session-123", sessionID)
}
```

## Test Execution Strategy

1. **Incremental Implementation**
   - Start with Phase 1 (BaseTransport)
   - Run coverage after each phase
   - Adjust subsequent phases based on results

2. **Continuous Integration**
   - Add tests to CI pipeline
   - Set coverage threshold at 80%
   - Block PRs that reduce coverage

3. **Performance Benchmarks**
   - Add benchmarks for critical paths
   - Monitor for performance regressions

4. **Mock Server Improvements**
   - Create reusable mock server package
   - Support various error scenarios
   - Enable deterministic testing
