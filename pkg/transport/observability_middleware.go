package transport

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// ObservabilityMiddleware adds metrics, logging, and tracing capabilities
type ObservabilityMiddleware struct {
	config  ObservabilityConfig
	metrics *transportMetrics
	logger  *log.Logger
}

// NewObservabilityMiddleware creates a new observability middleware
func NewObservabilityMiddleware(config ObservabilityConfig) Middleware {
	om := &ObservabilityMiddleware{
		config:  config,
		metrics: newTransportMetrics(config.MetricsPrefix),
		logger:  log.New(log.Writer(), "[ObservabilityMiddleware] ", log.LstdFlags),
	}

	if config.LogLevel == "debug" {
		om.logger.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	return om
}

// Wrap implements the Middleware interface
func (om *ObservabilityMiddleware) Wrap(transport Transport) Transport {
	return &observabilityTransport{
		middlewareTransport: middlewareTransport{next: transport},
		middleware:          om,
	}
}

// observabilityTransport wraps a transport with observability features
type observabilityTransport struct {
	middlewareTransport
	middleware *ObservabilityMiddleware
}

// SendRequest wraps the underlying SendRequest with observability
func (ot *observabilityTransport) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	start := time.Now()

	// Log request start
	if ot.middleware.config.EnableLogging {
		ot.middleware.logger.Printf("Sending request: method=%s", method)
	}

	// Record metrics
	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.incRequestsTotal(method)
	}

	// Execute request
	result, err := ot.middlewareTransport.SendRequest(ctx, method, params)

	// Record duration
	duration := time.Since(start)

	// Log and record metrics
	if err != nil {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Request failed: method=%s duration=%v error=%v", method, duration, err)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incRequestsErrors(method)
		}
	} else {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Request succeeded: method=%s duration=%v", method, duration)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incRequestsSuccess(method)
		}
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.observeRequestDuration(method, duration)
	}

	return result, err
}

// SendNotification wraps the underlying SendNotification with observability
func (ot *observabilityTransport) SendNotification(ctx context.Context, method string, params interface{}) error {
	start := time.Now()

	if ot.middleware.config.EnableLogging {
		ot.middleware.logger.Printf("Sending notification: method=%s", method)
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.incNotificationsTotal(method)
	}

	err := ot.middlewareTransport.SendNotification(ctx, method, params)

	duration := time.Since(start)

	if err != nil {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Notification failed: method=%s duration=%v error=%v", method, duration, err)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incNotificationsErrors(method)
		}
	} else {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Notification succeeded: method=%s duration=%v", method, duration)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incNotificationsSuccess(method)
		}
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.observeNotificationDuration(method, duration)
	}

	return err
}

// Initialize wraps the underlying Initialize with observability
func (ot *observabilityTransport) Initialize(ctx context.Context) error {
	start := time.Now()

	if ot.middleware.config.EnableLogging {
		ot.middleware.logger.Printf("Initializing transport")
	}

	err := ot.middlewareTransport.Initialize(ctx)

	duration := time.Since(start)

	if err != nil {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Transport initialization failed: duration=%v error=%v", duration, err)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incInitializationErrors()
		}
	} else {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Transport initialized successfully: duration=%v", duration)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incInitializationSuccess()
		}
	}

	return err
}

// Start wraps the underlying Start with observability
func (ot *observabilityTransport) Start(ctx context.Context) error {
	if ot.middleware.config.EnableLogging {
		ot.middleware.logger.Printf("Starting transport")
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.setTransportState("running")
	}

	err := ot.middlewareTransport.Start(ctx)

	if ot.middleware.config.EnableLogging {
		if err != nil {
			ot.middleware.logger.Printf("Transport start failed: error=%v", err)
		} else {
			ot.middleware.logger.Printf("Transport stopped")
		}
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.setTransportState("stopped")
	}

	return err
}

// Stop wraps the underlying Stop with observability
func (ot *observabilityTransport) Stop(ctx context.Context) error {
	if ot.middleware.config.EnableLogging {
		ot.middleware.logger.Printf("Stopping transport")
	}

	err := ot.middlewareTransport.Stop(ctx)

	if ot.middleware.config.EnableLogging {
		if err != nil {
			ot.middleware.logger.Printf("Transport stop failed: error=%v", err)
		} else {
			ot.middleware.logger.Printf("Transport stopped successfully")
		}
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.setTransportState("stopped")
	}

	return err
}

