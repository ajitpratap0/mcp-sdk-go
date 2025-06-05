package transport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// MockHTTPServer provides a configurable HTTP server for testing
type MockHTTPServer struct {
	server        *httptest.Server
	mu            sync.Mutex
	requests      [][]byte
	responses     map[string]*MockResponse
	sseMessages   []string
	sseDelay      time.Duration
	responseDelay time.Duration
	errorOnNth    int
	requestCount  int
}

// MockResponse represents a configurable HTTP response
type MockResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       interface{}
	IsSSE      bool
}

// NewMockHTTPServer creates a new mock HTTP server
func NewMockHTTPServer() *MockHTTPServer {
	m := &MockHTTPServer{
		responses: make(map[string]*MockResponse),
		requests:  make([][]byte, 0),
		sseDelay:  10 * time.Millisecond,
	}

	m.server = httptest.NewServer(http.HandlerFunc(m.handler))
	return m
}

// handler processes HTTP requests
func (m *MockHTTPServer) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	m.requestCount++
	currentCount := m.requestCount
	m.mu.Unlock()

	// Simulate errors
	if m.errorOnNth > 0 && currentCount == m.errorOnNth {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Capture request body
	body, _ := io.ReadAll(r.Body)
	m.mu.Lock()
	m.requests = append(m.requests, body)
	m.mu.Unlock()

	// Apply response delay
	if m.responseDelay > 0 {
		time.Sleep(m.responseDelay)
	}

	// Handle SSE requests
	if r.Header.Get("Accept") == "text/event-stream" {
		m.handleSSE(w, r)
		return
	}

	// Get configured response
	key := fmt.Sprintf("%s:%s", r.Method, r.URL.Path)
	m.mu.Lock()
	resp, ok := m.responses[key]
	m.mu.Unlock()

	if !ok {
		// Default response
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Apply headers
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}

	// Write status code
	if resp.StatusCode > 0 {
		w.WriteHeader(resp.StatusCode)
	}

	// Write body
	if resp.Body != nil {
		if err := json.NewEncoder(w).Encode(resp.Body); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

// handleSSE handles Server-Sent Events
func (m *MockHTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send SSE messages
	m.mu.Lock()
	messages := make([]string, len(m.sseMessages))
	copy(messages, m.sseMessages)
	delay := m.sseDelay
	m.mu.Unlock()

	for _, msg := range messages {
		fmt.Fprintf(w, "data: %s\n\n", msg)
		flusher.Flush()
		time.Sleep(delay)
	}

	// Keep connection open until client closes
	<-r.Context().Done()
}

// SetResponse configures a response for a specific method and path
func (m *MockHTTPServer) SetResponse(method, path string, response *MockResponse) {
	key := fmt.Sprintf("%s:%s", method, path)
	m.mu.Lock()
	m.responses[key] = response
	m.mu.Unlock()
}

// AddSSEMessage adds a message to be sent via SSE
func (m *MockHTTPServer) AddSSEMessage(msg string) {
	m.mu.Lock()
	m.sseMessages = append(m.sseMessages, msg)
	m.mu.Unlock()
}

// GetRequests returns all captured requests
func (m *MockHTTPServer) GetRequests() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	reqs := make([][]byte, len(m.requests))
	copy(reqs, m.requests)
	return reqs
}

// URL returns the server URL
func (m *MockHTTPServer) URL() string {
	return m.server.URL
}

// Close shuts down the server
func (m *MockHTTPServer) Close() {
	m.server.Close()
}

// MockReadWriteCloser implements io.ReadWriteCloser for testing
type MockReadWriteCloser struct {
	*bytes.Buffer
	closed bool
	mu     sync.Mutex
}

// NewMockReadWriteCloser creates a new mock ReadWriteCloser
func NewMockReadWriteCloser() *MockReadWriteCloser {
	return &MockReadWriteCloser{
		Buffer: bytes.NewBuffer(nil),
	}
}

// Close implements io.Closer
func (m *MockReadWriteCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// IsClosed returns whether Close was called
func (m *MockReadWriteCloser) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// MockLogger implements the Logger interface for testing
type MockLogger struct {
	mu       sync.Mutex
	messages []LogMessage
}

// LogMessage represents a captured log message
type LogMessage struct {
	Level   string
	Message string
	Args    []interface{}
}

// NewMockLogger creates a new mock logger
func NewMockLogger() *MockLogger {
	return &MockLogger{
		messages: make([]LogMessage, 0),
	}
}

// Debug logs a debug message
func (l *MockLogger) Debug(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, LogMessage{Level: "DEBUG", Message: msg, Args: args})
}

// Info logs an info message
func (l *MockLogger) Info(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, LogMessage{Level: "INFO", Message: msg, Args: args})
}

