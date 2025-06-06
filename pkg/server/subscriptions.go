package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// Subscription represents an active resource subscription
type Subscription struct {
	URI        string    // The resource URI being subscribed to
	Recursive  bool      // Whether to include child resources
	CreatedAt  time.Time // When the subscription was created
	LastUpdate time.Time // Last time a notification was sent
}

// ResourceChangeType represents the type of change to a resource
type ResourceChangeType int

const (
	ResourceAdded ResourceChangeType = iota
	ResourceModified
	ResourceRemoved
	ResourceUpdated // For content updates
)

// ResourceChange represents a change to a resource
type ResourceChange struct {
	Type     ResourceChangeType
	Resource *protocol.Resource
	Contents *protocol.ResourceContents
	URI      string // For removed resources
}

// ResourceNotifier defines the interface for sending resource notifications
type ResourceNotifier interface {
	NotifyResourcesChanged(uri string, resources []protocol.Resource, removed []string, added []protocol.Resource, modified []protocol.Resource) error
	NotifyResourceUpdated(uri string, contents *protocol.ResourceContents, deleted bool) error
}

// ResourceSubscriptionManager manages resource subscriptions and notifications
type ResourceSubscriptionManager struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription // Key is URI
	notifier      ResourceNotifier
	logger        Logger

	// Channel for resource changes
	changes chan ResourceChange
	done    chan struct{}
	wg      sync.WaitGroup
}

// NewResourceSubscriptionManager creates a new subscription manager
func NewResourceSubscriptionManager(notifier ResourceNotifier, logger Logger) *ResourceSubscriptionManager {
	return &ResourceSubscriptionManager{
		subscriptions: make(map[string]*Subscription),
		notifier:      notifier,
		logger:        logger,
		changes:       make(chan ResourceChange, 100), // Buffered channel
		done:          make(chan struct{}),
	}
}

// Start begins processing resource changes
func (m *ResourceSubscriptionManager) Start(ctx context.Context) {
	m.wg.Add(1)
	go m.processChanges(ctx)
}

// Stop gracefully shuts down the subscription manager
func (m *ResourceSubscriptionManager) Stop() {
	close(m.done)
	m.wg.Wait()
	close(m.changes)
}

// Subscribe adds a subscription for a resource URI
func (m *ResourceSubscriptionManager) Subscribe(uri string, recursive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already subscribed
	if _, exists := m.subscriptions[uri]; exists {
		m.logger.Debug("Already subscribed to URI: %s", uri)
		return nil
	}

	// Create new subscription
	sub := &Subscription{
		URI:        uri,
		Recursive:  recursive,
		CreatedAt:  time.Now(),
		LastUpdate: time.Now(),
	}

	m.subscriptions[uri] = sub
	m.logger.Info("Added subscription for URI: %s (recursive: %v)", uri, recursive)

	return nil
}

// Unsubscribe removes a subscription for a resource URI
func (m *ResourceSubscriptionManager) Unsubscribe(uri string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.subscriptions[uri]; !exists {
		return fmt.Errorf("no subscription found for URI: %s", uri)
	}

	delete(m.subscriptions, uri)
	m.logger.Info("Removed subscription for URI: %s", uri)

	return nil
}

// GetSubscriptions returns all active subscriptions
func (m *ResourceSubscriptionManager) GetSubscriptions() map[string]*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy to avoid race conditions
	subs := make(map[string]*Subscription)
	for k, v := range m.subscriptions {
		subs[k] = v
	}
	return subs
}

// IsSubscribed checks if a URI is subscribed
func (m *ResourceSubscriptionManager) IsSubscribed(uri string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check exact match
	if _, exists := m.subscriptions[uri]; exists {
		return true
	}

	// Check recursive subscriptions
	for subURI, sub := range m.subscriptions {
		if sub.Recursive && isChildResource(uri, subURI) {
			return true
		}
	}

	return false
}

// NotifyResourceChange queues a resource change notification
func (m *ResourceSubscriptionManager) NotifyResourceChange(change ResourceChange) {
	select {
	case m.changes <- change:
		m.logger.Debug("Queued resource change notification for URI: %s", change.URI)
	default:
		m.logger.Warn("Resource change channel full, dropping notification for URI: %s", change.URI)
	}
}

// processChanges processes resource change notifications
func (m *ResourceSubscriptionManager) processChanges(ctx context.Context) {
	defer m.wg.Done()

	// Batch changes for efficiency
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	batchedChanges := make(map[string][]ResourceChange)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		case change := <-m.changes:
			// Check if any subscription matches this change
			if m.shouldNotify(change.URI) {
				batchedChanges[change.URI] = append(batchedChanges[change.URI], change)
			}
		case <-ticker.C:
			// Process batched changes
			if len(batchedChanges) > 0 {
				m.sendBatchedNotifications(batchedChanges)
				batchedChanges = make(map[string][]ResourceChange)
			}
		}
	}
}

