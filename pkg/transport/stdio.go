// Package transport provides various transport mechanisms for MCP communication.
package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
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

    requestHandlers      map[string]RequestHandler

    notificationHandlers map[string]NotificationHandler
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
        requestHandlers:      make(map[string]RequestHandler),
        notificationHandlers: make(map[string]NotificationHandler),
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
			log.Printf("Scanner read line: %s", string(line))

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
    log.Println("Stopping transport...")
    close(t.done)

    if err := t.writer.Flush(); err != nil {
        log.Printf("Error flushing writer during stop: %v", err)
        return fmt.Errorf("error flushing writer: %w", err)
    }

    log.Println("Transport stopped successfully")
    return nil
}

// Send transmits a message over the transport.
// For StdioTransport, this writes a line to stdout.
func (t *StdioTransport) Send(data []byte) error {
    t.mutex.RLock()
    defer t.mutex.RUnlock()

    log.Printf("Writing data to stdout: %s", string(data))
    if _, err := t.writer.Write(data); err != nil {
        log.Printf("Error writing to stdout: %v", err)
        return fmt.Errorf("error writing to stdout: %w", err)
    }

    if err := t.writer.WriteByte('\n'); err != nil {
        log.Printf("Error writing newline to stdout: %v", err)
        return fmt.Errorf("error writing newline to stdout: %w", err)
    }

    if err := t.writer.Flush(); err != nil {
        log.Printf("Error flushing writer: %v", err)
        return fmt.Errorf("error flushing writer: %w", err)
    }

    log.Printf("Data successfully written to stdout")
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

func (t *StdioTransport) HandleRequest(ctx context.Context, req *protocol.Request) (*protocol.Response, error) {
    log.Printf("Handling request: method=%s, ID=%v", req.Method, req.ID)

    t.mutex.RLock()
    handler := t.receiveHandler
    t.mutex.RUnlock()

    if handler == nil {
        return nil, fmt.Errorf("no receive handler set")
    }

    data, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to serialize request: %w", err)
    }

    log.Printf("Passing request to receive handler: %s", string(data))
    handler(data)
    return nil, nil
}

// processMessage handles a received message by validating it and passing it to the receive handler.
func (t *StdioTransport) processMessage(data []byte) {
    log.Printf("Server received message: %s", string(data))

    // Try to unmarshal as a request
    var req protocol.Request
    if err := json.Unmarshal(data, &req); err == nil && req.Method != "" && req.ID != nil {
        t.mutex.RLock()
        handler, ok := t.requestHandlers[req.Method]
        t.mutex.RUnlock()
        if ok && handler != nil {
            result, err := handler(context.Background(), req.Params)
            resp, _ := protocol.NewResponse(req.ID, result)
            if err != nil {
                resp, _ = protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
            }
            respBytes, _ := json.Marshal(resp)
            t.Send(respBytes)
            return
        }
        // Unknown method
        resp, _ := protocol.NewErrorResponse(req.ID, protocol.MethodNotFound, "Method not found", nil)
        respBytes, _ := json.Marshal(resp)
        t.Send(respBytes)
        return
    }

    // Try to unmarshal as a notification
    var notif protocol.Notification
    if err := json.Unmarshal(data, &notif); err == nil && notif.Method != "" {
        t.mutex.RLock()
        handler, ok := t.notificationHandlers[notif.Method]
        t.mutex.RUnlock()
        if ok && handler != nil {
            handler(context.Background(), notif.Params)
        }
        return
    }

    log.Printf("Ignoring message: not a request or notification: %s", string(data))
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
    log.Printf("Preparing to send request: method=%s, params=%v", method, params)

    // Serialize params to json.RawMessage
    paramsJSON, err := json.Marshal(params)
    if err != nil {
        log.Printf("Failed to serialize params: %v", err)
        return nil, fmt.Errorf("failed to serialize params: %w", err)
    }

    // Create the request
    req := protocol.Request{
        ID:     t.GenerateID(),
        Method: method,
        Params: json.RawMessage(paramsJSON),
    }

    // Serialize the request to JSON
    reqBytes, err := json.Marshal(req)
    if err != nil {
        log.Printf("Failed to serialize request: %v", err)
        return nil, fmt.Errorf("failed to serialize request: %w", err)
    }

    // Write the request to stdout
    log.Printf("Sending request: %s", string(reqBytes))
    if err := t.Send(reqBytes); err != nil {
        log.Printf("Failed to send request: %v", err)
        return nil, fmt.Errorf("failed to send request: %w", err)
    }

    // Read the response from stdin
    log.Printf("Waiting for response...")
    respBytes, err := t.reader.ReadBytes('\n')
    if err != nil {
        log.Printf("Error reading response: %v", err)
        return nil, fmt.Errorf("failed to read response: %w", err)
    }
    log.Printf("Received raw response: %s", string(respBytes))

    // Deserialize the response
    var resp protocol.Response
    if err := json.Unmarshal(respBytes, &resp); err != nil {
        log.Printf("Failed to deserialize response: %v", err)
        return nil, fmt.Errorf("failed to deserialize response: %w", err)
    }

    if resp.Error != nil {
        log.Printf("Server returned an error: %s", resp.Error.Message)
        return nil, fmt.Errorf("server error: %s", resp.Error.Message)
    }

    log.Printf("Request successful: result=%v", resp.Result)
    return resp.Result, nil
}

// SendNotification sends a notification (one-way message)
func (t *StdioTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
    log.Printf("Preparing to send notification: method=%s, params=%v", method, params)

    // Serialize params to json.RawMessage
    paramsJSON, err := json.Marshal(params)
    if err != nil {
        log.Printf("Failed to serialize params: %v", err)
        return fmt.Errorf("failed to serialize params: %w", err)
    }

    // Create the notification
    notif := protocol.Notification{
        Method: method,
        Params: json.RawMessage(paramsJSON),
    }

    // Serialize the notification to JSON
    notifBytes, err := json.Marshal(notif)
    if err != nil {
        log.Printf("Failed to serialize notification: %v", err)
        return fmt.Errorf("failed to serialize notification: %w", err)
    }

    // Write the notification to stdout
    log.Printf("Sending notification: %s", string(notifBytes))
    return t.Send(notifBytes)
}

// RegisterRequestHandler registers a handler for incoming requests
func (t *StdioTransport) RegisterRequestHandler(method string, handler RequestHandler) {
    t.mutex.Lock()
    defer t.mutex.Unlock()
    t.requestHandlers[method] = handler
}

// RegisterNotificationHandler registers a handler for incoming notifications
func (t *StdioTransport) RegisterNotificationHandler(method string, handler NotificationHandler) {
    t.mutex.Lock()
    defer t.mutex.Unlock()
    t.notificationHandlers[method] = handler
}

// RegisterProgressHandler registers a handler for progress events
func (t *StdioTransport) RegisterProgressHandler(id interface{}, handler ProgressHandler) {
	// This is a placeholder implementation
}

// UnregisterProgressHandler removes a progress handler
func (t *StdioTransport) UnregisterProgressHandler(id interface{}) {
	// This is a placeholder implementation
}
