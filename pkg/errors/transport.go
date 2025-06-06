package errors

import (
	"fmt"
	"net/url"
	"time"
)

// TransportErrorData contains structured data for transport-related errors
type TransportErrorData struct {
	Transport    string        `json:"transport"`
	Operation    string        `json:"operation,omitempty"`
	Endpoint     string        `json:"endpoint,omitempty"`
	Connected    bool          `json:"connected"`
	Retryable    bool          `json:"retryable"`
	RetryAfter   time.Duration `json:"retry_after,omitempty"`
	Reason       string        `json:"reason,omitempty"`
	StatusCode   int           `json:"status_code,omitempty"`
	ResponseTime time.Duration `json:"response_time,omitempty"`
}

// ConnectionErrorData contains structured data for connection-related errors
type ConnectionErrorData struct {
	Transport     string        `json:"transport"`
	Endpoint      string        `json:"endpoint,omitempty"`
	LocalAddress  string        `json:"local_address,omitempty"`
	RemoteAddress string        `json:"remote_address,omitempty"`
	Timeout       time.Duration `json:"timeout,omitempty"`
	Retryable     bool          `json:"retryable"`
	Reason        string        `json:"reason,omitempty"`
}

// TransportError creates a generic transport error
func TransportError(transport, operation string, cause error) MCPError {
	message := fmt.Sprintf("%s transport error", transport)
	if operation != "" {
		message = fmt.Sprintf("%s transport error during %s", transport, operation)
	}
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	return WrapError(
		cause,
		CodeTransportError,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: transport,
		Operation: operation,
		Connected: false,
		Retryable: true,
		Reason:    cause.Error(),
	})
}

// ConnectionFailed creates an error for connection failures
func ConnectionFailed(transport, endpoint string, cause error) MCPError {
	message := fmt.Sprintf("Failed to connect via %s", transport)
	if endpoint != "" {
		message = fmt.Sprintf("Failed to connect to %s via %s", endpoint, transport)
	}
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	// Parse endpoint for additional details
	var endpointData string
	if endpoint != "" {
		if u, err := url.Parse(endpoint); err == nil {
			endpointData = u.Host
		} else {
			endpointData = endpoint
		}
	}

	return WrapError(
		cause,
		CodeConnectionFailed,
		message,
		CategoryTransport,
		SeverityCritical,
	).WithData(&ConnectionErrorData{
		Transport: transport,
		Endpoint:  endpointData,
		Retryable: true,
		Reason:    cause.Error(),
	})
}

// ConnectionLost creates an error for lost connections
func ConnectionLost(transport, endpoint string, cause error) MCPError {
	message := fmt.Sprintf("Lost connection via %s", transport)
	if endpoint != "" {
		message = fmt.Sprintf("Lost connection to %s via %s", endpoint, transport)
	}
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	return WrapError(
		cause,
		CodeConnectionLost,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(&ConnectionErrorData{
		Transport: transport,
		Endpoint:  endpoint,
		Retryable: true,
		Reason:    cause.Error(),
	})
}

// ConnectionTimeout creates an error for connection timeouts
func ConnectionTimeout(transport, endpoint string, timeout time.Duration) MCPError {
	message := fmt.Sprintf("Connection timeout via %s", transport)
	if endpoint != "" {
		message = fmt.Sprintf("Connection timeout to %s via %s", endpoint, transport)
	}
	if timeout > 0 {
		message = fmt.Sprintf("%s after %v", message, timeout)
	}

	return NewError(
		CodeConnectionTimeout,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(&ConnectionErrorData{
		Transport: transport,
		Endpoint:  endpoint,
		Timeout:   timeout,
		Retryable: true,
		Reason:    "timeout",
	})
}

// HTTPTransportError creates an error for HTTP transport issues
func HTTPTransportError(operation, endpoint string, statusCode int, cause error) MCPError {
	message := fmt.Sprintf("HTTP transport error during %s", operation)
	if statusCode > 0 {
		message = fmt.Sprintf("HTTP %d error during %s", statusCode, operation)
	}
	if endpoint != "" {
		message = fmt.Sprintf("%s to %s", message, endpoint)
	}
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	// Determine if the error is retryable based on status code
	retryable := statusCode >= 500 || statusCode == 429 || statusCode == 408

	return WrapError(
		cause,
		CodeTransportError,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport:  "http",
		Operation:  operation,
		Endpoint:   endpoint,
		Connected:  statusCode > 0, // We got a response
		Retryable:  retryable,
		StatusCode: statusCode,
		Reason:     cause.Error(),
	})
}

// StdioTransportError creates an error for stdio transport issues
func StdioTransportError(operation string, cause error) MCPError {
	message := fmt.Sprintf("Stdio transport error during %s", operation)
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	return WrapError(
		cause,
		CodeTransportError,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: "stdio",
		Operation: operation,
		Connected: true,  // Stdio is always "connected"
		Retryable: false, // Stdio errors are typically not retryable
		Reason:    cause.Error(),
	})
}