// HandleRequest wraps the underlying HandleRequest with observability
func (ot *observabilityTransport) HandleRequest(ctx context.Context, request *protocol.Request) (*protocol.Response, error) {
	start := time.Now()

	if ot.middleware.config.EnableLogging {
		ot.middleware.logger.Printf("Handling incoming request: method=%s id=%v", request.Method, request.ID)
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.incIncomingRequestsTotal(request.Method)
	}

	response, err := ot.middlewareTransport.HandleRequest(ctx, request)

	duration := time.Since(start)

	if err != nil {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Incoming request failed: method=%s id=%v duration=%v error=%v",
				request.Method, request.ID, duration, err)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incIncomingRequestsErrors(request.Method)
		}
	} else {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Incoming request handled: method=%s id=%v duration=%v",
				request.Method, request.ID, duration)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incIncomingRequestsSuccess(request.Method)
		}
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.observeIncomingRequestDuration(request.Method, duration)
	}

	return response, err
}

// HandleNotification wraps the underlying HandleNotification with observability
func (ot *observabilityTransport) HandleNotification(ctx context.Context, notification *protocol.Notification) error {
	start := time.Now()

	if ot.middleware.config.EnableLogging {
		ot.middleware.logger.Printf("Handling incoming notification: method=%s", notification.Method)
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.incIncomingNotificationsTotal(notification.Method)
	}

	err := ot.middlewareTransport.HandleNotification(ctx, notification)

	duration := time.Since(start)

	if err != nil {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Incoming notification failed: method=%s duration=%v error=%v",
				notification.Method, duration, err)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incIncomingNotificationsErrors(notification.Method)
		}
	} else {
		if ot.middleware.config.EnableLogging {
			ot.middleware.logger.Printf("Incoming notification handled: method=%s duration=%v",
				notification.Method, duration)
		}
		if ot.middleware.config.EnableMetrics {
			ot.middleware.metrics.incIncomingNotificationsSuccess(notification.Method)
		}
	}

	if ot.middleware.config.EnableMetrics {
		ot.middleware.metrics.observeIncomingNotificationDuration(notification.Method, duration)
	}

	return err
}

// GetMetrics returns the current metrics snapshot
func (ot *observabilityTransport) GetMetrics() *TransportMetricsSnapshot {
	if ot.middleware.config.EnableMetrics {
		return ot.middleware.metrics.snapshot()
	}
	return nil
}

// transportMetrics holds various transport metrics
type transportMetrics struct {
	prefix string

	// Request metrics
	requestsTotal    map[string]*atomic.Int64
	requestsSuccess  map[string]*atomic.Int64
	requestsErrors   map[string]*atomic.Int64
	requestDurations map[string]*durationTracker

	// Notification metrics
	notificationsTotal    map[string]*atomic.Int64
	notificationsSuccess  map[string]*atomic.Int64
	notificationsErrors   map[string]*atomic.Int64
	notificationDurations map[string]*durationTracker

	// Incoming request metrics
	incomingRequestsTotal    map[string]*atomic.Int64
	incomingRequestsSuccess  map[string]*atomic.Int64
	incomingRequestsErrors   map[string]*atomic.Int64
	incomingRequestDurations map[string]*durationTracker

	// Incoming notification metrics
	incomingNotificationsTotal    map[string]*atomic.Int64
	incomingNotificationsSuccess  map[string]*atomic.Int64
	incomingNotificationsErrors   map[string]*atomic.Int64
	incomingNotificationDurations map[string]*durationTracker

	// Transport lifecycle metrics
	initializationSuccess *atomic.Int64
	initializationErrors  *atomic.Int64
	transportState        string

	mu sync.RWMutex
}