// shouldNotify checks if a resource change should trigger notifications
func (m *ResourceSubscriptionManager) shouldNotify(uri string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check exact match
	if _, exists := m.subscriptions[uri]; exists {
		return true
	}

	// Check recursive subscriptions
	for subURI, sub := range m.subscriptions {
		if sub.Recursive && (uri == subURI || isChildResource(uri, subURI)) {
			return true
		}
	}

	return false
}

// sendBatchedNotifications sends batched resource change notifications
func (m *ResourceSubscriptionManager) sendBatchedNotifications(batchedChanges map[string][]ResourceChange) {
	// Group changes by subscription URI
	notificationGroups := make(map[string]*protocol.ResourcesChangedParams)

	m.mu.RLock()
	subscriptions := make(map[string]*Subscription)
	for k, v := range m.subscriptions {
		subscriptions[k] = v
	}
	m.mu.RUnlock()

	for uri, changes := range batchedChanges {
		// Find matching subscriptions
		for subURI, sub := range subscriptions {
			if uri == subURI || (sub.Recursive && isChildResource(uri, subURI)) {
				if _, exists := notificationGroups[subURI]; !exists {
					notificationGroups[subURI] = &protocol.ResourcesChangedParams{
						URI:      subURI,
						Added:    []protocol.Resource{},
						Modified: []protocol.Resource{},
						Removed:  []string{},
					}
				}

				// Add changes to the appropriate group
				for _, change := range changes {
					switch change.Type {
					case ResourceAdded:
						if change.Resource != nil {
							notificationGroups[subURI].Added = append(notificationGroups[subURI].Added, *change.Resource)
						}
					case ResourceModified:
						if change.Resource != nil {
							notificationGroups[subURI].Modified = append(notificationGroups[subURI].Modified, *change.Resource)
						}
					case ResourceRemoved:
						notificationGroups[subURI].Removed = append(notificationGroups[subURI].Removed, change.URI)
					case ResourceUpdated:
						// For content updates, send a separate ResourceUpdated notification
						if change.Contents != nil {
							if err := m.notifier.NotifyResourceUpdated(change.URI, change.Contents, false); err != nil {
								m.logger.Error("Failed to send resource updated notification: %v", err)
							}
						}
					}
				}
			}
		}
	}

	// Send grouped notifications
	for subURI, params := range notificationGroups {
		if len(params.Added) > 0 || len(params.Modified) > 0 || len(params.Removed) > 0 {
			// Update last notification time
			m.mu.Lock()
			if sub, exists := m.subscriptions[subURI]; exists {
				sub.LastUpdate = time.Now()
			}
			m.mu.Unlock()

			// Send notification
			if err := m.notifier.NotifyResourcesChanged(params.URI, nil, params.Removed, params.Added, params.Modified); err != nil {
				m.logger.Error("Failed to send resources changed notification: %v", err)
			}
		}
	}
}

// isChildResource checks if a URI is a child of a parent URI
func isChildResource(childURI, parentURI string) bool {
	// Simple prefix check with path separator
	if len(childURI) <= len(parentURI) {
		return false
	}
	return childURI[:len(parentURI)] == parentURI && childURI[len(parentURI)] == '/'
}

// ExtendedResourcesProvider extends ResourcesProvider with subscription management
type ExtendedResourcesProvider interface {
	ResourcesProvider

	// UnsubscribeResource unsubscribes from resource changes
	UnsubscribeResource(ctx context.Context, uri string) (bool, error)

	// NotifyResourceChange notifies about a resource change
	NotifyResourceChange(change ResourceChange) error
}

// SubscribableResourcesProvider wraps a ResourcesProvider with subscription capabilities
type SubscribableResourcesProvider struct {
	ResourcesProvider
	subscriptionManager *ResourceSubscriptionManager
}

// NewSubscribableResourcesProvider creates a new provider with subscription support
func NewSubscribableResourcesProvider(provider ResourcesProvider, manager *ResourceSubscriptionManager) *SubscribableResourcesProvider {
	return &SubscribableResourcesProvider{
		ResourcesProvider:   provider,
		subscriptionManager: manager,
	}
}

// SubscribeResource subscribes to resource changes
func (p *SubscribableResourcesProvider) SubscribeResource(ctx context.Context, uri string, recursive bool) (bool, error) {
	// Call the base implementation
	success, err := p.ResourcesProvider.SubscribeResource(ctx, uri, recursive)
	if err != nil || !success {
		return success, err
	}

	// Add to subscription manager
	if err := p.subscriptionManager.Subscribe(uri, recursive); err != nil {
		return false, err
	}

	return true, nil
}

// UnsubscribeResource unsubscribes from resource changes
func (p *SubscribableResourcesProvider) UnsubscribeResource(ctx context.Context, uri string) (bool, error) {
	if err := p.subscriptionManager.Unsubscribe(uri); err != nil {
		return false, err
	}
	return true, nil
}

// NotifyResourceChange notifies about a resource change
func (p *SubscribableResourcesProvider) NotifyResourceChange(change ResourceChange) error {
	p.subscriptionManager.NotifyResourceChange(change)
	return nil
}
