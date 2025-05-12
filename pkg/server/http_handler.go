package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
	"github.com/ajitpratap0/mcp-sdk-go/pkg/transport"
)

// HTTPHandler implements http.Handler for MCP
type HTTPHandler struct {
	transport       transport.Transport
	allowedOrigins  []string
	mu              sync.Mutex
	sessions        map[string]*SessionInfo
	sessionMu       sync.RWMutex
	sseConnections  map[string]*SSEConnection
	sseConnectionMu sync.RWMutex
}

// SessionInfo tracks information about a client session
type SessionInfo struct {
	ID          string
	CreatedAt   time.Time
	LastUsedAt  time.Time
	LastEventID string
}

// SSEConnection represents an active SSE connection
type SSEConnection struct {
	flusher     http.Flusher
	writer      http.ResponseWriter
	lastEventID string
	sessionID   string
	closeCh     chan struct{}
	mu          sync.Mutex
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		allowedOrigins: []string{"*"}, // Default to allow all origins, but this should be restricted in production
	}
}

// NewStreamableHTTPHandler creates a new streamable HTTP handler with SSE support
func NewStreamableHTTPHandler() *HTTPHandler {
	return &HTTPHandler{
		allowedOrigins: []string{"*"}, // Default to allow all origins, but this should be restricted in production
		sessions:       make(map[string]*SessionInfo),
		sseConnections: make(map[string]*SSEConnection),
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

	// Check for session ID and validate if present
	sessionID := r.Header.Get("MCP-Session-ID")
	if sessionID != "" {
		h.sessionMu.RLock()
		session, exists := h.sessions[sessionID]
		h.sessionMu.RUnlock()

		if !exists {
			// Session doesn't exist or has expired, but only return error for:
			// 1. POST requests that are not initialize
			// 2. DELETE requests
			// Allow GET requests and initialize POST requests without a valid session
			if r.Method == http.MethodPost {
				// Try to determine if this is an initialize request
				body, err := io.ReadAll(r.Body)
				isInitialize := false

				if err == nil && len(body) > 0 {
					// Check if this is a JSON-RPC request
					if protocol.IsRequest(body) {
						var req protocol.Request
						if err := json.Unmarshal(body, &req); err == nil {
							isInitialize = (req.Method == protocol.MethodInitialize)
						}
					}
				}

				// Put the body back for later processing
				r.Body = io.NopCloser(bytes.NewBuffer(body))

				if !isInitialize {
					w.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(w, "Session not found or expired")
					return
				}
			} else if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "Session not found or expired")
				return
			}
		} else {
			// Update last used time
			h.sessionMu.Lock()
			session.LastUsedAt = time.Now()
			h.sessionMu.Unlock()
		}
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

// handlePostRequest handles HTTP POST requests
func (h *HTTPHandler) handlePostRequest(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers first
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

	// Get session ID from request
	sessionID := r.Header.Get("MCP-Session-ID")
	fmt.Printf("[DEBUG] Received request with session ID: %s\n", sessionID)

	// Determine the type of JSON-RPC message
	if protocol.IsRequest(body) {
		// Handle JSON-RPC request
		var req protocol.Request
		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Invalid JSON-RPC request: %v", err)
			return
		}

		fmt.Printf("[DEBUG] Processing JSON-RPC request. Method: %s, ID: %v\n", req.Method, req.ID)

		// Check if this is an initialize request, which may create a new session
		newSessionCreated := false
		if req.Method == protocol.MethodInitialize && sessionID == "" {
			// Generate a new session ID for initialize requests without a session ID
			sessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())

			// Create a new session
			h.sessionMu.Lock()
			h.sessions[sessionID] = &SessionInfo{
				ID:         sessionID,
				CreatedAt:  time.Now(),
				LastUsedAt: time.Now(),
			}
			h.sessionMu.Unlock()

			// Set the session ID in the response header
			w.Header().Set("MCP-Session-ID", sessionID)
			fmt.Printf("[DEBUG] Created new session ID: %s\n", sessionID)
			fmt.Printf("[DEBUG] Set MCP-Session-ID header: %s\n", w.Header().Get("MCP-Session-ID"))
			newSessionCreated = true
		}

		// Check if client accepts SSE
		acceptHeader := r.Header.Get("Accept")
		wantsSSE := false
		if acceptHeader != "" {
			wantsSSE = (acceptHeader == "text/event-stream" ||
				acceptHeader == "application/json, text/event-stream" ||
				acceptHeader == "text/event-stream, application/json")
		}

		// For requests that want a streaming response
		if wantsSSE {
			h.handleStreamingRequest(w, r, &req, sessionID)
		} else {
			// For normal JSON response
			// If we created a new session, we need to ensure the session ID is in the response header
			if newSessionCreated {
				fmt.Printf("[DEBUG] Using handleRequestWithSessionID with session ID: %s\n", sessionID)
				h.handleRequestWithSessionID(w, r.Context(), &req, sessionID)
			} else {
				h.handleRequest(w, r.Context(), &req)
			}
		}
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

// handleStreamingRequest handles a request that wants a streaming response
func (h *HTTPHandler) handleStreamingRequest(w http.ResponseWriter, r *http.Request, req *protocol.Request, sessionID string) {
	// Set up SSE headers (set headers FIRST, before any writes)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Ensure session ID is set in header if provided
	if sessionID != "" {
		w.Header().Set("MCP-Session-ID", sessionID)
		fmt.Printf("[DEBUG] Set session ID in streaming response headers: %s\n", sessionID)
	}

	// Debug all headers
	fmt.Printf("[DEBUG] SSE response headers: %v\n", w.Header())

	// Check if we can flush
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Streaming not supported")
		return
	}

	// Create a connection ID for this SSE connection
	connectionID := fmt.Sprintf("conn-%s-%d", sessionID, time.Now().UnixNano())

	// Create an SSE connection and register it
	closeCh := make(chan struct{})
	conn := &SSEConnection{
		flusher:     flusher,
		writer:      w,
		sessionID:   sessionID,
		lastEventID: r.Header.Get("Last-Event-ID"),
		closeCh:     closeCh,
	}

	h.sseConnectionMu.Lock()
	h.sseConnections[connectionID] = conn
	h.sseConnectionMu.Unlock()

	// Clean up the connection when done
	defer func() {
		h.sseConnectionMu.Lock()
		delete(h.sseConnections, connectionID)
		h.sseConnectionMu.Unlock()
		close(closeCh)
	}()

	// Send an initial "connected" event
	fmt.Fprintf(w, "id: %s\n", fmt.Sprintf("evt-%d", time.Now().UnixNano()))
	fmt.Fprintf(w, "event: connected\ndata: {\"connectionId\":\"%s\"}\n\n", connectionID)
	flusher.Flush()

	// Process the request asynchronously
	go func() {
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		// Set up a goroutine to check if the client has disconnected
		go func() {
			select {
			case <-r.Context().Done():
				cancel() // Cancel our context if the request context is done
			case <-closeCh:
				// Connection closed by our handler
			}
		}()

		// Try to invoke the handler through the transport's implementation
		h.mu.Lock()
		tmp := h.transport
		h.mu.Unlock()

		if tmp == nil {
			// Error response for no transport
			errorResp, _ := protocol.NewErrorResponse(req.ID, protocol.InternalError, "Transport not properly configured", nil)
			h.sendSSEEvent(conn, "response", errorResp)
			return
		}

		// Send the request to the transport
		result, handlerErr := tmp.SendRequest(ctx, req.Method, req.Params)

		// Create and send the response
		var response *protocol.Response
		if handlerErr != nil {
			response, _ = protocol.NewErrorResponse(req.ID, protocol.InternalError, handlerErr.Error(), nil)
		} else {
			response, _ = protocol.NewResponse(req.ID, result)
		}

		// Send the response as an SSE event
		h.sendSSEEvent(conn, "response", response)
	}()

	// Keep the connection alive with periodic pings
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Handle client disconnection
	for {
		select {
		case <-r.Context().Done():
			// Client disconnected
			fmt.Printf("[DEBUG] Client disconnected from SSE stream for connection %s\n", connectionID)
			return
		case <-ticker.C:
			// Send a keep-alive ping with a comment line
			fmt.Fprintf(w, ": ping %d\n\n", time.Now().Unix())
			flusher.Flush()
			fmt.Printf("[DEBUG] Sent ping for connection %s\n", connectionID)
		}
	}
}

