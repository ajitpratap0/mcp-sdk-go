package client

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/logging"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// StreamingUpdateHandler is a function that processes streaming updates from a tool call
type StreamingUpdateHandler func(json.RawMessage)

// CallToolStreaming invokes a tool on the server with streaming updates
func (c *ClientConfig) CallToolStreaming(ctx context.Context, name string, input interface{}, toolContext interface{}, updateHandler StreamingUpdateHandler) (*protocol.CallToolResult, error) {
	if err := c.requireCapability(protocol.CapabilityTools, "CallToolStreaming"); err != nil {
		return nil, err
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
			return nil, mcperrors.CreateInternalError("marshal_input", err).
				WithContext(&mcperrors.Context{
					Component: "Client",
					Operation: "CallToolStreaming",
					Timestamp: time.Now(),
				}).
				WithDetail(fmt.Sprintf("Tool: %s, input type: %T", name, input))
		}
		params.Input = inputJSON
	}

	// Marshal context if provided
	if toolContext != nil {
		contextJSON, err := json.Marshal(toolContext)
		if err != nil {
			return nil, mcperrors.CreateInternalError("marshal_context", err).
				WithContext(&mcperrors.Context{
					Component: "Client",
					Operation: "CallToolStreaming",
					Timestamp: time.Now(),
				}).
				WithDetail(fmt.Sprintf("Tool: %s, context type: %T", name, toolContext))
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
			Done bool            `json:"done"`
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
			"name":      name,
			"streaming": true,
			"requestId": requestID,
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
		cancel()  // Cancel the streaming context
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
func (c *ClientConfig) registerProgressHandler(requestID string, handler func(json.RawMessage)) {
	// Create a ProgressHandler that adapts our handler to the transport's expected type
	progressHandler := func(params interface{}) error {
		// Try to extract the JSON data from params
		if data, ok := params.(json.RawMessage); ok {
			handler(data)
		} else {
			// If it's not already RawMessage, try to marshal it
			if jsonData, err := json.Marshal(params); err == nil {
				handler(jsonData)
			} else {
				c.logger.Error("Failed to marshal progress data", logging.ErrorField(err))
			}
		}
		return nil
	}

	// Register the handler with the transport
	c.transport.RegisterProgressHandler(requestID, progressHandler)
}
