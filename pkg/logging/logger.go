// Package logging provides structured logging functionality for the MCP SDK.
// It supports multiple output formats, log levels, and integration with error contexts.
package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	mcperrors "github.com/ajitpratap0/mcp-sdk-go/pkg/errors"
)

// Level represents the severity of a log message
type Level int

const (
	// DebugLevel is for detailed information useful for debugging
	DebugLevel Level = iota - 1
	// InfoLevel is for general informational messages
	InfoLevel
	// WarnLevel is for warning messages
	WarnLevel
	// ErrorLevel is for error messages
	ErrorLevel
	// FatalLevel is for fatal errors that will terminate the program
	FatalLevel
)

// String returns the string representation of a log level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// ErrorField creates an error field
func ErrorField(err error) Field {
	return Field{Key: "error", Value: err}
}

// Duration creates a duration field
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Time creates a time field
func Time(key string, value time.Time) Field {
	return Field{Key: key, Value: value}
}

// Any creates a field with any value
func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

// Logger is the interface for structured logging
type Logger interface {
	// Debug logs a debug message with fields
	Debug(msg string, fields ...Field)
	// Info logs an info message with fields
	Info(msg string, fields ...Field)
	// Warn logs a warning message with fields
	Warn(msg string, fields ...Field)
	// Error logs an error message with fields
	Error(msg string, fields ...Field)
	// Fatal logs a fatal message with fields and exits
	Fatal(msg string, fields ...Field)

	// WithFields returns a new logger with additional fields
	WithFields(fields ...Field) Logger
	// WithContext returns a new logger with context fields
	WithContext(ctx context.Context) Logger
	// WithError returns a new logger with error context
	WithError(err error) Logger

	// SetLevel sets the minimum log level
	SetLevel(level Level)
	// GetLevel returns the current log level
	GetLevel() Level
}

// Entry represents a log entry
type Entry struct {
	Level     Level
	Message   string
	Fields    map[string]interface{}
	Timestamp time.Time
	RequestID string
	Component string
	Operation string
}

// Formatter formats log entries
type Formatter interface {
	Format(entry *Entry) ([]byte, error)
}

// baseLogger is the base implementation of Logger
type baseLogger struct {
	mu         sync.RWMutex
	level      Level
	output     io.Writer
	formatter  Formatter
	fields     map[string]interface{}
	requestKey string
}

// New creates a new structured logger
func New(output io.Writer, formatter Formatter) Logger {
	if output == nil {
		output = os.Stdout
	}
	if formatter == nil {
		formatter = NewTextFormatter()
	}

	return &baseLogger{
		level:      InfoLevel,
		output:     output,
		formatter:  formatter,
		fields:     make(map[string]interface{}),
		requestKey: "request_id",
	}
}

// Debug logs a debug message
func (l *baseLogger) Debug(msg string, fields ...Field) {
	l.log(DebugLevel, msg, fields...)
}

// Info logs an info message
func (l *baseLogger) Info(msg string, fields ...Field) {
	l.log(InfoLevel, msg, fields...)
}

// Warn logs a warning message
func (l *baseLogger) Warn(msg string, fields ...Field) {
	l.log(WarnLevel, msg, fields...)
}

// Error logs an error message
func (l *baseLogger) Error(msg string, fields ...Field) {
	l.log(ErrorLevel, msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *baseLogger) Fatal(msg string, fields ...Field) {
	l.log(FatalLevel, msg, fields...)
	os.Exit(1)
}

// WithFields returns a new logger with additional fields
func (l *baseLogger) WithFields(fields ...Field) Logger {
	l.mu.RLock()
	defer l.mu.RUnlock()

	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}

	for _, field := range fields {
		newFields[field.Key] = field.Value
	}

	return &baseLogger{
		level:      l.level,
		output:     l.output,
		formatter:  l.formatter,
		fields:     newFields,
		requestKey: l.requestKey,
	}
}

// WithContext returns a new logger with context fields
func (l *baseLogger) WithContext(ctx context.Context) Logger {
	fields := []Field{}

	// Extract request ID from context
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		fields = append(fields, String(l.requestKey, requestID))
	}

	return l.WithFields(fields...)
}

// WithError returns a new logger with error context
func (l *baseLogger) WithError(err error) Logger {
	fields := []Field{ErrorField(err)}

	// If it's an MCP error, extract context
	if mcpErr, ok := err.(mcperrors.MCPError); ok {
		if ctx := mcpErr.Context(); ctx != nil {
			fields = append(fields,
				String("error_code", fmt.Sprintf("%d", mcpErr.Code())),
				String("error_category", string(mcpErr.Category())),
				String("error_severity", string(mcpErr.Severity())),
			)

			if ctx.RequestID != "" {
				fields = append(fields, String(l.requestKey, ctx.RequestID))
			}
			if ctx.Component != "" {
				fields = append(fields, String("component", ctx.Component))
			}
			if ctx.Operation != "" {
				fields = append(fields, String("operation", ctx.Operation))
			}
		}
	}

	return l.WithFields(fields...)
}

// SetLevel sets the minimum log level
func (l *baseLogger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current log level
func (l *baseLogger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// log writes a log entry
func (l *baseLogger) log(level Level, msg string, fields ...Field) {
	l.mu.RLock()
	if level < l.level {
		l.mu.RUnlock()
		return
	}
	l.mu.RUnlock()

	entry := &Entry{
		Level:     level,
		Message:   msg,
		Fields:    make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	// Copy base fields
	l.mu.RLock()
	for k, v := range l.fields {
		entry.Fields[k] = v
	}
	l.mu.RUnlock()

	// Add new fields
	for _, field := range fields {
		entry.Fields[field.Key] = field.Value
	}

	// Extract special fields
	if requestID, ok := entry.Fields[l.requestKey].(string); ok {
		entry.RequestID = requestID
	}
	if component, ok := entry.Fields["component"].(string); ok {
		entry.Component = component
	}
	if operation, ok := entry.Fields["operation"].(string); ok {
		entry.Operation = operation
	}

	// Format and write
	data, err := l.formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to format log entry: %v\n", err)
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if _, err := l.output.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write log entry: %v\n", err)
	}
}

// RequestIDKey is the context key for request IDs
type contextKey string

const requestIDKey contextKey = "request_id"

// ContextWithRequestID returns a context with a request ID
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// RequestIDFromContext extracts the request ID from a context
func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		return requestID
	}
	return ""
}
