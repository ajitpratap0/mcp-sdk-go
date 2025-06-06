// Package errors provides structured error handling for the MCP SDK.
// It defines custom error types that map to JSON-RPC error codes and provide
// rich context for debugging and programmatic error handling.
package errors

import (
	"encoding/json"
	"fmt"
	"time"
)

// Category represents the type/category of an error for classification and handling
type Category string

const (
	CategoryValidation Category = "validation"
	CategoryAuth       Category = "auth"
	CategoryNotFound   Category = "not_found"
	CategoryTransport  Category = "transport"
	CategoryProvider   Category = "provider"
	CategoryInternal   Category = "internal"
	CategoryTimeout    Category = "timeout"
	CategoryCancelled  Category = "cancelled"
	CategoryProtocol   Category = "protocol"
)

// Severity indicates how critical an error is
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Context provides additional context about where and when an error occurred
type Context struct {
	RequestID  string                 `json:"request_id,omitempty"`
	Method     string                 `json:"method,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Component  string                 `json:"component,omitempty"`
	Operation  string                 `json:"operation,omitempty"`
	UserID     string                 `json:"user_id,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
}

// MCPError defines the interface for all MCP SDK errors
type MCPError interface {
	error

	// Code returns the JSON-RPC error code
	Code() int

	// Message returns a human-readable error message
	Message() string

	// Details returns detailed technical description for debugging
	Details() string

	// Data returns structured error data for programmatic handling
	Data() interface{}

	// Category returns the error category for classification
	Category() Category

	// Severity returns the error severity level
	Severity() Severity

	// Context returns the error context information
	Context() *Context

	// WithContext returns a new error with the provided context
	WithContext(ctx *Context) MCPError

	// WithDetail returns a new error with additional detail
	WithDetail(detail string) MCPError

	// WithData returns a new error with structured data
	WithData(data interface{}) MCPError

	// Unwrap returns the underlying error for error chain traversal
	Unwrap() error

	// ToJSON returns the error as a JSON-serializable map
	ToJSON() map[string]interface{}
}

// baseError implements the MCPError interface
type baseError struct {
	code     int
	message  string
	details  string
	data     interface{}
	category Category
	severity Severity
	context  *Context
	cause    error
}

// Error implements the error interface
func (e *baseError) Error() string {
	if e.details != "" {
		return fmt.Sprintf("%s: %s", e.message, e.details)
	}
	return e.message
}

// Code returns the JSON-RPC error code
func (e *baseError) Code() int {
	return e.code
}

// Message returns the human-readable error message
func (e *baseError) Message() string {
	return e.message
}

// Details returns detailed technical description
func (e *baseError) Details() string {
	return e.details
}

// Data returns structured error data
func (e *baseError) Data() interface{} {
	return e.data
}

// Category returns the error category
func (e *baseError) Category() Category {
	return e.category
}

// Severity returns the error severity
func (e *baseError) Severity() Severity {
	return e.severity
}

// Context returns the error context
func (e *baseError) Context() *Context {
	return e.context
}

// WithContext returns a new error with the provided context
func (e *baseError) WithContext(ctx *Context) MCPError {
	newErr := *e
	newErr.context = ctx
	return &newErr
}

// WithDetail returns a new error with additional detail
func (e *baseError) WithDetail(detail string) MCPError {
	newErr := *e
	if newErr.details != "" {
		newErr.details = fmt.Sprintf("%s; %s", newErr.details, detail)
	} else {
		newErr.details = detail
	}
	return &newErr
}

// WithData returns a new error with structured data
func (e *baseError) WithData(data interface{}) MCPError {
	newErr := *e
	newErr.data = data
	return &newErr
}

// Unwrap returns the underlying error
func (e *baseError) Unwrap() error {
	return e.cause
}

// ToJSON returns the error as a JSON-serializable map
func (e *baseError) ToJSON() map[string]interface{} {
	result := map[string]interface{}{
		"code":     e.code,
		"message":  e.message,
		"category": string(e.category),
		"severity": string(e.severity),
	}

	if e.details != "" {
		result["details"] = e.details
	}

	if e.data != nil {
		result["data"] = e.data
	}

	if e.context != nil {
		result["context"] = e.context
	}

	if e.cause != nil {
		result["cause"] = e.cause.Error()
	}

	return result
}

// NewError creates a new MCPError with the specified parameters
func NewError(code int, message string, category Category, severity Severity) MCPError {
	return &baseError{
		code:     code,
		message:  message,
		category: category,
		severity: severity,
		context: &Context{
			Timestamp: time.Now(),
		},
	}
}

// NewErrorf creates a new MCPError with formatted message
func NewErrorf(code int, category Category, severity Severity, format string, args ...interface{}) MCPError {
	return &baseError{
		code:     code,
		message:  fmt.Sprintf(format, args...),
		category: category,
		severity: severity,
		context: &Context{
			Timestamp: time.Now(),
		},
	}
}

// WrapError wraps an existing error as an MCPError
func WrapError(err error, code int, message string, category Category, severity Severity) MCPError {
	return &baseError{
		code:     code,
		message:  message,
		category: category,
		severity: severity,
		cause:    err,
		context: &Context{
			Timestamp: time.Now(),
		},
	}
}

// WrapErrorf wraps an existing error as an MCPError with formatted message
func WrapErrorf(err error, code int, category Category, severity Severity, format string, args ...interface{}) MCPError {
	return &baseError{
		code:     code,
		message:  fmt.Sprintf(format, args...),
		category: category,
		severity: severity,
		cause:    err,
		context: &Context{
			Timestamp: time.Now(),
		},
	}
}

// AsMCPError extracts an MCPError from any error, or wraps it if it's not already an MCPError
func AsMCPError(err error) (MCPError, bool) {
	if err == nil {
		return nil, false
	}

	if mcpErr, ok := err.(MCPError); ok {
		return mcpErr, true
	}

	return nil, false
}

// IsMCPError checks if an error is an MCPError
func IsMCPError(err error) bool {
	_, ok := AsMCPError(err)
	return ok
}

// IsCategory checks if an error is of a specific category
func IsCategory(err error, category Category) bool {
	if mcpErr, ok := AsMCPError(err); ok {
		return mcpErr.Category() == category
	}
	return false
}

// IsCode checks if an error has a specific error code
func IsCode(err error, code int) bool {
	if mcpErr, ok := AsMCPError(err); ok {
		return mcpErr.Code() == code
	}
	return false
}

// MarshalJSON implements json.Marshaler for baseError
func (e *baseError) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.ToJSON())
}