// newTransportMetrics creates a new metrics collection
func newTransportMetrics(prefix string) *transportMetrics {
	return &transportMetrics{
		prefix:                        prefix,
		requestsTotal:                 make(map[string]*atomic.Int64),
		requestsSuccess:               make(map[string]*atomic.Int64),
		requestsErrors:                make(map[string]*atomic.Int64),
		requestDurations:              make(map[string]*durationTracker),
		notificationsTotal:            make(map[string]*atomic.Int64),
		notificationsSuccess:          make(map[string]*atomic.Int64),
		notificationsErrors:           make(map[string]*atomic.Int64),
		notificationDurations:         make(map[string]*durationTracker),
		incomingRequestsTotal:         make(map[string]*atomic.Int64),
		incomingRequestsSuccess:       make(map[string]*atomic.Int64),
		incomingRequestsErrors:        make(map[string]*atomic.Int64),
		incomingRequestDurations:      make(map[string]*durationTracker),
		incomingNotificationsTotal:    make(map[string]*atomic.Int64),
		incomingNotificationsSuccess:  make(map[string]*atomic.Int64),
		incomingNotificationsErrors:   make(map[string]*atomic.Int64),
		incomingNotificationDurations: make(map[string]*durationTracker),
		initializationSuccess:         &atomic.Int64{},
		initializationErrors:          &atomic.Int64{},
		transportState:                "stopped",
	}
}

// Helper methods for request metrics
func (tm *transportMetrics) incRequestsTotal(method string) {
	tm.getOrCreateInt64Counter(tm.requestsTotal, method).Add(1)
}

func (tm *transportMetrics) incRequestsSuccess(method string) {
	tm.getOrCreateInt64Counter(tm.requestsSuccess, method).Add(1)
}

func (tm *transportMetrics) incRequestsErrors(method string) {
	tm.getOrCreateInt64Counter(tm.requestsErrors, method).Add(1)
}

func (tm *transportMetrics) observeRequestDuration(method string, duration time.Duration) {
	tm.getOrCreateDurationTracker(tm.requestDurations, method).observe(duration)
}

// Helper methods for notification metrics
func (tm *transportMetrics) incNotificationsTotal(method string) {
	tm.getOrCreateInt64Counter(tm.notificationsTotal, method).Add(1)
}

func (tm *transportMetrics) incNotificationsSuccess(method string) {
	tm.getOrCreateInt64Counter(tm.notificationsSuccess, method).Add(1)
}

func (tm *transportMetrics) incNotificationsErrors(method string) {
	tm.getOrCreateInt64Counter(tm.notificationsErrors, method).Add(1)
}

func (tm *transportMetrics) observeNotificationDuration(method string, duration time.Duration) {
	tm.getOrCreateDurationTracker(tm.notificationDurations, method).observe(duration)
}

// Helper methods for incoming request metrics
func (tm *transportMetrics) incIncomingRequestsTotal(method string) {
	tm.getOrCreateInt64Counter(tm.incomingRequestsTotal, method).Add(1)
}

func (tm *transportMetrics) incIncomingRequestsSuccess(method string) {
	tm.getOrCreateInt64Counter(tm.incomingRequestsSuccess, method).Add(1)
}

func (tm *transportMetrics) incIncomingRequestsErrors(method string) {
	tm.getOrCreateInt64Counter(tm.incomingRequestsErrors, method).Add(1)
}

func (tm *transportMetrics) observeIncomingRequestDuration(method string, duration time.Duration) {
	tm.getOrCreateDurationTracker(tm.incomingRequestDurations, method).observe(duration)
}

// Helper methods for incoming notification metrics
func (tm *transportMetrics) incIncomingNotificationsTotal(method string) {
	tm.getOrCreateInt64Counter(tm.incomingNotificationsTotal, method).Add(1)
}

func (tm *transportMetrics) incIncomingNotificationsSuccess(method string) {
	tm.getOrCreateInt64Counter(tm.incomingNotificationsSuccess, method).Add(1)
}

func (tm *transportMetrics) incIncomingNotificationsErrors(method string) {
	tm.getOrCreateInt64Counter(tm.incomingNotificationsErrors, method).Add(1)
}

func (tm *transportMetrics) observeIncomingNotificationDuration(method string, duration time.Duration) {
	tm.getOrCreateDurationTracker(tm.incomingNotificationDurations, method).observe(duration)
}

// Helper methods for transport lifecycle metrics
func (tm *transportMetrics) incInitializationSuccess() {
	tm.initializationSuccess.Add(1)
}

