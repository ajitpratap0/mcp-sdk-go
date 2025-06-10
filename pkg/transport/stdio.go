// Package transport provides various transport mechanisms for MCP communication.
package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"

	"golang.org/x/sync/errgroup"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// StdioTransport implements the Transport interface using standard input and output.
// This is the recommended transport mechanism in the MCP specification for command-line
// tools and applications where the client and server are typically connected via pipes.
type StdioTransport struct {
	*BaseTransport               // Embedded BaseTransport
	reader         io.Reader     // Changed from *bufio.Reader for flexibility
	writer         io.Writer     // Changed from *bufio.Writer for flexibility
	rawWriter      *bufio.Writer // Internal buffered writer
	errorHandler   ErrorHandler  // For low-level I/O errors
	mutex          sync.RWMutex  // Protects writer and errorHandler
	done           chan struct{}
	stopOnce       sync.Once // Ensures Stop logic runs once
}

// newStdioTransport creates a new Stdio transport from config
func newStdioTransport(config TransportConfig) (Transport, error) {
	// Use custom readers/writers if provided (for testing), otherwise use os.Stdin/Stdout
	reader := config.StdioReader
	writer := config.StdioWriter

	if reader == nil {
		reader = os.Stdin
	}
	if writer == nil {
		writer = os.Stdout
	}

	return &StdioTransport{
		BaseTransport: NewBaseTransport(),
		reader:        reader,
		writer:        writer,
		rawWriter:     bufio.NewWriter(writer),
		done:          make(chan struct{}),
	}, nil
}

// Initialize prepares the transport for use.
// For StdioTransport, this is a no-op as the stdin/stdout are already available.
func (t *StdioTransport) Initialize(ctx context.Context) error {
	return nil
}

// Start begins reading messages from stdin and processing them.
// This method blocks until the context is canceled or an error occurs.
func (t *StdioTransport) Start(ctx context.Context) error {
	// Create errgroup for coordinated goroutine management
	g, gctx := errgroup.WithContext(ctx)

	// Create a scanner for reading lines from the reader
	scanner := bufio.NewScanner(t.reader)

	// Channel for coordination between scanner and main loop
	scannerDone := make(chan struct{})

	// Start scanner goroutine
	g.Go(func() error {
		defer close(scannerDone)

		// Read lines until EOF or error
		for scanner.Scan() {
			select {
			case <-gctx.Done():
				return gctx.Err()
			case <-t.done:
				return nil
			default:
				// Get the line (should be a complete JSON message)
				line := scanner.Bytes()

				// Copy the line to avoid it being overwritten by the next Scan
				data := make([]byte, len(line))
				copy(data, line)

				// Process the message with error recovery
				func() {
					defer func() {
						if r := recover(); r != nil {
							stackTrace := string(debug.Stack())
							t.BaseTransport.Logf("ERROR: Panic in message processing: %v\nStack trace:\n%s", r, stackTrace)
						}
					}()
					t.processMessage(data)
				}()
			}
		}

		// Check for scanner errors
		if err := scanner.Err(); err != nil && err != io.EOF {
			return mcperrors.StdioTransportError("read_input", err).
				WithContext(&mcperrors.Context{
					Component: "StdioTransport",
					Operation: "scan_input",
				})
		}
		return nil
	})

	// Start context monitoring goroutine to handle cleanup
	g.Go(func() error {
		select {
		case <-gctx.Done():
			// Close the reader to unblock scanner.Scan()
			if closer, ok := t.reader.(io.Closer); ok {
				_ = closer.Close() // Ignore error on forced close
			}
			return gctx.Err()
		case <-t.done:
			// Close the reader to unblock scanner.Scan()
			if closer, ok := t.reader.(io.Closer); ok {
				_ = closer.Close() // Ignore error on forced close
			}
			return nil
		case <-scannerDone:
			// Scanner finished normally
			return nil
		}
	})

	// Wait for all goroutines to complete
	return g.Wait()
}

// Stop halts the transport and cleans up resources.
func (t *StdioTransport) Stop(ctx context.Context) error {
	var flushErr error

	t.stopOnce.Do(func() {
		close(t.done) // Signal all loops relying on t.done to stop

		// Note: errgroup.Wait() in Start() will handle coordinated shutdown
		// No need for manual timeout here

		// Flush the writer before cleanup
		t.mutex.Lock()
		if t.rawWriter != nil {
			flushErr = t.rawWriter.Flush()
		}

		// Clear error handler
		t.errorHandler = nil
		t.mutex.Unlock()

		// Clean up BaseTransport resources
		t.BaseTransport.Cleanup()
	})

	if flushErr != nil {
		return mcperrors.StdioTransportError("stop", flushErr).
			WithContext(&mcperrors.Context{
				Component: "StdioTransport",
				Operation: "flush_on_stop",
			})
	}
	return nil
}

