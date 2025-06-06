package logging

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// HTTPMiddleware provides HTTP request logging
func HTTPMiddleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate request ID if not present
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			// Add request ID to context
			ctx := ContextWithRequestID(r.Context(), requestID)
			r = r.WithContext(ctx)

			// Create logger with request context
			reqLogger := logger.WithFields(
				String("request_id", requestID),
				String("method", r.Method),
				String("path", r.URL.Path),
				String("remote_addr", r.RemoteAddr),
			)

			// Log request start
			reqLogger.Info("HTTP request started")

			// Create response wrapper to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			// Track request duration
			start := time.Now()

			// Call next handler
			next.ServeHTTP(rw, r)

			// Log request completion
			duration := time.Since(start)
			reqLogger.WithFields(
				Int("status", rw.statusCode),
				Int("bytes", rw.bytesWritten),
				Duration("duration", duration),
			).Info("HTTP request completed")
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture response details
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Write(data []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(data)
	rw.bytesWritten += n
	return n, err
}

// ContextMiddleware adds request context to all operations
type ContextMiddleware struct {
	logger Logger
}

// NewContextMiddleware creates a new context middleware
func NewContextMiddleware(logger Logger) *ContextMiddleware {
	return &ContextMiddleware{logger: logger}
}

// WrapHandler wraps a handler function with context logging
func (m *ContextMiddleware) WrapHandler(operation string, handler func(context.Context, interface{}) (interface{}, error)) func(context.Context, interface{}) (interface{}, error) {
	return func(ctx context.Context, params interface{}) (interface{}, error) {
		// Extract request ID from context
		requestID := RequestIDFromContext(ctx)
		if requestID == "" {
			requestID = uuid.New().String()
			ctx = ContextWithRequestID(ctx, requestID)
		}

		// Create logger with context
		logger := m.logger.WithFields(
			String("request_id", requestID),
			String("operation", operation),
		)

		// Log operation start
		logger.Debug("Operation started", Any("params", params))

		// Track duration
		start := time.Now()

		// Call handler
		result, err := handler(ctx, params)

		// Log operation completion
		duration := time.Since(start)
		if err != nil {
			logger.WithError(err).WithFields(
				Duration("duration", duration),
			).Error("Operation failed")
		} else {
			logger.WithFields(
				Duration("duration", duration),
			).Debug("Operation completed")
		}

		return result, err
	}
}

// RequestIDGenerator generates unique request IDs
type RequestIDGenerator interface {
	Generate() string
}

// UUIDGenerator generates UUID request IDs
type UUIDGenerator struct{}

// Generate generates a new UUID
func (g *UUIDGenerator) Generate() string {
	return uuid.New().String()
}

// PrefixedGenerator generates prefixed request IDs
type PrefixedGenerator struct {
	Prefix    string
	Generator RequestIDGenerator
}

// Generate generates a new prefixed ID
func (g *PrefixedGenerator) Generate() string {
	base := g.Generator.Generate()
	return fmt.Sprintf("%s-%s", g.Prefix, base)
}

// RequestIDMiddleware extracts or generates request IDs
func RequestIDMiddleware(generator RequestIDGenerator) func(http.Handler) http.Handler {
	if generator == nil {
		generator = &UUIDGenerator{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for existing request ID
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = r.Header.Get("X-Correlation-ID")
			}
			if requestID == "" {
				requestID = generator.Generate()
			}

			// Add to response headers
			w.Header().Set("X-Request-ID", requestID)

			// Add to context
			ctx := ContextWithRequestID(r.Context(), requestID)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}