// sendSSEEvent sends an event over an SSE connection
func (h *HTTPHandler) sendSSEEvent(conn *SSEConnection, eventType string, data interface{}) {
	select {
	case <-conn.closeCh:
		// Connection is closed, don't try to send
		fmt.Printf("[DEBUG] Cannot send event: connection closed\n")
		return
	default:
		// Connection is open, proceed
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Error marshaling SSE data: %v", err)
		return
	}

	// Generate a unique event ID
	eventID := fmt.Sprintf("evt-%d", time.Now().UnixNano())

	// Acquire a lock before writing to ensure atomic writes
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Write the event - proper SSE format with proper line endings
	fmt.Fprintf(conn.writer, "id: %s\n", eventID)
	fmt.Fprintf(conn.writer, "event: %s\n", eventType)

	// For multi-line data, need to prefix each line with "data: "
	dataStr := string(jsonData)
	if strings.Contains(dataStr, "\n") {
		lines := strings.Split(dataStr, "\n")
		for _, line := range lines {
			fmt.Fprintf(conn.writer, "data: %s\n", line)
		}
	} else {
		fmt.Fprintf(conn.writer, "data: %s\n", dataStr)
	}

	// End the event with an empty line
	fmt.Fprintf(conn.writer, "\n")

	// Flush immediately to ensure delivery
	conn.flusher.Flush()

	// Update the last event ID
	conn.lastEventID = eventID

	fmt.Printf("[DEBUG] Sent SSE event type=%s id=%s\n", eventType, eventID)
}

