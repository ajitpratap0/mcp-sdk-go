package server

import (
	"encoding/hex"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestSecureSessionIDGeneration tests the cryptographically secure session ID generation
func TestSecureSessionIDGeneration(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	// Test multiple session ID generations
	sessionIDs := make(map[string]bool)
	for i := 0; i < 100; i++ {
		sessionID, err := handler.generateSecureSessionID()
		if err != nil {
			t.Fatalf("Failed to generate secure session ID: %v", err)
		}

		// Check that session ID is not empty
		if sessionID == "" {
			t.Error("Generated session ID is empty")
		}

		// Check that session ID has proper prefix
		if !strings.HasPrefix(sessionID, "mcp_session_") {
			t.Errorf("Session ID does not have proper prefix: %s", sessionID)
		}

		// Check that session ID is the expected length
		// Should be "mcp_session_" (12 chars) + 64 hex chars = 76 total
		if len(sessionID) != 76 {
			t.Errorf("Session ID has unexpected length: expected 76, got %d", len(sessionID))
		}

		// Check that session IDs are unique
		if sessionIDs[sessionID] {
			t.Errorf("Duplicate session ID generated: %s", sessionID)
		}
		sessionIDs[sessionID] = true

		// Check that hex part is valid hexadecimal
		hexPart := strings.TrimPrefix(sessionID, "mcp_session_")
		if len(hexPart) != 64 {
			t.Errorf("Hex part has unexpected length: expected 64, got %d", len(hexPart))
		}

		// Validate hex characters
		if !isValidHex(hexPart) {
			t.Errorf("Session ID contains invalid hex characters: %s", hexPart)
		}
	}
}

// TestSessionEntropy tests that session IDs have sufficient entropy
func TestSessionEntropy(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	const numSessions = 1000
	sessionIDs := make([]string, numSessions)

	// Generate session IDs
	for i := 0; i < numSessions; i++ {
		sessionID, err := handler.generateSecureSessionID()
		if err != nil {
			t.Fatalf("Failed to generate session ID: %v", err)
		}
		sessionIDs[i] = sessionID
	}

	// Test that no two session IDs are the same
	seen := make(map[string]bool)
	for _, sessionID := range sessionIDs {
		if seen[sessionID] {
			t.Errorf("Duplicate session ID found: %s", sessionID)
		}
		seen[sessionID] = true
	}

	// Test entropy of the random part - count unique prefixes
	prefixes := make(map[string]int)
	for _, sessionID := range sessionIDs {
		hexPart := strings.TrimPrefix(sessionID, "mcp_session_")
		// Take first 8 characters as prefix for entropy test
		prefix := hexPart[:8]
		prefixes[prefix]++
	}

	// With good entropy, we should have mostly unique prefixes
	// Allow some duplicates but not too many
	maxDuplicates := numSessions / 20 // Allow 5% duplicates
	duplicates := 0
	for _, count := range prefixes {
		if count > 1 {
			duplicates++
		}
	}

	if duplicates > maxDuplicates {
		t.Errorf("Too many duplicate prefixes: %d (max allowed: %d)", duplicates, maxDuplicates)
	}
}

// TestSessionCreation tests secure session creation
func TestSessionCreation(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	session, err := handler.createSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Validate session properties
	if session.ID == "" {
		t.Error("Session ID is empty")
	}

	if !strings.HasPrefix(session.ID, "mcp_session_") {
		t.Errorf("Session ID does not have proper prefix: %s", session.ID)
	}

	if session.CreatedAt.IsZero() {
		t.Error("Session CreatedAt is zero")
	}

	if session.LastUsedAt.IsZero() {
		t.Error("Session LastUsedAt is zero")
	}

	if session.ExpiresAt.IsZero() {
		t.Error("Session ExpiresAt is zero")
	}

	// Check that session expires in the future
	if session.ExpiresAt.Before(time.Now()) {
		t.Error("Session ExpiresAt is in the past")
	}

	// Check that session is stored in handler
	if handler.GetSessionCount() != 1 {
		t.Errorf("Expected 1 session, got %d", handler.GetSessionCount())
	}

	// Check that session can be retrieved and validated
	storedSession, isValid := handler.isSessionValid(session.ID)
	if !isValid {
		t.Error("Created session is not valid")
	}

	if storedSession.ID != session.ID {
		t.Errorf("Retrieved session ID mismatch: expected %s, got %s", session.ID, storedSession.ID)
	}
}

// TestSessionValidation tests session validation logic
func TestSessionValidation(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	// Test invalid session ID
	_, isValid := handler.isSessionValid("invalid_session_id")
	if isValid {
		t.Error("Invalid session ID should not be valid")
	}

	// Create a valid session
	session, err := handler.createSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Test valid session
	_, isValid = handler.isSessionValid(session.ID)
	if !isValid {
		t.Error("Valid session should be valid")
	}

	// Test expired session
	handler.sessionMu.Lock()
	session.ExpiresAt = time.Now().Add(-1 * time.Hour) // Expired 1 hour ago
	handler.sessionMu.Unlock()

	_, isValid = handler.isSessionValid(session.ID)
	if isValid {
		t.Error("Expired session should not be valid")
	}

	// Check that expired session was automatically removed
	if handler.GetSessionCount() != 0 {
		t.Errorf("Expected 0 sessions after expiration, got %d", handler.GetSessionCount())
	}
}

// TestSessionExpiration tests session expiration and cleanup
func TestSessionExpiration(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	// Set a very short session timeout for testing
	handler.SetSessionTimeout(100 * time.Millisecond)

	// Create a session
	session, err := handler.createSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Session should be valid initially
	_, isValid := handler.isSessionValid(session.ID)
	if !isValid {
		t.Error("Session should be valid initially")
	}

	// Wait for session to expire
	time.Sleep(200 * time.Millisecond)

	// Session should now be invalid
	_, isValid = handler.isSessionValid(session.ID)
	if isValid {
		t.Error("Session should be invalid after expiration")
	}

	// Session should be automatically removed
	if handler.GetSessionCount() != 0 {
		t.Errorf("Expected 0 sessions after expiration, got %d", handler.GetSessionCount())
	}
}

// TestSessionActivityExtension tests that session activity extends expiration
func TestSessionActivityExtension(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	// Set a short session timeout
	handler.SetSessionTimeout(200 * time.Millisecond)

	// Create a session
	session, err := handler.createSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	originalExpiry := session.ExpiresAt

	// Wait a bit then update session activity
	time.Sleep(100 * time.Millisecond)
	handler.updateSessionLastUsed(session.ID)

	// Check that expiration was extended
	handler.sessionMu.RLock()
	newExpiry := handler.sessions[session.ID].ExpiresAt
	handler.sessionMu.RUnlock()

	if !newExpiry.After(originalExpiry) {
		t.Error("Session expiration should be extended after activity")
	}

	// Session should still be valid
	_, isValid := handler.isSessionValid(session.ID)
	if !isValid {
		t.Error("Session should still be valid after activity update")
	}
}

// TestSessionRotation tests session ID rotation
func TestSessionRotation(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	// Create a session
	originalSession, err := handler.createSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	originalID := originalSession.ID
	originalCreatedAt := originalSession.CreatedAt

	// Rotate the session ID
	newID, err := handler.rotateSessionID(originalID)
	if err != nil {
		t.Fatalf("Failed to rotate session ID: %v", err)
	}

	// New ID should be different
	if newID == originalID {
		t.Error("Rotated session ID should be different from original")
	}

	// New ID should follow the same format
	if !strings.HasPrefix(newID, "mcp_session_") {
		t.Errorf("Rotated session ID does not have proper prefix: %s", newID)
	}

	// Old session should no longer exist
	_, isValid := handler.isSessionValid(originalID)
	if isValid {
		t.Error("Original session ID should no longer be valid")
	}

	// New session should be valid
	newSession, isValid := handler.isSessionValid(newID)
	if !isValid {
		t.Error("Rotated session ID should be valid")
	}

	// Check rotation count
	if newSession.RotationCount != 1 {
		t.Errorf("Expected rotation count 1, got %d", newSession.RotationCount)
	}

	// Original creation time should be preserved
	if !newSession.CreatedAt.Equal(originalCreatedAt) {
		t.Error("Original creation time should be preserved during rotation")
	}

	// Only one session should exist
	if handler.GetSessionCount() != 1 {
		t.Errorf("Expected 1 session after rotation, got %d", handler.GetSessionCount())
	}
}

// TestSessionCleanup tests automatic session cleanup
func TestSessionCleanup(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	// Create multiple sessions with different expiration times
	session1, _ := handler.createSession()
	session2, _ := handler.createSession()
	session3, _ := handler.createSession()

	// Manually expire some sessions
	handler.sessionMu.Lock()
	handler.sessions[session1.ID].ExpiresAt = time.Now().Add(-1 * time.Hour)
	handler.sessions[session2.ID].ExpiresAt = time.Now().Add(-2 * time.Hour)
	// session3 remains valid
	handler.sessionMu.Unlock()

	// Should have 3 sessions initially
	if handler.GetSessionCount() != 3 {
		t.Errorf("Expected 3 sessions initially, got %d", handler.GetSessionCount())
	}

	// Run cleanup manually
	handler.cleanupExpiredSessions()

	// Should have 1 session remaining
	if handler.GetSessionCount() != 1 {
		t.Errorf("Expected 1 session after cleanup, got %d", handler.GetSessionCount())
	}

	// The remaining session should be session3
	_, isValid := handler.isSessionValid(session3.ID)
	if !isValid {
		t.Error("Session3 should still be valid after cleanup")
	}
}

// TestSessionTimeout tests session timeout configuration
func TestSessionTimeout(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	// Test default timeout
	defaultTimeout := handler.GetSessionTimeout()
	if defaultTimeout != 24*time.Hour {
		t.Errorf("Expected default timeout 24h, got %v", defaultTimeout)
	}

	// Test setting custom timeout
	customTimeout := 2 * time.Hour
	handler.SetSessionTimeout(customTimeout)

	if handler.GetSessionTimeout() != customTimeout {
		t.Errorf("Expected custom timeout %v, got %v", customTimeout, handler.GetSessionTimeout())
	}

	// Test that new sessions use the custom timeout
	session, err := handler.createSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	expectedExpiry := session.CreatedAt.Add(customTimeout)
	if !session.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("Session expiry mismatch: expected %v, got %v", expectedExpiry, session.ExpiresAt)
	}
}

