// Package transport provides various transport mechanisms for MCP communication.
package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// StdioTransport implements the Transport interface using standard input and output.
// This is the recommended transport mechanism in the MCP specification for command-line
// tools and applications where the client and server are typically connected via pipes.
type StdioTransport struct {
	// reader reads from stdin
	reader *bufio.Reader
	// writer writes to stdout
	writer *bufio.Writer
	// receiveHandler is called when a message is received
	receiveHandler ReceiveHandler
	// errorHandler is called when an error occurs
	errorHandler ErrorHandler
	// mutex protects access to the handlers
	mutex sync.RWMutex
	// done is closed when the transport is stopped
	done chan struct{}
}

// NewStdioTransport creates a new transport that uses standard input and output.
// This is the recommended transport in the MCP specification.
func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		reader: bufio.NewReader(os.Stdin),
		writer: bufio.NewWriter(os.Stdout),
		done:   make(chan struct{}),
	}
}

// Initialize prepares the transport for use.
// For StdioTransport, this is a no-op as the stdin/stdout are already available.
func (t *StdioTransport) Initialize(ctx context.Context) error {
	return nil
}

// Start begins reading messages from stdin and processing them.
// This method blocks until the context is canceled or an error occurs.
func (t *StdioTransport) Start(ctx context.Context) error {
	// Create a scanner for reading lines from stdin
	scanner := bufio.NewScanner(t.reader)

	// Create a channel for the scanner to signal it's done
	scannerDone := make(chan struct{})

	// Set up a goroutine to read from stdin
	go func() {
		defer close(scannerDone)

		// Read lines until EOF or error
		for scanner.Scan() {
			// Get the line (should be a complete JSON message)
			line := scanner.Bytes()

			// Copy the line to avoid it being overwritten by the next Scan
			data := make([]byte, len(line))
			copy(data, line)

			// Process the message in a goroutine to avoid blocking
			go t.processMessage(data)
		}

		// Check for scanner errors
		if err := scanner.Err(); err != nil && err != io.EOF {
			t.handleError(fmt.Errorf("error reading from stdin: %w", err))
		}
	}()

	// Wait for either the context to be canceled or the scanner to finish
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-scannerDone:
		return nil
	case <-t.done:
		return nil
	}
}

// Stop halts the transport and cleans up resources.
func (t *StdioTransport) Stop(ctx context.Context) error {
	close(t.done)
	if err := t.writer.Flush(); err != nil {
		return fmt.Errorf("error flushing writer: %w", err)
	}
	return nil
}

// Send transmits a message over the transport.
// For StdioTransport, this writes a line to stdout.
func (t *StdioTransport) Send(data []byte) error {
	// Acquire a lock to prevent concurrent writes
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	// Write the message followed by a newline
	if _, err := t.writer.Write(data); err != nil {
		return fmt.Errorf("error writing to stdout: %w", err)
	}

	// Write a newline to terminate the message
	if err := t.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("error writing newline to stdout: %w", err)
	}

	// Flush the buffer to ensure the message is sent
	if err := t.writer.Flush(); err != nil {
		return fmt.Errorf("error flushing stdout: %w", err)
	}

	return nil
}

// SetReceiveHandler sets the handler for received messages.
func (t *StdioTransport) SetReceiveHandler(handler ReceiveHandler) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.receiveHandler = handler
}

// SetErrorHandler sets the handler for transport errors.
func (t *StdioTransport) SetErrorHandler(handler ErrorHandler) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.errorHandler = handler
}

// processMessage handles a received message by validating it and passing it to the receive handler.
func (t *StdioTransport) processMessage(data []byte) {
	// Acquire a read lock to access the handler
	t.mutex.RLock()
	handler := t.receiveHandler
	t.mutex.RUnlock()

	// Check if we have a handler
	if handler == nil {
		t.handleError(fmt.Errorf("received message but no handler is set"))
		return
	}

	// Validate that the message is valid JSON
	var jsonObj interface{}
	if err := json.Unmarshal(data, &jsonObj); err != nil {
		t.handleError(fmt.Errorf("received invalid JSON: %w", err))
		return
	}

	// Pass the message to the handler
	handler(data)
}

// handleError processes an error by passing it to the error handler if one is set.
func (t *StdioTransport) handleError(err error) {
	// Acquire a read lock to access the handler
	t.mutex.RLock()
	handler := t.errorHandler
	t.mutex.RUnlock()

	// If we have an error handler, use it
	if handler != nil {
		handler(err)
	}
}

// GenerateID generates a unique ID for requests.
// For StdioTransport, this is a simple incrementing counter.
func (t *StdioTransport) GenerateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// SendRequest sends a request and returns the response
func (t *StdioTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	// This is a placeholder implementation
	return nil, fmt.Errorf("SendRequest not implemented in StdioTransport")
}

// SendNotification sends a notification (one-way message)
func (t *StdioTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	// This is a placeholder implementation
	return fmt.Errorf("SendNotification not implemented in StdioTransport")
}

// RegisterRequestHandler registers a handler for incoming requests
func (t *StdioTransport) RegisterRequestHandler(method string, handler RequestHandler) {
	// This is a placeholder implementation
}

// RegisterNotificationHandler registers a handler for incoming notifications
func (t *StdioTransport) RegisterNotificationHandler(method string, handler NotificationHandler) {
	// This is a placeholder implementation
}

// RegisterProgressHandler registers a handler for progress events
func (t *StdioTransport) RegisterProgressHandler(id interface{}, handler ProgressHandler) {
	// This is a placeholder implementation
}

// UnregisterProgressHandler removes a progress handler
func (t *StdioTransport) UnregisterProgressHandler(id interface{}) {
	// This is a placeholder implementation
}
