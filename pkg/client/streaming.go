package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/model-context-protocol/go-mcp/pkg/protocol"
)

// StreamingUpdateHandler is a function that processes streaming updates from a tool call
type StreamingUpdateHandler func(json.RawMessage)

// CallToolStreaming invokes a tool on the server with streaming updates
func (c *Client) CallToolStreaming(ctx context.Context, name string, input interface{}, toolContext interface{}, updateHandler StreamingUpdateHandler) (*protocol.CallToolResult, error) {
	if !c.HasCapability(protocol.CapabilityTools) {
		return nil, errors.New("server does not support tools")
	}

	// Create tool call parameters
	params := &protocol.CallToolParams{
		Name: name,
	}

	// We'll add streaming flag in the request parameters

	// Marshal input if provided
	if input != nil {
		inputJSON, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input: %w", err)
		}
		params.Input = inputJSON
	}

	// Marshal context if provided
	if toolContext != nil {
		contextJSON, err := json.Marshal(toolContext)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal context: %w", err)
		}
		params.Context = contextJSON
	}

	// Generate a unique request ID for this tool call
	requestID := fmt.Sprintf("%s-%d", name, nextRequestID())

	// Create a channel to receive progress updates
	updateCh := make(chan json.RawMessage, 10)
	doneCh := make(chan struct{})
	resultCh := make(chan *protocol.CallToolResult, 1)
	errCh := make(chan error, 1)

	// Set up a context with cancellation
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start a goroutine to handle streaming updates
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-streamCtx.Done():
				// Context cancelled
				return
			case <-doneCh:
				// Streaming completed
				return
			case update := <-updateCh:
				// Process update
				if updateHandler != nil {
					updateHandler(update)
				}
			}
		}
	}()

	// Register a progress handler for this request to receive streaming updates
	c.registerProgressHandler(requestID, func(progressData json.RawMessage) {
		// Extract the data from the progress notification and send it to the update channel
		var progress struct {
			Data json.RawMessage `json:"data"`
			Done bool           `json:"done"`
		}

		if err := json.Unmarshal(progressData, &progress); err == nil {
			if progress.Data != nil {
				// Send the data to the update channel
				update := progress.Data
				updateCh <- update
			}

			// If this is the final update, close the done channel
			if progress.Done {
				close(doneCh)
			}
		} else {
			// If we can't parse the progress data, just send it directly
			updateCh <- progressData
		}
	})

	// Start a goroutine to send the request and handle the response
	go func() {
		defer close(resultCh)
		defer close(errCh)

		// Metadata for this call is already included in streamingParams

		// Create custom parameters with streaming flag
		streamingParams := map[string]interface{}{
			"name":       name,
			"streaming":  true,
			"requestId":  requestID,
		}

		if params.Input != nil {
			streamingParams["input"] = params.Input
		}

		if params.Context != nil {
			streamingParams["context"] = params.Context
		}

		streamingParamsJSON, err := json.Marshal(streamingParams)
		if err != nil {
			errCh <- fmt.Errorf("failed to marshal streaming params: %w", err)
			return
		}

		// Send the streaming tool call request
		result, err := c.transport.SendRequest(ctx, protocol.MethodCallTool, streamingParamsJSON)
		if err != nil {
			errCh <- err
			return
		}

		// Parse the final result
		var callResult protocol.CallToolResult
		if err := parseResult(result, &callResult); err != nil {
			errCh <- fmt.Errorf("failed to parse call tool result: %w", err)
			return
		}

		// Send the result
		resultCh <- &callResult
	}()

	// Wait for the result or an error
	select {
	case <-ctx.Done():
		// Context cancelled
		return nil, ctx.Err()
	case err := <-errCh:
		// Error occurred
		return nil, err
	case result := <-resultCh:
		// Got a result
		cancel() // Cancel the streaming context
		wg.Wait() // Wait for streaming to complete
		return result, nil
	}
}

// Helper function to generate request IDs
var requestIDCounter int64
var requestIDMutex sync.Mutex

func nextRequestID() int64 {
	requestIDMutex.Lock()
	defer requestIDMutex.Unlock()
	requestIDCounter++
	return requestIDCounter
}

// Helper function to register a progress handler
func (c *Client) registerProgressHandler(requestID string, handler func(json.RawMessage)) {
	// In a real implementation, this would register the handler with the transport
	// For now, this is a placeholder since the actual protocol doesn't support streaming yet
}