// Warn logs a warning message
func (l *MockLogger) Warn(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, LogMessage{Level: "WARN", Message: msg, Args: args})
}

// Error logs an error message
func (l *MockLogger) Error(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.messages = append(l.messages, LogMessage{Level: "ERROR", Message: msg, Args: args})
}

// GetMessages returns all captured log messages
func (l *MockLogger) GetMessages() []LogMessage {
	l.mu.Lock()
	defer l.mu.Unlock()
	msgs := make([]LogMessage, len(l.messages))
	copy(msgs, l.messages)
	return msgs
}

// HasMessage checks if a message with the given level and substring exists
func (l *MockLogger) HasMessage(level, substring string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, msg := range l.messages {
		if msg.Level == level && bytes.Contains([]byte(msg.Message), []byte(substring)) {
			return true
		}
	}
	return false
}

// TestMessage creates a test protocol message
func TestMessage(method string, params interface{}) *protocol.Request {
	paramsJSON, _ := json.Marshal(params)
	return &protocol.Request{
		JSONRPCMessage: protocol.JSONRPCMessage{
			JSONRPC: protocol.JSONRPCVersion,
		},
		ID:     "test-" + method,
		Method: method,
		Params: json.RawMessage(paramsJSON),
	}
}

// TestNotification creates a test protocol notification
func TestNotification(method string, params interface{}) *protocol.Notification {
	paramsJSON, _ := json.Marshal(params)
	return &protocol.Notification{
		JSONRPCMessage: protocol.JSONRPCMessage{
			JSONRPC: protocol.JSONRPCVersion,
		},
		Method: method,
		Params: json.RawMessage(paramsJSON),
	}
}

// TestResponse creates a test protocol response
func TestResponse(id string, result interface{}) *protocol.Response {
	resultJSON, _ := json.Marshal(result)
	return &protocol.Response{
		JSONRPCMessage: protocol.JSONRPCMessage{
			JSONRPC: protocol.JSONRPCVersion,
		},
		ID:     id,
		Result: resultJSON,
	}
}

// TestErrorResponse creates a test error response
func TestErrorResponse(id string, code int, message string) *protocol.Response {
	return &protocol.Response{
		JSONRPCMessage: protocol.JSONRPCMessage{
			JSONRPC: protocol.JSONRPCVersion,
		},
		ID: id,
		Error: &protocol.Error{
			Code:    code,
			Message: message,
		},
	}
}

// SSEScanner helps parse SSE events
type SSEScanner struct {
	scanner *bufio.Scanner
}

// NewSSEScanner creates a new SSE scanner
func NewSSEScanner(r io.Reader) *SSEScanner {
	return &SSEScanner{
		scanner: bufio.NewScanner(r),
	}
}

// Next reads the next SSE event
func (s *SSEScanner) Next() (string, bool) {
	for s.scanner.Scan() {
		line := s.scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			return strings.TrimPrefix(line, "data: "), true
		}
	}
	return "", false
}

// WaitForCondition waits for a condition to be true or times out
func WaitForCondition(t *testing.T, timeout time.Duration, check func() bool, msg string) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("Timeout waiting for condition: %s", msg)
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual(t *testing.T, expected, actual interface{}, msg string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// RunWithTimeout runs a function with a timeout
func RunWithTimeout(t *testing.T, timeout time.Duration, fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()

	select {
	case <-done:
		// Success
	case <-time.After(timeout):
		t.Fatal("Test timed out")
	}
}
