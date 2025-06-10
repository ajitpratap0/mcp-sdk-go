package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

// MockLogger for testing
type MockLogger struct {
	mu       sync.Mutex
	messages []string
}

func (m *MockLogger) Debug(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *MockLogger) Info(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *MockLogger) Warn(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

func (m *MockLogger) Error(msg string, args ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
}

// TestResourceSubscriptionManager tests the subscription manager
func TestResourceSubscriptionManager(t *testing.T) {
	server := createTestServer()
	logger := &MockLogger{}
	manager := NewResourceSubscriptionManager(server, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)
	defer manager.Stop()

	t.Run("Subscribe", func(t *testing.T) {
		err := manager.Subscribe("/test/resource", false)
		assert.NoError(t, err)

		subs := manager.GetSubscriptions()
		assert.Len(t, subs, 1)
		assert.Contains(t, subs, "/test/resource")
		assert.False(t, subs["/test/resource"].Recursive)
	})

	t.Run("Subscribe Recursive", func(t *testing.T) {
		err := manager.Subscribe("/test/recursive", true)
		assert.NoError(t, err)

		subs := manager.GetSubscriptions()
		assert.Contains(t, subs, "/test/recursive")
		assert.True(t, subs["/test/recursive"].Recursive)
	})

	t.Run("Subscribe Duplicate", func(t *testing.T) {
		// Subscribe to same URI again
		err := manager.Subscribe("/test/resource", true)
		assert.NoError(t, err)

		// Should still have same number of subscriptions
		subs := manager.GetSubscriptions()
		assert.Len(t, subs, 2) // From previous tests
	})

	t.Run("Unsubscribe", func(t *testing.T) {
		err := manager.Unsubscribe("/test/resource")
		assert.NoError(t, err)

		subs := manager.GetSubscriptions()
		assert.NotContains(t, subs, "/test/resource")
	})

	t.Run("Unsubscribe NonExistent", func(t *testing.T) {
		err := manager.Unsubscribe("/nonexistent")
		assert.Error(t, err)
	})

	t.Run("IsSubscribed", func(t *testing.T) {
		// Direct subscription
		assert.True(t, manager.IsSubscribed("/test/recursive"))

		// Child resource of recursive subscription
		assert.True(t, manager.IsSubscribed("/test/recursive/child"))

		// Non-subscribed resource
		assert.False(t, manager.IsSubscribed("/other/resource"))
	})
}

// TestResourceChangeNotifications tests change notifications
func TestResourceChangeNotifications(t *testing.T) {
	mockServer := createTestServerWithNotifications()
	logger := &MockLogger{}
	manager := NewResourceSubscriptionManager(mockServer, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)
	defer manager.Stop()

	// Subscribe to a resource
	err := manager.Subscribe("/test", true)
	require.NoError(t, err)

	// Track notifications
	notifications := &mockServer.notifications

	t.Run("ResourceAdded", func(t *testing.T) {
		resource := &protocol.Resource{
			URI:  "/test/new-file.txt",
			Name: "new-file.txt",
			Type: "text/plain",
		}

		change := ResourceChange{
			Type:     ResourceAdded,
			Resource: resource,
			URI:      resource.URI,
		}

		manager.NotifyResourceChange(change)

		// Wait for notification processing
		time.Sleep(150 * time.Millisecond)

		// Check notification was sent
		notifications.mu.Lock()
		defer notifications.mu.Unlock()

		require.Greater(t, len(notifications.resourcesChanged), 0)
		lastNotif := notifications.resourcesChanged[len(notifications.resourcesChanged)-1]
		assert.Equal(t, "/test", lastNotif.URI)
		assert.Len(t, lastNotif.Added, 1)
		assert.Equal(t, resource.URI, lastNotif.Added[0].URI)
	})

	t.Run("ResourceModified", func(t *testing.T) {
		resource := &protocol.Resource{
			URI:  "/test/existing-file.txt",
			Name: "existing-file.txt",
			Type: "text/plain",
		}

		change := ResourceChange{
			Type:     ResourceModified,
			Resource: resource,
			URI:      resource.URI,
		}

		manager.NotifyResourceChange(change)

		// Wait for notification processing
		time.Sleep(150 * time.Millisecond)

		// Check notification was sent
		notifications.mu.Lock()
		defer notifications.mu.Unlock()

		lastNotif := notifications.resourcesChanged[len(notifications.resourcesChanged)-1]
		assert.Len(t, lastNotif.Modified, 1)
		assert.Equal(t, resource.URI, lastNotif.Modified[0].URI)
	})

	t.Run("ResourceRemoved", func(t *testing.T) {
		change := ResourceChange{
			Type: ResourceRemoved,
			URI:  "/test/deleted-file.txt",
		}

		manager.NotifyResourceChange(change)

		// Wait for notification processing
		time.Sleep(150 * time.Millisecond)

		// Check notification was sent
		notifications.mu.Lock()
		defer notifications.mu.Unlock()

		lastNotif := notifications.resourcesChanged[len(notifications.resourcesChanged)-1]
		assert.Len(t, lastNotif.Removed, 1)
		assert.Equal(t, change.URI, lastNotif.Removed[0])
	})

	t.Run("ResourceUpdated", func(t *testing.T) {
		contents := &protocol.ResourceContents{
			URI:     "/test/updated-file.txt",
			Type:    "text/plain",
			Content: json.RawMessage(`"Updated content"`),
		}

		change := ResourceChange{
			Type:     ResourceUpdated,
			Contents: contents,
			URI:      contents.URI,
		}

		manager.NotifyResourceChange(change)

		// Wait for notification processing
		time.Sleep(150 * time.Millisecond)

		// Check notification was sent
		notifications.mu.Lock()
		defer notifications.mu.Unlock()

		require.Greater(t, len(notifications.resourceUpdated), 0)
		lastUpdate := notifications.resourceUpdated[len(notifications.resourceUpdated)-1]
		assert.Equal(t, contents.URI, lastUpdate.URI)
		assert.Equal(t, contents.Content, lastUpdate.Contents.Content)
		assert.False(t, lastUpdate.Deleted)
	})
}

// TestBatchedNotifications tests notification batching
func TestBatchedNotifications(t *testing.T) {
	mockServer := createTestServerWithNotifications()
	logger := &MockLogger{}
	manager := NewResourceSubscriptionManager(mockServer, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager.Start(ctx)
	defer manager.Stop()

	// Subscribe to a resource
	err := manager.Subscribe("/test", true)
	require.NoError(t, err)

	// Send multiple changes quickly
	for i := 0; i < 5; i++ {
		resource := &protocol.Resource{
			URI:  fmt.Sprintf("/test/file%d.txt", i),
			Name: fmt.Sprintf("file%d.txt", i),
			Type: "text/plain",
		}

		change := ResourceChange{
			Type:     ResourceAdded,
			Resource: resource,
			URI:      resource.URI,
		}

		manager.NotifyResourceChange(change)
	}

	// Wait for batch processing
	time.Sleep(150 * time.Millisecond)

	// Check that changes were batched
	notifications := &mockServer.notifications
	notifications.mu.Lock()
	defer notifications.mu.Unlock()

	// Should have received one notification with 5 added resources
	require.Greater(t, len(notifications.resourcesChanged), 0)
	lastNotif := notifications.resourcesChanged[len(notifications.resourcesChanged)-1]
	assert.Len(t, lastNotif.Added, 5)
}

// TestSubscribableResourcesProvider tests the subscribable wrapper
func TestSubscribableResourcesProvider(t *testing.T) {
	baseProvider := NewBaseResourcesProvider()
	server := createTestServer()
	logger := &MockLogger{}
	manager := NewResourceSubscriptionManager(server, logger)

	provider := NewSubscribableResourcesProvider(baseProvider, manager)

	ctx := context.Background()

	t.Run("SubscribeResource", func(t *testing.T) {
		success, err := provider.SubscribeResource(ctx, "/test/resource", true)
		assert.NoError(t, err)
		assert.True(t, success)

		// Check that subscription was added to manager
		assert.True(t, manager.IsSubscribed("/test/resource"))
	})

	t.Run("UnsubscribeResource", func(t *testing.T) {
		success, err := provider.UnsubscribeResource(ctx, "/test/resource")
		assert.NoError(t, err)
		assert.True(t, success)

		// Check that subscription was removed from manager
		assert.False(t, manager.IsSubscribed("/test/resource"))
	})

	t.Run("NotifyResourceChange", func(t *testing.T) {
		change := ResourceChange{
			Type: ResourceAdded,
			URI:  "/test/new",
		}

		err := provider.NotifyResourceChange(change)
		assert.NoError(t, err)
	})
}

// TestIsChildResource tests the child resource detection
func TestIsChildResource(t *testing.T) {
	tests := []struct {
		name     string
		child    string
		parent   string
		expected bool
	}{
		{
			name:     "Direct child",
			child:    "/parent/child",
			parent:   "/parent",
			expected: true,
		},
		{
			name:     "Nested child",
			child:    "/parent/child/grandchild",
			parent:   "/parent",
			expected: true,
		},
		{
			name:     "Not a child",
			child:    "/other/path",
			parent:   "/parent",
			expected: false,
		},
		{
			name:     "Same path",
			child:    "/parent",
			parent:   "/parent",
			expected: false,
		},
		{
			name:     "Parent longer than child",
			child:    "/par",
			parent:   "/parent",
			expected: false,
		},
		{
			name:     "Similar prefix but not child",
			child:    "/parenthood",
			parent:   "/parent",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isChildResource(tt.child, tt.parent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// MockNotificationServer for testing notifications
type MockNotificationServer struct {
	*Server
	notifications struct {
		mu               sync.Mutex
		resourcesChanged []protocol.ResourcesChangedParams
		resourceUpdated  []protocol.ResourceUpdatedParams
	}
}

func (s *MockNotificationServer) NotifyResourcesChanged(uri string, resources []protocol.Resource, removed []string, added []protocol.Resource, modified []protocol.Resource) error {
	s.notifications.mu.Lock()
	defer s.notifications.mu.Unlock()

	params := protocol.ResourcesChangedParams{
		URI:       uri,
		Resources: resources,
		Removed:   removed,
		Added:     added,
		Modified:  modified,
	}
	s.notifications.resourcesChanged = append(s.notifications.resourcesChanged, params)
	return nil
}

func (s *MockNotificationServer) NotifyResourceUpdated(uri string, contents *protocol.ResourceContents, deleted bool) error {
	s.notifications.mu.Lock()
	defer s.notifications.mu.Unlock()

	params := protocol.ResourceUpdatedParams{
		URI:     uri,
		Deleted: deleted,
	}
	if contents != nil {
		params.Contents = *contents
	}
	s.notifications.resourceUpdated = append(s.notifications.resourceUpdated, params)
	return nil
}

func createTestServerWithNotifications() *MockNotificationServer {
	return &MockNotificationServer{
		Server: createTestServer(),
	}
}