// TestConcurrentSessionOperations tests thread safety of session operations
func TestConcurrentSessionOperations(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	const numGoroutines = 10
	const numOperations = 100

	// Channel to collect session IDs
	sessionChan := make(chan string, numGoroutines*numOperations)
	doneChan := make(chan bool, numGoroutines)

	// Start multiple goroutines creating sessions concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { doneChan <- true }()
			for j := 0; j < numOperations; j++ {
				session, err := handler.createSession()
				if err != nil {
					t.Errorf("Failed to create session: %v", err)
					return
				}
				sessionChan <- session.ID
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-doneChan
	}
	close(sessionChan)

	// Collect all session IDs
	sessionIDs := make(map[string]bool)
	for sessionID := range sessionChan {
		if sessionIDs[sessionID] {
			t.Errorf("Duplicate session ID found in concurrent operations: %s", sessionID)
		}
		sessionIDs[sessionID] = true
	}

	// Verify all sessions were created
	expectedCount := numGoroutines * numOperations
	if len(sessionIDs) != expectedCount {
		t.Errorf("Expected %d unique sessions, got %d", expectedCount, len(sessionIDs))
	}

	if handler.GetSessionCount() != expectedCount {
		t.Errorf("Expected %d sessions in handler, got %d", expectedCount, handler.GetSessionCount())
	}
}

