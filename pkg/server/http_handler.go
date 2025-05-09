package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// HTTPHandler implements http.Handler for MCP
type HTTPHandler struct {
	transport      transport.Transport
	allowedOrigins []string
	mu             sync.Mutex
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		allowedOrigins: []string{"*"}, // Default to allow all origins, but this should be restricted in production
	}
}

// NewStreamableHTTPHandler creates a new streamable HTTP handler
func NewStreamableHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		allowedOrigins: []string{"*"}, // Default to allow all origins, but this should be restricted in production
	}
}

// ServeHTTP handles HTTP requests
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Validate Origin header for security (prevent DNS rebinding attacks)
	if !h.isOriginAllowed(r.Header.Get("Origin")) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Origin not allowed")
		return
	}

	switch r.Method {
	case http.MethodPost:
		h.handlePostRequest(w, r)
	case http.MethodGet:
		h.handleGetRequest(w, r)
	case http.MethodOptions:
		h.handleOptionsRequest(w, r)
	case http.MethodDelete:
		h.handleDeleteRequest(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Method not allowed")
	}
}

func (h *HTTPHandler) handlePostRequest(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, MCP-Session-ID, Last-Event-ID")

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error reading request body: %v", err)
		return
	}

	if h.transport == nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Transport not set")
		return
	}

	// Determine the type of JSON-RPC message
	if protocol.IsRequest(body) {
		// Handle JSON-RPC request
		var req protocol.Request
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Invalid JSON-RPC request: %v", err)
			return
		}

		h.handleRequest(w, r.Context(), &req)
	} else if protocol.IsNotification(body) {
		// Handle JSON-RPC notification
		var notif protocol.Notification
		if err := json.Unmarshal(body, &notif); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Invalid JSON-RPC notification: %v", err)
			return
		}

		h.handleNotification(r.Context(), &notif)
		w.WriteHeader(http.StatusAccepted) // 202 Accepted for notifications as per spec
	} else {
		// Invalid JSON-RPC message
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Invalid JSON-RPC message")
	}
}

// handleRequest processes a JSON-RPC request and sends the response
func (h *HTTPHandler) handleRequest(w http.ResponseWriter, ctx context.Context, req *protocol.Request) {
	var response *protocol.Response
	var err error

	if h.transport == nil {
		response, err = protocol.NewErrorResponse(req.ID, protocol.InternalError, "Transport not properly configured", nil)
	} else {
		// Get a safe reference to the transport
		h.mu.Lock()
		tmp := h.transport
		h.mu.Unlock()

		// Try to invoke the handler through the transport's implementation
		result, handlerErr := tmp.SendRequest(ctx, req.Method, req.Params)
		if handlerErr != nil {
			response, err = protocol.NewErrorResponse(req.ID, protocol.InternalError, handlerErr.Error(), nil)
		} else {
			response, err = protocol.NewResponse(req.ID, result)
		}
	}

	// Handle marshaling error
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating response: %v", err)
		return
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	jsonResp, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error marshaling response: %v", err)
		return
	}
	if _, err := w.Write(jsonResp); err != nil {
		log.Printf("Error writing response: %v", err)
	}
}

// handleNotification processes a JSON-RPC notification (no response)
func (h *HTTPHandler) handleNotification(ctx context.Context, notif *protocol.Notification) {
	h.mu.Lock()
	transport := h.transport
	h.mu.Unlock()

	if transport == nil {
		return
	}

	// Use SendNotification instead of trying to access handlers directly
	go func() {
		_ = transport.SendNotification(ctx, notif.Method, notif.Params)
	}()
}

// handleOptionsRequest handles CORS preflight requests
func (h *HTTPHandler) handleOptionsRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, MCP-Session-ID, Last-Event-ID")
	w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteRequest handles session termination requests
func (h *HTTPHandler) handleDeleteRequest(w http.ResponseWriter, r *http.Request) {
	// Check if we have a session ID
	sessionID := r.Header.Get("MCP-Session-ID")
	if sessionID == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Missing MCP-Session-ID header")
		return
	}

	// In a real implementation, you would terminate the session here
	// For now, just send a success response
	w.WriteHeader(http.StatusOK)
}

func (h *HTTPHandler) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))

	// Create a flusher if the ResponseWriter supports it
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Streaming not supported")
		return
	}

	// Keep the connection alive by sending a comment periodically
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Handle client disconnection using context
	ctx := r.Context()

	// Send an initial event
	fmt.Fprintf(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	// Keep the connection alive
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// SetTransport sets the transport for this handler
func (h *HTTPHandler) SetTransport(t transport.Transport) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.transport = t
}

// SetAllowedOrigins sets the allowed origins for CORS and security validation
func (h *HTTPHandler) SetAllowedOrigins(origins []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allowedOrigins = origins
}

// AddAllowedOrigin adds an allowed origin for CORS and security validation
func (h *HTTPHandler) AddAllowedOrigin(origin string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allowedOrigins = append(h.allowedOrigins, origin)
}

// isOriginAllowed checks if the given origin is allowed
func (h *HTTPHandler) isOriginAllowed(origin string) bool {
	if origin == "" {
		// Some clients may not send an Origin header
		// Server implementers should decide if they want to allow this
		return true
	}

	h.mu.Lock()
	origins := h.allowedOrigins
	h.mu.Unlock()

	for _, allowed := range origins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}

	return false
}