func (tm *transportMetrics) incInitializationErrors() {
	tm.initializationErrors.Add(1)
}

func (tm *transportMetrics) setTransportState(state string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.transportState = state
}

// getOrCreateInt64Counter gets or creates an atomic int64 counter for a method
func (tm *transportMetrics) getOrCreateInt64Counter(counters map[string]*atomic.Int64, method string) *atomic.Int64 {
	tm.mu.RLock()
	if counter, exists := counters[method]; exists {
		tm.mu.RUnlock()
		return counter
	}
	tm.mu.RUnlock()

	tm.mu.Lock()
	defer tm.mu.Unlock()
	// Double-check after acquiring write lock
	if counter, exists := counters[method]; exists {
		return counter
	}

	counter := &atomic.Int64{}
	counters[method] = counter
	return counter
}

// getOrCreateDurationTracker gets or creates a duration tracker for a method
func (tm *transportMetrics) getOrCreateDurationTracker(trackers map[string]*durationTracker, method string) *durationTracker {
	tm.mu.RLock()
	if tracker, exists := trackers[method]; exists {
		tm.mu.RUnlock()
		return tracker
	}
	tm.mu.RUnlock()

	tm.mu.Lock()
	defer tm.mu.Unlock()
	// Double-check after acquiring write lock
	if tracker, exists := trackers[method]; exists {
		return tracker
	}

	tracker := &durationTracker{}
	trackers[method] = tracker
	return tracker
}

// durationTracker tracks duration statistics
type durationTracker struct {
	count   atomic.Int64
	totalNs atomic.Int64
	minNs   atomic.Int64
	maxNs   atomic.Int64
	mu      sync.Mutex
}

func (dt *durationTracker) observe(duration time.Duration) {
	nanos := duration.Nanoseconds()

	dt.count.Add(1)
	dt.totalNs.Add(nanos)

	// Update min/max with proper synchronization
	dt.mu.Lock()
	if current := dt.minNs.Load(); current == 0 || nanos < current {
		dt.minNs.Store(nanos)
	}
	if current := dt.maxNs.Load(); nanos > current {
		dt.maxNs.Store(nanos)
	}
	dt.mu.Unlock()
}

func (dt *durationTracker) stats() (count int64, total, min, max, avg time.Duration) {
	c := dt.count.Load()
	if c == 0 {
		return 0, 0, 0, 0, 0
	}

	totalNs := dt.totalNs.Load()
	minNs := dt.minNs.Load()
	maxNs := dt.maxNs.Load()

	return c, time.Duration(totalNs), time.Duration(minNs), time.Duration(maxNs), time.Duration(totalNs / c)
}

// TransportMetricsSnapshot represents a point-in-time view of transport metrics
type TransportMetricsSnapshot struct {
	TransportState        string                   `json:"transport_state"`
	Requests              map[string]MethodMetrics `json:"requests"`
	Notifications         map[string]MethodMetrics `json:"notifications"`
	IncomingRequests      map[string]MethodMetrics `json:"incoming_requests"`
	IncomingNotifications map[string]MethodMetrics `json:"incoming_notifications"`
	Initialization        InitializationMetrics    `json:"initialization"`
}

// MethodMetrics represents metrics for a specific method
type MethodMetrics struct {
	Total    int64           `json:"total"`
	Success  int64           `json:"success"`
	Errors   int64           `json:"errors"`
	Duration DurationMetrics `json:"duration"`
}

// DurationMetrics represents duration statistics
type DurationMetrics struct {
	Count int64         `json:"count"`
	Total time.Duration `json:"total"`
	Min   time.Duration `json:"min"`
	Max   time.Duration `json:"max"`
	Avg   time.Duration `json:"avg"`
}

// InitializationMetrics represents initialization metrics
type InitializationMetrics struct {
	Success int64 `json:"success"`
	Errors  int64 `json:"errors"`
}

