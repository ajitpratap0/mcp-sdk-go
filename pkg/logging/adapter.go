package logging

import (
	"fmt"
	"strings"
)

// LegacyAdapter adapts the structured logger to the legacy Logger interface
type LegacyAdapter struct {
	logger Logger
}

// NewLegacyAdapter creates a new legacy adapter
func NewLegacyAdapter(logger Logger) *LegacyAdapter {
	return &LegacyAdapter{logger: logger}
}

// Debug logs a debug message using printf-style formatting
func (a *LegacyAdapter) Debug(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	fields := extractFieldsFromMessage(&formatted)
	a.logger.Debug(formatted, fields...)
}

// Info logs an info message using printf-style formatting
func (a *LegacyAdapter) Info(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	fields := extractFieldsFromMessage(&formatted)
	a.logger.Info(formatted, fields...)
}

// Warn logs a warning message using printf-style formatting
func (a *LegacyAdapter) Warn(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	fields := extractFieldsFromMessage(&formatted)
	a.logger.Warn(formatted, fields...)
}

// Error logs an error message using printf-style formatting
func (a *LegacyAdapter) Error(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	fields := extractFieldsFromMessage(&formatted)
	a.logger.Error(formatted, fields...)
}

// TransportAdapter adapts the structured logger to the transport Logf interface
type TransportAdapter struct {
	logger    Logger
	component string
}

// NewTransportAdapter creates a new transport adapter
func NewTransportAdapter(logger Logger, component string) *TransportAdapter {
	return &TransportAdapter{
		logger:    logger.WithFields(String("component", component)),
		component: component,
	}
}

// Logf logs a message using printf-style formatting
func (a *TransportAdapter) Logf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)

	// Determine log level based on message content
	level := InfoLevel
	if strings.Contains(msg, "ERROR:") || strings.Contains(msg, "error") {
		level = ErrorLevel
	} else if strings.Contains(msg, "WARN:") || strings.Contains(msg, "warning") {
		level = WarnLevel
	} else if strings.Contains(msg, "DEBUG:") || strings.Contains(msg, "debug") {
		level = DebugLevel
	}

	// Extract any structured data from the message
	fields := extractFieldsFromMessage(&msg)

	// Log based on level
	switch level {
	case DebugLevel:
		a.logger.Debug(msg, fields...)
	case WarnLevel:
		a.logger.Warn(msg, fields...)
	case ErrorLevel:
		a.logger.Error(msg, fields...)
	default:
		a.logger.Info(msg, fields...)
	}
}

// extractFieldsFromMessage attempts to extract structured fields from a message
// It looks for patterns like "key=value" or "key: value" and creates fields
func extractFieldsFromMessage(msg *string) []Field {
	fields := []Field{}

	// Common patterns to extract
	patterns := []struct {
		prefix string
		field  string
	}{
		{"Method=", "method"},
		{"method=", "method"},
		{"ID=", "id"},
		{"id=", "id"},
		{"error=", "error_detail"},
		{"Error=", "error_detail"},
	}

	for _, pattern := range patterns {
		if idx := strings.Index(*msg, pattern.prefix); idx >= 0 {
			// Find the value after the prefix
			start := idx + len(pattern.prefix)
			end := start

			// Find the end of the value (space, comma, or end of string)
			for end < len(*msg) && (*msg)[end] != ' ' && (*msg)[end] != ',' && (*msg)[end] != '\n' {
				end++
			}

			if end > start {
				value := (*msg)[start:end]
				fields = append(fields, String(pattern.field, value))
			}
		}
	}

	return fields
}

// GlobalLogger is the default global logger instance
var globalLogger Logger

// init initializes the global logger
func init() {
	globalLogger = New(nil, nil)
}

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger Logger) {
	globalLogger = logger
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() Logger {
	return globalLogger
}

// Debug logs a debug message to the global logger
func Debug(msg string, fields ...Field) {
	globalLogger.Debug(msg, fields...)
}

// Info logs an info message to the global logger
func Info(msg string, fields ...Field) {
	globalLogger.Info(msg, fields...)
}

// Warn logs a warning message to the global logger
func Warn(msg string, fields ...Field) {
	globalLogger.Warn(msg, fields...)
}

// LogError logs an error message to the global logger
func LogError(msg string, fields ...Field) {
	globalLogger.Error(msg, fields...)
}

// Fatal logs a fatal message to the global logger and exits
func Fatal(msg string, fields ...Field) {
	globalLogger.Fatal(msg, fields...)
}