// TestSessionSecurityProperties tests security properties of sessions
func TestSessionSecurityProperties(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	// Test that session IDs are unpredictable
	sessions := make([]*SessionInfo, 100)
	for i := 0; i < 100; i++ {
		session, err := handler.createSession()
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		sessions[i] = session
	}

	// Check that session IDs don't follow predictable patterns
	for i := 1; i < len(sessions); i++ {
		prev := sessions[i-1]
		curr := sessions[i]

		// Extract hex parts for comparison
		prevHex := strings.TrimPrefix(prev.ID, "mcp_session_")
		currHex := strings.TrimPrefix(curr.ID, "mcp_session_")

		// Test that consecutive sessions don't have similar IDs
		// Compare first 16 characters - they should be different
		if prevHex[:16] == currHex[:16] {
			t.Errorf("Consecutive sessions have similar ID prefixes: %s vs %s", prevHex[:16], currHex[:16])
		}
	}

	// Test that session IDs have sufficient length for security
	for _, session := range sessions {
		hexPart := strings.TrimPrefix(session.ID, "mcp_session_")
		if len(hexPart) < 64 {
			t.Errorf("Session ID hex part too short for security: %d chars (minimum 64)", len(hexPart))
		}
	}
}

// Helper function to validate hexadecimal strings
func isValidHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// Helper function to check if random bytes have sufficient entropy
func hasGoodEntropy(data []byte) bool {
	if len(data) < 32 {
		return false
	}

	// Simple entropy check - count unique bytes
	byteCount := make(map[byte]int)
	for _, b := range data {
		byteCount[b]++
	}

	// With good entropy, we should have a reasonable distribution
	// For 32 bytes, we should have at least 20 unique byte values
	return len(byteCount) >= 20
}

// TestSessionIDFormat tests the specific format requirements
func TestSessionIDFormat(t *testing.T) {
	handler := NewStreamableHTTPHandler()
	defer handler.stopSessionCleanup()

	sessionID, err := handler.generateSecureSessionID()
	if err != nil {
		t.Fatalf("Failed to generate session ID: %v", err)
	}

	// Test format: mcp_session_<64 hex chars>
	pattern := regexp.MustCompile(`^mcp_session_[a-f0-9]{64}$`)
	if !pattern.MatchString(sessionID) {
		t.Errorf("Session ID does not match expected format: %s", sessionID)
	}

	// Test that hex part decodes to exactly 32 bytes
	hexPart := strings.TrimPrefix(sessionID, "mcp_session_")
	decoded, err := hex.DecodeString(hexPart)
	if err != nil {
		t.Errorf("Session ID hex part is not valid hex: %v", err)
	}

	if len(decoded) != 32 {
		t.Errorf("Session ID should decode to 32 bytes, got %d", len(decoded))
	}

	// Test that the random bytes have good entropy
	if !hasGoodEntropy(decoded) {
		t.Error("Session ID does not have sufficient entropy")
	}
}
