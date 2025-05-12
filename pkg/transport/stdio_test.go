package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStdioTransport(t *testing.T) {
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}
	tr := NewStdioTransport(reader, writer)

	assert.NotNil(t, tr, "NewStdioTransport should return a non-nil transport")
	assert.Equal(t, reader, tr.reader, "Reader should be initialized")
	assert.Equal(t, writer, tr.writer, "Writer should be initialized")
	assert.NotNil(t, tr.rawWriter, "rawWriter should be initialized")
	assert.NotNil(t, tr.BaseTransport, "BaseTransport should be initialized")
}

func TestStdioTransport_SendRawBytes(t *testing.T) {
	t.Log("TestStdioTransport_SendRawBytes: Setting up pipe")
	outR, outW := io.Pipe() // Transport's output is written to outW, read from outR

	t.Log("TestStdioTransport_SendRawBytes: Creating StdioTransport")
	// Use the constructor that accepts reader and writer
	tr := NewStdioTransport(strings.NewReader(""), outW) // Input reader is empty, not used in this Send test

	testData := []byte("hello world")
	buf := make([]byte, len(testData)+1) // +1 for newline
	expectedData := append(testData, '\n')
	readDone := make(chan error, 1)        // Channel to signal read completion and pass error
	var nRead int

	t.Log("TestStdioTransport_SendRawBytes: Starting reader goroutine")
	go func() {
		defer outR.Close() // Ensure reader pipe end is closed
		t.Log("TestStdioTransport_SendRawBytes (goroutine): Calling io.ReadFull")
		n, err := io.ReadFull(outR, buf)
		t.Logf("TestStdioTransport_SendRawBytes (goroutine): io.ReadFull returned (n=%d, err=%v)", n, err)
		nRead = n
		readDone <- err
	}()

	t.Logf("TestStdioTransport_SendRawBytes: Calling tr.Send with data: %s", string(testData))
	err := tr.Send(testData) // This should write "hello world\n" to outW
	t.Log("TestStdioTransport_SendRawBytes: tr.Send returned")
	require.NoError(t, err)

	// IMPORTANT: Close the writer part of the pipe AFTER the Send operation.
	// This signals EOF to the reader, allowing ReadFull to complete if it hasn't already.
	t.Log("TestStdioTransport_SendRawBytes: Closing outW (pipe writer)")
	err = outW.Close()
	if err != nil {
		t.Logf("TestStdioTransport_SendRawBytes: Error closing outW: %v", err)
		// We might get an error if the reader goroutine closed outR already due to its own error, fine to log.
	}
	t.Log("TestStdioTransport_SendRawBytes: outW closed")

	t.Log("TestStdioTransport_SendRawBytes: Waiting for reader goroutine to complete")
	readErr := <-readDone
	t.Logf("TestStdioTransport_SendRawBytes: Reader goroutine completed with error: %v", readErr)

	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		t.Fatalf("ReadFull from goroutine failed: %v", readErr)
	}

	t.Logf("TestStdioTransport_SendRawBytes: Asserting nRead (%d) == len(expectedData) (%d)", nRead, len(expectedData))
	assert.Equal(t, len(expectedData), nRead, "Number of bytes read should match expected")
	t.Logf("TestStdioTransport_SendRawBytes: Asserting buf[:nRead] (%s) == expectedData (%s)", string(buf[:nRead]), string(expectedData))
	assert.Equal(t, expectedData, buf[:nRead], "Data read should match sent data with newline")
	t.Log("TestStdioTransport_SendRawBytes: Test completed")
}

