package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TextFormatter formats log entries as human-readable text
type TextFormatter struct {
	// TimestampFormat is the format for timestamps
	TimestampFormat string
	// DisableColors disables terminal colors
	DisableColors bool
	// DisableTimestamp disables timestamp output
	DisableTimestamp bool
	// DisableSorting disables sorting of fields
	DisableSorting bool
}

// NewTextFormatter creates a new text formatter
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
	}
}

// Format formats a log entry as text
func (f *TextFormatter) Format(entry *Entry) ([]byte, error) {
	var buf bytes.Buffer

	// Timestamp
	if !f.DisableTimestamp {
		timestamp := entry.Timestamp.Format(f.TimestampFormat)
		buf.WriteString(timestamp)
		buf.WriteByte(' ')
	}

	// Level with color
	levelText := fmt.Sprintf("[%s]", entry.Level.String())
	if !f.DisableColors {
		levelText = f.colorLevel(entry.Level, levelText)
	}
	buf.WriteString(levelText)
	buf.WriteByte(' ')

	// Request ID if present
	if entry.RequestID != "" {
		buf.WriteString(fmt.Sprintf("[%s] ", entry.RequestID))
	}

	// Component and operation
	if entry.Component != "" {
		buf.WriteString(entry.Component)
		if entry.Operation != "" {
			buf.WriteByte('/')
			buf.WriteString(entry.Operation)
		}
		buf.WriteString(": ")
	}

	// Message
	buf.WriteString(entry.Message)

	// Fields
	if len(entry.Fields) > 0 {
		buf.WriteString(" | ")
		fields := f.formatFields(entry.Fields, entry)
		buf.WriteString(fields)
	}

	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// formatFields formats fields as key=value pairs
func (f *TextFormatter) formatFields(fields map[string]interface{}, entry *Entry) string {
	// Skip special fields already handled
	skip := map[string]bool{
		"request_id": true,
	}

	// Only skip component/operation if they were displayed in the header
	if entry.Component != "" {
		skip["component"] = true
		// Only skip operation if it was displayed with component
		if entry.Operation != "" {
			skip["operation"] = true
		}
	}

	var pairs []string
	for k, v := range fields {
		if skip[k] {
			continue
		}

		var valueStr string
		switch val := v.(type) {
		case error:
			valueStr = val.Error()
		case string:
			// Quote strings if they contain spaces
			if strings.Contains(val, " ") {
				valueStr = fmt.Sprintf("%q", val)
			} else {
				valueStr = val
			}
		default:
			valueStr = fmt.Sprintf("%v", v)
		}

		pairs = append(pairs, fmt.Sprintf("%s=%s", k, valueStr))
	}

	if !f.DisableSorting {
		sort.Strings(pairs)
	}

	return strings.Join(pairs, " ")
}

// colorLevel returns the colored level string
func (f *TextFormatter) colorLevel(level Level, text string) string {
	const (
		red    = "\033[31m"
		yellow = "\033[33m"
		blue   = "\033[34m"
		gray   = "\033[90m"
		reset  = "\033[0m"
	)

	switch level {
	case DebugLevel:
		return gray + text + reset
	case InfoLevel:
		return blue + text + reset
	case WarnLevel:
		return yellow + text + reset
	case ErrorLevel, FatalLevel:
		return red + text + reset
	default:
		return text
	}
}

// JSONFormatter formats log entries as JSON
type JSONFormatter struct {
	// PrettyPrint enables pretty printing
	PrettyPrint bool
	// TimestampFormat is the format for timestamps
	TimestampFormat string
	// DisableTimestamp disables timestamp output
	DisableTimestamp bool
}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	}
}

// Format formats a log entry as JSON
func (f *JSONFormatter) Format(entry *Entry) ([]byte, error) {
	data := make(map[string]interface{})

	// Core fields
	data["level"] = entry.Level.String()
	data["message"] = entry.Message

	if !f.DisableTimestamp {
		data["timestamp"] = entry.Timestamp.Format(f.TimestampFormat)
	}

	// Add all fields
	for k, v := range entry.Fields {
		// Handle error objects specially
		if err, ok := v.(error); ok {
			data[k] = err.Error()
		} else {
			data[k] = v
		}
	}

	// Marshal to JSON
	var out []byte
	var err error

	if f.PrettyPrint {
		out, err = json.MarshalIndent(data, "", "  ")
	} else {
		out, err = json.Marshal(data)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to marshal log entry: %w", err)
	}

	// Add newline
	out = append(out, '\n')
	return out, nil
}
