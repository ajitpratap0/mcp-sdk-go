package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/model-context-protocol/go-mcp/pkg/protocol"
	"github.com/model-context-protocol/go-mcp/pkg/transport"
)

// HTTPHandler implements http.Handler for MCP
type HTTPHandler struct {
	transport transport.Transport
	mu        sync.Mutex
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler() *HTTPHandler {
	return &HTTPHandler{}
}

// NewStreamableHTTPHandler creates a new streamable HTTP handler
func NewStreamableHTTPHandler() *HTTPHandler {
	return &HTTPHandler{}
}

// ServeHTTP handles HTTP requests
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handlePostRequest(w, r)
	case http.MethodGet:
		h.handleGetRequest(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprintf(w, "Method not allowed")
	}
}

func (h *HTTPHandler) handlePostRequest(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, MCP-Session-ID")

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
		w.WriteHeader(http.StatusOK) // No response for notifications
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
	w.Write(jsonResp)
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

func (h *HTTPHandler) handleGetRequest(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

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

	// Handle client disconnection
	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		ticker.Stop()
	}()

	// Send an initial event
	fmt.Fprintf(w, "event: ready\ndata: {}\n\n")
	flusher.Flush()

	// Keep the connection alive
	for {
		select {
		case <-r.Context().Done():
			return
		case <-notify:
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