// snapshot creates a snapshot of current metrics
func (tm *transportMetrics) snapshot() *TransportMetricsSnapshot {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	snapshot := &TransportMetricsSnapshot{
		TransportState:        tm.transportState,
		Requests:              make(map[string]MethodMetrics),
		Notifications:         make(map[string]MethodMetrics),
		IncomingRequests:      make(map[string]MethodMetrics),
		IncomingNotifications: make(map[string]MethodMetrics),
		Initialization: InitializationMetrics{
			Success: tm.initializationSuccess.Load(),
			Errors:  tm.initializationErrors.Load(),
		},
	}

	// Collect request metrics
	for method := range tm.requestsTotal {
		metrics := MethodMetrics{}

		if counter, exists := tm.requestsTotal[method]; exists && counter != nil {
			metrics.Total = counter.Load()
		}
		if counter, exists := tm.requestsSuccess[method]; exists && counter != nil {
			metrics.Success = counter.Load()
		}
		if counter, exists := tm.requestsErrors[method]; exists && counter != nil {
			metrics.Errors = counter.Load()
		}

		if tracker, exists := tm.requestDurations[method]; exists {
			count, total, min, max, avg := tracker.stats()
			metrics.Duration = DurationMetrics{
				Count: count,
				Total: total,
				Min:   min,
				Max:   max,
				Avg:   avg,
			}
		}
		snapshot.Requests[method] = metrics
	}

	// Collect notification metrics
	for method := range tm.notificationsTotal {
		metrics := MethodMetrics{}

		if counter, exists := tm.notificationsTotal[method]; exists && counter != nil {
			metrics.Total = counter.Load()
		}
		if counter, exists := tm.notificationsSuccess[method]; exists && counter != nil {
			metrics.Success = counter.Load()
		}
		if counter, exists := tm.notificationsErrors[method]; exists && counter != nil {
			metrics.Errors = counter.Load()
		}

		if tracker, exists := tm.notificationDurations[method]; exists {
			count, total, min, max, avg := tracker.stats()
			metrics.Duration = DurationMetrics{
				Count: count,
				Total: total,
				Min:   min,
				Max:   max,
				Avg:   avg,
			}
		}
		snapshot.Notifications[method] = metrics
	}

	// Collect incoming request metrics
	for method := range tm.incomingRequestsTotal {
		metrics := MethodMetrics{}

		if counter, exists := tm.incomingRequestsTotal[method]; exists && counter != nil {
			metrics.Total = counter.Load()
		}
		if counter, exists := tm.incomingRequestsSuccess[method]; exists && counter != nil {
			metrics.Success = counter.Load()
		}
		if counter, exists := tm.incomingRequestsErrors[method]; exists && counter != nil {
			metrics.Errors = counter.Load()
		}

		if tracker, exists := tm.incomingRequestDurations[method]; exists {
			count, total, min, max, avg := tracker.stats()
			metrics.Duration = DurationMetrics{
				Count: count,
				Total: total,
				Min:   min,
				Max:   max,
				Avg:   avg,
			}
		}
		snapshot.IncomingRequests[method] = metrics
	}

	// Collect incoming notification metrics
	for method := range tm.incomingNotificationsTotal {
		metrics := MethodMetrics{}

		if counter, exists := tm.incomingNotificationsTotal[method]; exists && counter != nil {
			metrics.Total = counter.Load()
		}
		if counter, exists := tm.incomingNotificationsSuccess[method]; exists && counter != nil {
			metrics.Success = counter.Load()
		}
		if counter, exists := tm.incomingNotificationsErrors[method]; exists && counter != nil {
			metrics.Errors = counter.Load()
		}

		if tracker, exists := tm.incomingNotificationDurations[method]; exists {
			count, total, min, max, avg := tracker.stats()
			metrics.Duration = DurationMetrics{
				Count: count,
				Total: total,
				Min:   min,
				Max:   max,
				Avg:   avg,
			}
		}
		snapshot.IncomingNotifications[method] = metrics
	}

	return snapshot
}

// String provides a human-readable representation of the metrics
func (snapshot *TransportMetricsSnapshot) String() string {
	return fmt.Sprintf("TransportMetrics{state=%s, requests=%d methods, notifications=%d methods, incoming_requests=%d methods, incoming_notifications=%d methods}",
		snapshot.TransportState,
		len(snapshot.Requests),
		len(snapshot.Notifications),
		len(snapshot.IncomingRequests),
		len(snapshot.IncomingNotifications))
}