// StreamableHTTPError creates an error for streamable HTTP transport issues
func StreamableHTTPError(operation, endpoint, streamID string, cause error) MCPError {
	message := fmt.Sprintf("Streamable HTTP error during %s", operation)
	if streamID != "" {
		message = fmt.Sprintf("Streamable HTTP error during %s (stream: %s)", operation, streamID)
	}
	if endpoint != "" {
		message = fmt.Sprintf("%s to %s", message, endpoint)
	}
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	data := &TransportErrorData{
		Transport: "streamable_http",
		Operation: operation,
		Endpoint:  endpoint,
		Connected: false,
		Retryable: true,
		Reason:    cause.Error(),
	}

	// Add stream ID to the error data
	if streamID != "" {
		dataMap := map[string]interface{}{
			"transport": data.Transport,
			"operation": data.Operation,
			"endpoint":  data.Endpoint,
			"connected": data.Connected,
			"retryable": data.Retryable,
			"reason":    data.Reason,
			"stream_id": streamID,
		}
		return WrapError(
			cause,
			CodeTransportError,
			message,
			CategoryTransport,
			SeverityError,
		).WithData(dataMap)
	}

	return WrapError(
		cause,
		CodeTransportError,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(data)
}

// EventSourceError creates an error for Server-Sent Events issues
func EventSourceError(endpoint, reason string, cause error) MCPError {
	message := fmt.Sprintf("Event source error: %s", reason)
	if endpoint != "" {
		message = fmt.Sprintf("Event source error for %s: %s", endpoint, reason)
	}
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	return WrapError(
		cause,
		CodeTransportError,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: "sse",
		Operation: "event_stream",
		Endpoint:  endpoint,
		Connected: false,
		Retryable: true,
		Reason:    reason,
	})
}

// MessageSendError creates an error for message sending failures
func MessageSendError(transport, messageType string, cause error) MCPError {
	message := fmt.Sprintf("Failed to send %s message via %s", messageType, transport)
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	return WrapError(
		cause,
		CodeTransportError,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: transport,
		Operation: "send_message",
		Connected: true,
		Retryable: true,
		Reason:    cause.Error(),
	})
}

// MessageReceiveError creates an error for message receiving failures
func MessageReceiveError(transport, messageType string, cause error) MCPError {
	message := fmt.Sprintf("Failed to receive %s message via %s", messageType, transport)
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	return WrapError(
		cause,
		CodeTransportError,
		message,
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: transport,
		Operation: "receive_message",
		Connected: true,
		Retryable: true,
		Reason:    cause.Error(),
	})
}

// TransportNotInitialized creates an error for uninitialized transports
func TransportNotInitialized(transport string) MCPError {
	return NewError(
		CodeTransportError,
		fmt.Sprintf("%s transport is not initialized", transport),
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: transport,
		Operation: "check_initialization",
		Connected: false,
		Retryable: false,
		Reason:    "not initialized",
	})
}

// TransportAlreadyRunning creates an error for transports that are already running
func TransportAlreadyRunning(transport string) MCPError {
	return NewError(
		CodeTransportError,
		fmt.Sprintf("%s transport is already running", transport),
		CategoryTransport,
		SeverityWarning,
	).WithData(&TransportErrorData{
		Transport: transport,
		Operation: "start",
		Connected: true,
		Retryable: false,
		Reason:    "already running",
	})
}

// TransportNotRunning creates an error for operations on stopped transports
func TransportNotRunning(transport string) MCPError {
	return NewError(
		CodeTransportError,
		fmt.Sprintf("%s transport is not running", transport),
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: transport,
		Connected: false,
		Retryable: false,
		Reason:    "not running",
	})
}

// InvalidTransportConfiguration creates an error for invalid transport configurations
func InvalidTransportConfiguration(transport, parameter, reason string) MCPError {
	return NewError(
		CodeTransportError,
		fmt.Sprintf("Invalid %s transport configuration for parameter '%s': %s", transport, parameter, reason),
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: transport,
		Operation: "configure",
		Connected: false,
		Retryable: false,
		Reason:    fmt.Sprintf("invalid %s: %s", parameter, reason),
	})
}

// MessageTooLarge creates an error for messages that exceed size limits
func MessageTooLarge(transport string, messageSize, maxSize int64) MCPError {
	return NewError(
		CodeTransportError,
		fmt.Sprintf("Message size %d exceeds maximum allowed size %d for %s transport", messageSize, maxSize, transport),
		CategoryTransport,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: transport,
		Operation: "send_message",
		Connected: true,
		Retryable: false,
		Reason:    fmt.Sprintf("message size %d > max %d", messageSize, maxSize),
	})
}

// ResponseTimeout creates an error for response timeouts
func ResponseTimeout(transport, requestID string, timeout time.Duration) MCPError {
	message := fmt.Sprintf("Response timeout for request %s via %s", requestID, transport)
	if timeout > 0 {
		message = fmt.Sprintf("%s after %v", message, timeout)
	}

	return NewError(
		CodeOperationTimeout,
		message,
		CategoryTimeout,
		SeverityError,
	).WithData(&TransportErrorData{
		Transport: transport,
		Operation: "wait_response",
		Connected: true,
		Retryable: true,
		Reason:    "response timeout",
	})
}