func TestStdioTransport_SendRequest_ReceiveResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Pipe for emulating server's stdin (transport's output)
	srvInR, srvInW := io.Pipe()
	// Pipe for emulating server's stdout (transport's input)
	srvOutR, srvOutW := io.Pipe()

	tr := NewStdioTransport(srvOutR, srvInW)
	go func() {
		if err := tr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "i/o operation on closed pipe") {
			t.Logf("Transport Start error: %v", err) // Use t.Logf for errors in goroutines
		}
	}()
	defer tr.Stop(ctx) // Use Stop instead of Close

	var wg sync.WaitGroup
	wg.Add(1)

	// Simulate server behavior
	go func() {
		defer wg.Done()
		defer srvInR.Close()
		defer srvOutW.Close()

		t.Log("[Test Server] Goroutine started, waiting to read request from clientOutR")
		reqBytes, err := readJSONLine(srvInR) // Reads from where the client writes requests
		if err != nil {
			t.Logf("[Test Server] Error reading request line: %v", err)
			// tr.Stop(ctx) // Not ideal to call Stop from here, let client timeout
			return
		}
		t.Logf("[Test Server] Read request line: %s", string(reqBytes))

		var req protocol.Request
		if err := json.Unmarshal(reqBytes, &req); err != nil {
			t.Logf("[Test Server] Error unmarshalling request: %v", err)
			return
		}
		t.Logf("[Test Server] Unmarshalled request ID: %s, Method: %s", req.ID, req.Method)

		// Send a response
		respMsg, _ := protocol.NewResponse(req.ID, map[string]string{"status": "ok"})
		respBytes, _ := json.Marshal(respMsg)
		t.Logf("[Test Server] Writing response to clientInW: %s", string(respBytes))
		if _, err := srvOutW.Write(append(respBytes, '\n')); err != nil {
			t.Logf("[Test Server] Error writing response: %v", err)
		}
		t.Log("[Test Server] Response written and clientInW closed by defer")
	}()

	// Client sends request
	params := map[string]string{"param1": "value1"}
	respInterface, err := tr.SendRequest(ctx, "test.method", params)

	require.NoError(t, err)
	require.NotNil(t, respInterface)
	
	resp, ok := respInterface.(*protocol.Response)
	require.True(t, ok, "Response should be of type *protocol.Response")

	assert.Nil(t, resp.Error, "Response error should be nil")

	var resultData map[string]string
	err = json.Unmarshal(resp.Result, &resultData)
	require.NoError(t, err)
	assert.Equal(t, "ok", resultData["status"])

	wg.Wait() // Wait for server goroutine to finish assertions
}

func TestStdioTransport_ReceiveRequest_SendResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cltInR, cltInW := io.Pipe()   // Transport's input (client writes here)
	cltOutR, cltOutW := io.Pipe() // Transport's output (client reads here)

	tr := NewStdioTransport(cltInR, cltOutW)

	// Register a request handler
	type EchoRequestParams struct {
		Data string `json:"data"`
	}
	type EchoResponseResult struct {
		EchoResponse EchoRequestParams `json:"echo_response"`
	}

	echoRequestHandler := func(ctx context.Context, params interface{}) (interface{}, error) {
		rawParams, ok := params.(json.RawMessage)
		if !ok {
			return nil, fmt.Errorf("echoRequestHandler: expected json.RawMessage, got %T", params)
		}
		t.Logf("echoRequestHandler: received rawParams: %s", string(rawParams))
		var parsedParams EchoRequestParams
		if err := json.Unmarshal(rawParams, &parsedParams); err != nil {
			t.Logf("echoRequestHandler: error unmarshalling params: %v", err)
			return nil, fmt.Errorf("error unmarshalling echo params: %w", err)
		}
		t.Logf("echoRequestHandler: unmarshalled params: %+v", parsedParams)

		return &EchoResponseResult{EchoResponse: EchoRequestParams{Data: parsedParams.Data}}, nil
	}

	tr.RegisterRequestHandler("echo", echoRequestHandler)

	go func() {
		if err := tr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "i/o operation on closed pipe") {
			t.Logf("Transport Start error: %v", err)
		}
	}()
	defer tr.Stop(ctx) // Use Stop instead of Close

	// Simulate client sending a request
	go func() {
		defer cltInW.Close()
		reqParams := map[string]string{"data": "hello server"}
		reqMsg, _ := protocol.NewRequest(1, "echo", reqParams)
		reqBytes, _ := json.Marshal(reqMsg)
		_, err := cltInW.Write(append(reqBytes, '\n'))
		if err != nil {
			t.Logf("Client: failed to write request: %v", err)
		}
	}()

	// Read the response from transport's output
	respBytes, err := readJSONLine(cltOutR)
	defer cltOutR.Close()
	require.NoError(t, err, "Failed to read response from transport")

	var resp protocol.Response
	err = json.Unmarshal(respBytes, &resp)
	require.NoError(t, err, "Failed to unmarshal response")

	var resultData map[string]interface{}
	err = json.Unmarshal(resp.Result, &resultData)
	require.NoError(t, err)
	assert.NotNil(t, resultData["echo_response"])
	if echoResp, ok := resultData["echo_response"].(map[string]interface{}); ok {
		assert.Equal(t, "hello server", echoResp["data"])
	} else {
		t.Errorf("Echo response data of unexpected type: %T", resultData["echo_response"])
	}
}

