// Package transport provides transport mechanisms for the MCP protocol.
package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// StdioTransport implements Transport using stdin/stdout.
// According to the MCP specification, clients SHOULD support stdio transport
// whenever possible as it's the most widely supported transport mechanism.
type StdioTransport struct {
	*BaseTransport
	reader          io.Reader
	writer          io.Writer
	bufWriter       *bufio.Writer
	scanner         *bufio.Scanner
	running         atomic.Bool
	mu              sync.Mutex
	options         *Options
	requestIDPrefix string
}

// NewStdioTransport creates a new stdio transport using os.Stdin and os.Stdout.
// This is the recommended default transport according to the MCP specification.
func NewStdioTransport(options ...Option) *StdioTransport {
	return NewStdioTransportWithStreams(os.Stdin, os.Stdout, options...)
}

// NewStdioTransportWithStreams creates a new stdio transport with the given streams.
// This allows using custom readers and writers instead of stdin/stdout.
func NewStdioTransportWithStreams(reader io.Reader, writer io.Writer, options ...Option) *StdioTransport {
	opts := NewOptions(options...)
	scanner := bufio.NewScanner(reader)

	// Use a 1MB buffer to handle large JSON messages
	const maxScanTokenSize = 1024 * 1024
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	// Create a buffered writer to improve performance while ensuring line-buffering
	bufWriter := bufio.NewWriter(writer)

	return &StdioTransport{
		BaseTransport:   NewBaseTransport(),
		reader:          reader,
		writer:          writer,
		bufWriter:       bufWriter,
		scanner:         scanner,
		options:         opts,
		requestIDPrefix: "stdio",
	}
}

// SetRequestIDPrefix sets the prefix for request IDs
func (t *StdioTransport) SetRequestIDPrefix(prefix string) {
	t.requestIDPrefix = prefix
}

// Initialize sets up the transport
func (t *StdioTransport) Initialize(ctx context.Context) error {
	// No specific initialization needed for stdio
	return nil
}

// SendRequest sends a request and waits for the response
func (t *StdioTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	id := fmt.Sprintf("%s-%d", t.requestIDPrefix, t.GetNextID())

	req, err := protocol.NewRequest(id, method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, t.options.RequestTimeout)
	defer cancel()

	if err := t.sendMessage(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	resp, err := t.WaitForResponse(reqCtx, id)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("server error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	return resp.Result, nil
}

// SendNotification sends a notification (one-way message)
func (t *StdioTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	notif, err := protocol.NewNotification(method, params)
	if err != nil {
		return fmt.Errorf("failed to create notification: %w", err)
	}

	return t.sendMessage(notif)
}

// Start begins processing messages (blocking)
func (t *StdioTransport) Start(ctx context.Context) error {
	if !t.running.CompareAndSwap(false, true) {
		return fmt.Errorf("transport already running")
	}

	defer t.running.Store(false)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if !t.scanner.Scan() {
				if err := t.scanner.Err(); err != nil {
					return fmt.Errorf("scanner error: %w", err)
				}
				// EOF
				return nil
			}

			line := t.scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}

			if err := t.handleMessage(ctx, []byte(line)); err != nil {
				log.Printf("Error handling message: %v", err)
			}
		}
	}
}

// Stop gracefully shuts down the transport
func (t *StdioTransport) Stop(ctx context.Context) error {
	// Nothing to close for stdio
	return nil
}

// sendMessage sends a message to the writer.
// It ensures the message is written as a single line with proper line-buffering
// to guarantee that messages are sent immediately.
func (t *StdioTransport) sendMessage(message interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Write message as a single line
	if _, err = t.bufWriter.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Add newline
	if err = t.bufWriter.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush to ensure the message is sent immediately
	if err = t.bufWriter.Flush(); err != nil {
		return fmt.Errorf("failed to flush buffer: %w", err)
	}

	return nil
}

// handleMessage processes an incoming JSON-RPC message
func (t *StdioTransport) handleMessage(ctx context.Context, data []byte) error {
	if protocol.IsRequest(data) {
		var req protocol.Request
		if err := json.Unmarshal(data, &req); err != nil {
			return fmt.Errorf("failed to unmarshal request: %w", err)
		}

		resp, err := t.HandleRequest(ctx, &req)
		if err != nil {
			errResp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, err.Error(), nil)
			return t.sendMessage(errResp)
		}

		return t.sendMessage(resp)

	} else if protocol.IsResponse(data) {
		var resp protocol.Response
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}

		t.HandleResponse(&resp)
		return nil

	} else if protocol.IsNotification(data) {
		var notif protocol.Notification
		if err := json.Unmarshal(data, &notif); err != nil {
			return fmt.Errorf("failed to unmarshal notification: %w", err)
		}

		return t.HandleNotification(ctx, &notif)

	} else {
		return fmt.Errorf("unknown message type: %s", string(data))
	}
}