// Send transmits a message over the transport.
// For StdioTransport, this writes a line to stdout.
func (t *StdioTransport) Send(data []byte) error {
	// Acquire a lock to prevent concurrent writes
	t.mutex.Lock()
	defer t.mutex.Unlock()

	if t.rawWriter == nil {
		return mcperrors.TransportNotInitialized("stdio")
	}

	// Write the message followed by a newline
	if _, err := t.rawWriter.Write(data); err != nil {
		return mcperrors.StdioTransportError("send_message", err).
			WithContext(&mcperrors.Context{
				Component: "StdioTransport",
				Operation: "write_data",
			})
	}

	// Write a newline to terminate the message
	if err := t.rawWriter.WriteByte('\n'); err != nil {
		return mcperrors.StdioTransportError("send_message", err).
			WithContext(&mcperrors.Context{
				Component: "StdioTransport",
				Operation: "write_newline",
			})
	}

	// Flush the buffer to ensure the message is sent
	if err := t.rawWriter.Flush(); err != nil {
		return mcperrors.StdioTransportError("send_message", err).
			WithContext(&mcperrors.Context{
				Component: "StdioTransport",
				Operation: "flush_output",
			})
	}

	return nil
}

// SetErrorHandler sets the handler for transport errors.
func (t *StdioTransport) SetErrorHandler(handler ErrorHandler) {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.errorHandler = handler
}