func TestStdioTransport_ReceiveNotification(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cltInR, cltInW := io.Pipe() // Transport's input (client writes here)

	// Output pipe is not strictly needed for this test if no response is sent, but good practice
	_, cltOutW := io.Pipe()

	tr := NewStdioTransport(cltInR, cltOutW)

	type NotificationParams struct {
		EventData string `json:"event_data"`
	}
	notificationReceived := make(chan *NotificationParams, 1)

	testNotificationHandler := func(ctx context.Context, params interface{}) error {
		rawParams, ok := params.(json.RawMessage)
		if !ok {
			return fmt.Errorf("testNotificationHandler: expected json.RawMessage, got %T", params)
		}
		t.Logf("testNotificationHandler: received rawParams: %s", string(rawParams))
		var parsedParams NotificationParams
		if err := json.Unmarshal(rawParams, &parsedParams); err != nil {
			t.Logf("testNotificationHandler: error unmarshalling params: %v", err)
			return fmt.Errorf("error unmarshalling notification params: %w", err)
		}
		t.Logf("testNotificationHandler: unmarshalled params: %+v", parsedParams)

		notificationReceived <- &parsedParams
		return nil
	}

	tr.RegisterNotificationHandler("notify_event", testNotificationHandler)

	go func() {
		if err := tr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "i/o operation on closed pipe") {
			t.Logf("Transport Start error: %v", err)
		}
	}()
	defer tr.Stop(ctx)      // Use Stop instead of Close
	defer cltOutW.Close() // Close writer part of output pipe

	// Simulate client sending a notification
	notifParams := map[string]string{"event_data": "startup complete"}
	notifMsg, _ := protocol.NewNotification("notify_event", notifParams)
	notifBytes, _ := json.Marshal(notifMsg)

	_, err := cltInW.Write(append(notifBytes, '\n'))
	cltInW.Close() // Close input pipe to signal EOF to transport reader
	require.NoError(t, err)

	// Give transport time to process notification
	// In a real scenario, might need a channel to signal completion from handler
	time.Sleep(100 * time.Millisecond) 

	select {
	case params := <-notificationReceived:
		assert.NotNil(t, params, "Notification handler should have received params")
		assert.Equal(t, "startup complete", params.EventData)
	default:
		t.Errorf("Notification handler was not called")
	}
}

