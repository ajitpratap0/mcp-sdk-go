package errors

import (
	"encoding/json"
	"fmt"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// ToJSONRPCResponse converts any error to a JSON-RPC response
func ToJSONRPCResponse(err error, requestID interface{}) (*protocol.Response, error) {
	if err == nil {
		return nil, fmt.Errorf("cannot create error response from nil error")
	}

	// Check if it's already an MCPError
	if mcpErr, ok := AsMCPError(err); ok {
		return protocol.NewErrorResponse(requestID, mcpErr.Code(), mcpErr.Message(), mcpErr.Data())
	}

	// For non-MCP errors, wrap as internal error
	return protocol.NewErrorResponse(requestID, CodeInternalError, err.Error(), nil)
}

// ToJSONRPCError converts any error to a JSON-RPC error object
func ToJSONRPCError(err error) *protocol.Error {
	if err == nil {
		return nil
	}

	// Check if it's already an MCPError
	if mcpErr, ok := AsMCPError(err); ok {
		return &protocol.Error{
			Code:    mcpErr.Code(),
			Message: mcpErr.Message(),
			Data:    mcpErr.Data(),
		}
	}

	// For non-MCP errors, wrap as internal error
	return &protocol.Error{
		Code:    CodeInternalError,
		Message: err.Error(),
		Data:    nil,
	}
}

// FromJSONRPCError converts a JSON-RPC error to an MCPError
func FromJSONRPCError(jsonrpcErr *protocol.Error) MCPError {
	if jsonrpcErr == nil {
		return nil
	}

	// Get error info from registry
	category := GetErrorCodeCategory(jsonrpcErr.Code)
	severity := GetErrorCodeSeverity(jsonrpcErr.Code)

	err := NewError(jsonrpcErr.Code, jsonrpcErr.Message, category, severity)
	if jsonrpcErr.Data != nil {
		err = err.WithData(jsonrpcErr.Data)
	}

	return err
}

// FromProtocolErrorObject converts a protocol.ErrorObject to an MCPError
func FromProtocolErrorObject(errorObj *protocol.ErrorObject) MCPError {
	if errorObj == nil {
		return nil
	}

	// Get error info from registry
	category := GetErrorCodeCategory(errorObj.Code)
	severity := GetErrorCodeSeverity(errorObj.Code)

	err := NewError(errorObj.Code, errorObj.Message, category, severity)
	if errorObj.Data != nil {
		err = err.WithData(errorObj.Data)
	}

	return err
}

// ToProtocolError converts an MCPError to a protocol.Error
func ToProtocolError(err MCPError) *protocol.Error {
	if err == nil {
		return nil
	}

	return &protocol.Error{
		Code:    err.Code(),
		Message: err.Message(),
		Data:    err.Data(),
	}
}

// ToProtocolErrorObject converts an MCPError to a protocol.ErrorObject
func ToProtocolErrorObject(err MCPError) *protocol.ErrorObject {
	if err == nil {
		return nil
	}

	return &protocol.ErrorObject{
		Code:    err.Code(),
		Message: err.Message(),
		Data:    err.Data(),
	}
}

// WrapProtocolError wraps a standard Go error with additional MCP context
func WrapProtocolError(err error, method string, requestID interface{}) MCPError {
	if err == nil {
		return nil
	}

	// If it's already an MCPError, add context
	if mcpErr, ok := AsMCPError(err); ok {
		context := &Context{
			Method:    method,
			RequestID: fmt.Sprintf("%v", requestID),
		}
		return mcpErr.WithContext(context)
	}

	// Otherwise, wrap as internal error with context
	context := &Context{
		Method:    method,
		RequestID: fmt.Sprintf("%v", requestID),
	}

	return WrapError(
		err,
		CodeInternalError,
		fmt.Sprintf("Error processing %s", method),
		CategoryInternal,
		SeverityError,
	).WithContext(context)
}

// ConvertStandardError converts common Go errors to appropriate MCP errors
func ConvertStandardError(err error) MCPError {
	if err == nil {
		return nil
	}

	// Check if it's already an MCPError
	if mcpErr, ok := AsMCPError(err); ok {
		return mcpErr
	}

	// Common error patterns
	errStr := err.Error()

	// Context cancellation errors
	if errStr == "context canceled" {
		return OperationCancelled("request")
	}

	// Timeout errors
	if errStr == "context deadline exceeded" {
		return OperationTimeout("request", "unknown")
	}

	// JSON parsing errors
	if _, ok := err.(*json.SyntaxError); ok {
		return NewError(CodeParseError, "Invalid JSON", CategoryProtocol, SeverityError)
	}

	if _, ok := err.(*json.UnmarshalTypeError); ok {
		return NewError(CodeInvalidParams, "Invalid parameter type", CategoryValidation, SeverityError)
	}

	// Default to internal error
	return WrapError(err, CodeInternalError, "Internal error", CategoryInternal, SeverityError)
}

// CombineErrors combines multiple errors into a single MCPError
func CombineErrors(errors []error) MCPError {
	if len(errors) == 0 {
		return nil
	}

	// Filter out nil errors
	validErrors := make([]error, 0, len(errors))
	for _, err := range errors {
		if err != nil {
			validErrors = append(validErrors, err)
		}
	}

	if len(validErrors) == 0 {
		return nil
	}

	if len(validErrors) == 1 {
		return ConvertStandardError(validErrors[0])
	}

	// Multiple errors - create a combined error
	messages := make([]string, len(validErrors))
	errorData := make([]interface{}, len(validErrors))

	for i, err := range validErrors {
		messages[i] = err.Error()
		if mcpErr, ok := AsMCPError(err); ok {
			errorData[i] = mcpErr.ToJSON()
		} else {
			errorData[i] = map[string]interface{}{
				"message": err.Error(),
				"type":    fmt.Sprintf("%T", err),
			}
		}
	}

	return NewError(
		CodeInternalError,
		fmt.Sprintf("Multiple errors occurred: %v", messages),
		CategoryInternal,
		SeverityError,
	).WithData(map[string]interface{}{
		"errors": errorData,
		"count":  len(validErrors),
	})
}

// CreateMethodNotFoundError creates a standardized method not found error
func CreateMethodNotFoundError(method string, requestID interface{}) MCPError {
	context := &Context{
		Method:    method,
		RequestID: fmt.Sprintf("%v", requestID),
	}

	return NewError(
		CodeMethodNotFound,
		fmt.Sprintf("Method not found: %s", method),
		CategoryProtocol,
		SeverityError,
	).WithContext(context)
}

// CreateInvalidParamsError creates a standardized invalid params error
func CreateInvalidParamsError(method string, requestID interface{}, details string) MCPError {
	context := &Context{
		Method:    method,
		RequestID: fmt.Sprintf("%v", requestID),
	}

	message := "Invalid method parameters"
	if details != "" {
		message = fmt.Sprintf("Invalid method parameters: %s", details)
	}

	return NewError(
		CodeInvalidParams,
		message,
		CategoryValidation,
		SeverityError,
	).WithContext(context).WithDetail(details)
}

// CreateParseError creates a standardized parse error
func CreateParseError(details string) MCPError {
	message := "Parse error"
	if details != "" {
		message = fmt.Sprintf("Parse error: %s", details)
	}

	return NewError(
		CodeParseError,
		message,
		CategoryProtocol,
		SeverityError,
	).WithDetail(details)
}

// CreateInvalidRequestError creates a standardized invalid request error
func CreateInvalidRequestError(details string) MCPError {
	message := "Invalid Request"
	if details != "" {
		message = fmt.Sprintf("Invalid Request: %s", details)
	}

	return NewError(
		CodeInvalidRequest,
		message,
		CategoryProtocol,
		SeverityError,
	).WithDetail(details)
}

// CreateInternalError creates a standardized internal error with optional context
func CreateInternalError(operation string, cause error) MCPError {
	message := "Internal error"
	if operation != "" {
		message = fmt.Sprintf("Internal error during %s", operation)
	}

	err := WrapError(
		cause,
		CodeInternalError,
		message,
		CategoryInternal,
		SeverityError,
	)

	if operation != "" {
		context := &Context{
			Operation: operation,
		}
		err = err.WithContext(context)
	}

	return err
}

// IsRetryableError checks if an error is retryable based on its properties
func IsRetryableError(err error) bool {
	if mcpErr, ok := AsMCPError(err); ok {
		// Check if the error data indicates it's retryable
		if data := mcpErr.Data(); data != nil {
			if dataMap, ok := data.(map[string]interface{}); ok {
				if retryable, exists := dataMap["retryable"]; exists {
					if retryableBool, ok := retryable.(bool); ok {
						return retryableBool
					}
				}
			}
		}

		// Check by category and code
		switch mcpErr.Category() {
		case CategoryTimeout, CategoryTransport:
			return true
		case CategoryCancelled:
			return false
		}

		// Check by specific error codes
		switch mcpErr.Code() {
		case CodeConnectionFailed, CodeConnectionLost, CodeConnectionTimeout,
			CodeOperationTimeout, CodeResourceUnavailable, CodeProviderUnavailable:
			return true
		case CodeResourceNotFound, CodeMethodNotFound, CodeInvalidParams,
			CodeUnauthorized, CodeInvalidCapability:
			return false
		}
	}

	// Default based on error type
	errStr := err.Error()
	return errStr == "context deadline exceeded" ||
		errStr == "connection refused" ||
		errStr == "connection reset by peer"
}