// processMessage handles a received message by validating it and passing it to the receive handler.
func (t *StdioTransport) processMessage(data []byte) {
	// Recover from panics in message processing
	defer func() {
		if r := recover(); r != nil {
			stackTrace := string(debug.Stack())
			t.BaseTransport.Logf("ERROR: Panic in processMessage: %v\nStack trace:\n%s", r, stackTrace)
			t.handleError(fmt.Errorf("panic processing message: %v", r))
		}
	}()

	t.BaseTransport.Logf("StdioTransport.processMessage: Received raw data: %s", string(data))
	// Attempt to determine message type by unmarshalling into a generic map
	var genericMsg map[string]interface{}
	if err := json.Unmarshal(data, &genericMsg); err != nil {
		t.handleError(fmt.Errorf("error unmarshalling generic message: %w", err))
		t.BaseTransport.Logf("StdioTransport.processMessage: Error unmarshalling generic message: %v. Data: %s", err, string(data))
		return
	}

	_, hasMethod := genericMsg["method"]
	_, hasID := genericMsg["id"]
	// Distinguish between Response (hasID and (hasResult or hasError)) and Request (hasID and hasMethod)
	// Notification hasMethod and (usually) no ID, or if ID is present, no result/error.
	isResponse := hasID && (genericMsg["result"] != nil || genericMsg["error"] != nil)
	isRequest := hasID && hasMethod && !isResponse // Ensure it's not also a response structure
	isNotification := hasMethod && !hasID          // Simplest notification form

	t.BaseTransport.Logf("StdioTransport.processMessage: Parsed generic: hasMethod=%t, hasID=%t, isResponse=%t, isRequest=%t, isNotification=%t", hasMethod, hasID, isResponse, isRequest, isNotification)

	// More robust check for notifications that might include an ID (though atypical for JSON-RPC notifs)
	if hasMethod && hasID && !isRequest && !isResponse {
		t.BaseTransport.Logf("StdioTransport.processMessage: Re-classifying as notification based on robust check")
		isNotification = true
	}

	ctx := context.Background() // Or derive from transport's main context

	if isRequest { // It's a Request
		t.BaseTransport.Logf("StdioTransport.processMessage: Handling as Request. Data: %s", string(data))
		var req protocol.Request
		if err := json.Unmarshal(data, &req); err != nil {
			t.handleError(fmt.Errorf("error unmarshalling request: %w", err))
			t.BaseTransport.Logf("StdioTransport.processMessage: Error unmarshalling request: %v. Data: %s", err, string(data))
			return
		}
		resp, err := t.BaseTransport.HandleRequest(ctx, &req)
		if err != nil { // This error is from the handler execution itself
			t.handleError(fmt.Errorf("HandleRequest for %v returned error: %w", req.ID, err))
		}
		if resp != nil {
			respData, marshalErr := json.Marshal(resp)
			if marshalErr != nil {
				t.handleError(fmt.Errorf("error marshalling response for request %v: %w", req.ID, marshalErr))
				return
			}
			t.BaseTransport.Logf("StdioTransport.processMessage: Sending response for request %v: %s", req.ID, string(respData))
			if sendErr := t.Send(respData); sendErr != nil {
				t.handleError(fmt.Errorf("error sending response for request %v: %w", req.ID, sendErr))
			}
		}

	} else if isResponse { // It's a Response
		t.BaseTransport.Logf("StdioTransport.processMessage: Handling as Response. Data: %s", string(data))
		var resp protocol.Response
		if err := json.Unmarshal(data, &resp); err != nil {
			t.handleError(fmt.Errorf("error unmarshalling response: %w", err))
			t.BaseTransport.Logf("StdioTransport.processMessage: Error unmarshalling response: %v. Data: %s", err, string(data))
			return
		}
		t.BaseTransport.Logf("StdioTransport.processMessage: Calling BaseTransport.HandleResponse for ID %v", resp.ID)
		t.BaseTransport.HandleResponse(&resp)
		t.BaseTransport.Logf("StdioTransport.processMessage: BaseTransport.HandleResponse for ID %v returned", resp.ID)

	} else if isNotification { // It's a Notification
		t.BaseTransport.Logf("StdioTransport.processMessage: Handling as Notification. Data: %s", string(data))
		var notif protocol.Notification
		if err := json.Unmarshal(data, &notif); err != nil {
			t.handleError(fmt.Errorf("error unmarshalling notification: %w", err))
			t.BaseTransport.Logf("StdioTransport.processMessage: Error unmarshalling notification: %v. Data: %s", err, string(data))
			return
		}
		if err := t.BaseTransport.HandleNotification(ctx, &notif); err != nil {
			// For unregistered notification methods, just log the error instead of treating it as a real error
			// This is common practice for JSON-RPC notifications since they are meant to be fire-and-forget
			if errors.Is(err, ErrUnsupportedMethod) {
				t.BaseTransport.Logf("Ignoring notification for unregistered method: %s", notif.Method)
			} else {
				// For any other errors during notification handling, report them normally
				t.handleError(fmt.Errorf("error handling notification %s: %w", notif.Method, err))
			}
		}

	} else {
		t.handleError(fmt.Errorf("unknown message type received (method: %v, id: %v, result: %v, error: %v): %s", hasMethod, hasID, genericMsg["result"] != nil, genericMsg["error"] != nil, string(data)))
	}
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
func (t *StdioTransport) GenerateID() string {
	return t.BaseTransport.GenerateID()
}

// SendRequest sends a request and returns the response
func (t *StdioTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	t.Logf("SendRequest: Method=%s, Params=%+v", method, params)
	id := t.BaseTransport.GenerateID()

	// Ensure ID is stored as a string in pendingRequests map before sending the request
	stringID := fmt.Sprintf("%v", id)

	req, err := protocol.NewRequest(stringID, method, params) // Use stringID to ensure consistent type
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	bytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request: %w", err)
	}

	t.Logf("SendRequest: Sending marshalled request: %s", string(bytes))
	if err := t.Send(bytes); err != nil {
		return nil, fmt.Errorf("error sending request bytes: %w", err)
	}

	t.Logf("SendRequest: Request sent, waiting for response ID %s", stringID)
	// WaitForResponse should return *protocol.Response or an error
	pResp, err := t.BaseTransport.WaitForResponse(ctx, stringID)
	if err != nil {
		t.Logf("SendRequest: Error waiting for response ID %s: %v", stringID, err)
		return nil, fmt.Errorf("error waiting for response: %w", err)
	}

	t.Logf("SendRequest: Received response for ID %s: %+v", stringID, pResp)

	// If the response itself contains a JSON-RPC error, return that error object.
	// The client.Client layer is responsible for interpreting this.
	if pResp.Error != nil {
		return nil, pResp.Error // pResp.Error is now a valid error type
	}

	// On success (no transport error, no JSON-RPC error), return the full response object.
	return pResp, nil
}

// SendNotification sends a notification (one-way message)
func (t *StdioTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	// Note: BaseTransport doesn't have a SendNotification method that directly takes method/params
	// and handles marshaling + sending. It has HandleNotification for incoming.
	// We construct the notification and use StdioTransport's Send.
	notification, err := protocol.NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("error creating notification: %w", err)
	}
	jsonData, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("error marshalling notification: %w", err)
	}
	return t.Send(jsonData) // Uses StdioTransport's Send to write to stdout
}

// RegisterRequestHandler registers a handler for incoming requests
func (t *StdioTransport) RegisterRequestHandler(method string, handler RequestHandler) {
	t.BaseTransport.RegisterRequestHandler(method, handler)
}

// RegisterNotificationHandler registers a handler for incoming notifications
func (t *StdioTransport) RegisterNotificationHandler(method string, handler NotificationHandler) {
	t.BaseTransport.RegisterNotificationHandler(method, handler)
}

// RegisterProgressHandler registers a handler for progress events
func (t *StdioTransport) RegisterProgressHandler(id interface{}, handler ProgressHandler) {
	// This is a placeholder implementation
}

// UnregisterProgressHandler removes a progress handler
func (t *StdioTransport) UnregisterProgressHandler(id interface{}) {
	// This is a placeholder implementation
}