func TestStdioTransport_ProcessMessage_MalformedJSON(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cltInR, cltInW := io.Pipe()   // Transport's input
	_, cltOutW := io.Pipe() // Transport's output (not used for response here)

	tr := NewStdioTransport(cltInR, cltOutW)

	var errorHandlerCalled bool
	var receivedError error
	tr.SetErrorHandler(func(err error) {
		errorHandlerCalled = true
		receivedError = err
	})

	go func() {
		if err := tr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "i/o operation on closed pipe") && !strings.Contains(err.Error(), "unexpected EOF"){
				if !errorHandlerCalled { // Only log if error handler wasn't called, suggesting a different issue
					t.Logf("Transport Start error: %v", err)
				}
		}
	}()
	defer tr.Stop(ctx) // Use Stop instead of Close
	defer cltOutW.Close()

	malformedJSON := []byte("this is not json\n")
	_, err := cltInW.Write(malformedJSON)
	cltInW.Close() // Close input pipe
	require.NoError(t, err)

	// Wait for potential error handling
	time.Sleep(100 * time.Millisecond) 

	assert.True(t, errorHandlerCalled, "Error handler should have been called for malformed JSON")
	assert.NotNil(t, receivedError, "Received error should not be nil")
	t.Logf("Received error via handler: %v", receivedError)
	// Check if the error is a JSON unmarshaling error (this can be tricky as it might be wrapped)
	// For now, checking it's not nil is a good start.
	// Example: assert.Contains(t, receivedError.Error(), "invalid character 'h' looking for beginning of value")
}

// TestStdioTransport_SendNotification_NoHandler tests that sending a notification
// for which no handler is registered does not cause an error or panic.
func TestStdioTransport_SendNotification_NoHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cltInR, cltInW := io.Pipe() // Transport's input (client writes here)
	_, cltOutW := io.Pipe()     // Transport's output

	tr := NewStdioTransport(cltInR, cltOutW)

	// No handler registered for "unhandled_event"

	var errorHandlerCalled bool
	tr.SetErrorHandler(func(err error) { // Should not be called for unhandled notification
		errorHandlerCalled = true
		t.Errorf("Error handler called unexpectedly for unhandled notification: %v", err)
	})

	go func() {
		if err := tr.Start(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "i/o operation on closed pipe") && !strings.Contains(err.Error(), "unexpected EOF"){
				t.Logf("Transport Start error: %v", err)
		}
	}()
	defer tr.Stop(ctx) // Use Stop instead of Close
	defer cltOutW.Close()

	notifParams := map[string]string{"data": "test"}
	notifMsg, _ := protocol.NewNotification("unhandled_event", notifParams)
	notifBytes, _ := json.Marshal(notifMsg)

	_, err := cltInW.Write(append(notifBytes, '\n'))
	cltInW.Close() // Close input pipe to signal EOF
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond) // Allow time for processing

	assert.False(t, errorHandlerCalled, "Error handler should not be called for an unhandled notification")
	// No panic and no error in error handler is success.
}

// TestStdioTransport_ContextCancellation ensures the transport stops when context is canceled.
func TestStdioTransport_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cltInR, cltInPipeW := io.Pipe() // Transport's input, reader will block indefinitely
	_, cltOutW := io.Pipe() // Transport's output

	tr := NewStdioTransport(cltInR, cltOutW)

	startErrChan := make(chan error, 1)
	go func() {
		startErrChan <- tr.Start(ctx)
	}()

	time.Sleep(50 * time.Millisecond) // Give Start goroutine a chance to run

	cancel() // Cancel the context

	select {
	case err := <-startErrChan:
		assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || (err != nil && (strings.Contains(err.Error(), "i/o operation on closed pipe") || strings.Contains(err.Error(), "use of closed network connection"))), "Expected context.Canceled or io.EOF or closed pipe/network error, got %v", err)
	case <-time.After(1 * time.Second):
		t.Fatal("Transport did not stop after context cancellation")
	}
	// Also ensure Stop can be called without issue after context cancellation
	assert.NotPanics(t, func() { tr.Stop(ctx) }, "Stop should not panic")

	// Explicitly close the pipe writer to avoid resource leak in test, if not already closed by transport
	cltInPipeW.Close()
	cltOutW.Close()
}

// Helper to read a JSON line from a reader
func readJSONLine(r io.Reader) ([]byte, error) {
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return scanner.Bytes(), scanner.Err()
	}
	return nil, io.EOF
}