// handleRequest processes a JSON-RPC request and sends the response
func (h *HTTPHandler) handleRequest(w http.ResponseWriter, ctx context.Context, req *protocol.Request) {
	var response *protocol.Response
	var err error

	// IMPORTANT: Set all headers first, before any writing to the response
	w.Header().Set("Content-Type", "application/json")

	// Check if session ID is set in the header
	sessionID := w.Header().Get("MCP-Session-ID")
	fmt.Printf("[DEBUG] Session ID in response header before sending: %s\n", sessionID)

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

	// Debug all headers before writing response
	fmt.Printf("[DEBUG] Response headers before writing: %v\n", w.Header())

	// Send response
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

// handleRequestWithSessionID processes a JSON-RPC request and ensures the session ID is in the response
func (h *HTTPHandler) handleRequestWithSessionID(w http.ResponseWriter, ctx context.Context, req *protocol.Request, sessionID string) {
	var response *protocol.Response
	var err error

	// IMPORTANT: Set all headers first, before any writing to the response
	w.Header().Set("Content-Type", "application/json")

	// Ensure session ID is set in the header if provided
	if sessionID != "" {
		w.Header().Set("MCP-Session-ID", sessionID)
		fmt.Printf("[DEBUG] Added session ID to response header: %s\n", sessionID)
	}

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

	// Debug all headers before writing response
	fmt.Printf("[DEBUG] Response headers before writing: %v\n", w.Header())

	// Send response
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

	// Check if the session exists
	h.sessionMu.RLock()
	_, exists := h.sessions[sessionID]
	h.sessionMu.RUnlock()

	if !exists {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Session not found")
		return
	}

	// Remove the session
	h.sessionMu.Lock()
	delete(h.sessions, sessionID)
	h.sessionMu.Unlock()

	// Close any SSE connections for this session
	h.sseConnectionMu.Lock()
	for id, conn := range h.sseConnections {
		if conn.sessionID == sessionID {
			close(conn.closeCh)
			delete(h.sseConnections, id)
		}
	}
	h.sseConnectionMu.Unlock()

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

	// Get session ID from request
	sessionID := r.Header.Get("MCP-Session-ID")

	// Only validate the session ID if one was provided
	if sessionID != "" {
		h.sessionMu.RLock()
		_, exists := h.sessions[sessionID]
		h.sessionMu.RUnlock()

		if !exists {
			// Session doesn't exist or has expired
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, "Session not found or expired")
			return
		}
	} else if r.Header.Get("Last-Event-ID") != "" {
		// If trying to resume without a session ID
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Missing MCP-Session-ID header for resume")
		return
	}

	// Check for Last-Event-ID for resumability
	lastEventID := r.Header.Get("Last-Event-ID")

	// Create a connection ID for this SSE connection
	connectionID := fmt.Sprintf("conn-%s-%d", sessionID, time.Now().UnixNano())

	// Create an SSE connection and register it
	closeCh := make(chan struct{})
	conn := &SSEConnection{
		flusher:     flusher,
		writer:      w,
		sessionID:   sessionID,
		lastEventID: lastEventID,
		closeCh:     closeCh,
	}

	h.sseConnectionMu.Lock()
	h.sseConnections[connectionID] = conn
	h.sseConnectionMu.Unlock()

	// Clean up the connection when done
	defer func() {
		h.sseConnectionMu.Lock()
		delete(h.sseConnections, connectionID)
		h.sseConnectionMu.Unlock()
		close(closeCh)
	}()

	// Send an initial "ready" event
	fmt.Fprintf(w, "id: %s\n", fmt.Sprintf("evt-%d", time.Now().UnixNano()))
	fmt.Fprintf(w, "event: ready\ndata: {\"connectionId\":\"%s\"}\n\n", connectionID)
	flusher.Flush()

	// Keep the connection alive by sending a comment periodically
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Handle client disconnection using context
	ctx := r.Context()

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

// BroadcastNotification sends a notification to all connected SSE clients
func (h *HTTPHandler) BroadcastNotification(method string, params interface{}) {
	// Create the notification
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		log.Printf("Error marshaling notification params: %v", err)
		return
	}

	notification := &protocol.Notification{
		Method: method,
		Params: paramsJSON,
	}

	// Send to all SSE connections
	h.sseConnectionMu.RLock()
	conns := make([]*SSEConnection, 0, len(h.sseConnections))
	for _, conn := range h.sseConnections {
		conns = append(conns, conn)
	}
	h.sseConnectionMu.RUnlock()

	for _, conn := range conns {
		h.sendSSEEvent(conn, "notification", notification)
	}
}
