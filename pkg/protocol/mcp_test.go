package protocol

import (
	"encoding/json"
	"testing"
)

func TestCapabilityTypes(t *testing.T) {
	// Test that capability types are defined correctly
	if CapabilityTools != "tools" {
		t.Errorf("Expected CapabilityTools to be 'tools', got %q", CapabilityTools)
	}

	if CapabilityResources != "resources" {
		t.Errorf("Expected CapabilityResources to be 'resources', got %q", CapabilityResources)
	}

	if CapabilityResourceSubscriptions != "resourceSubscriptions" {
		t.Errorf("Expected CapabilityResourceSubscriptions to be 'resourceSubscriptions', got %q", CapabilityResourceSubscriptions)
	}

	if CapabilityPrompts != "prompts" {
		t.Errorf("Expected CapabilityPrompts to be 'prompts', got %q", CapabilityPrompts)
	}

	if CapabilityComplete != "complete" {
		t.Errorf("Expected CapabilityComplete to be 'complete', got %q", CapabilityComplete)
	}

	if CapabilityRoots != "roots" {
		t.Errorf("Expected CapabilityRoots to be 'roots', got %q", CapabilityRoots)
	}

	if CapabilitySampling != "sampling" {
		t.Errorf("Expected CapabilitySampling to be 'sampling', got %q", CapabilitySampling)
	}

	if CapabilityLogging != "logging" {
		t.Errorf("Expected CapabilityLogging to be 'logging', got %q", CapabilityLogging)
	}

	if CapabilityPagination != "pagination" {
		t.Errorf("Expected CapabilityPagination to be 'pagination', got %q", CapabilityPagination)
	}
}

func TestInitializeParams(t *testing.T) {
	// Test InitializeParams struct and JSON serialization
	params := InitializeParams{
		ProtocolVersion: ProtocolRevision,
		Name:            "test-client",
		Version:         "1.0.0",
		Capabilities: map[string]bool{
			string(CapabilityTools):     true,
			string(CapabilityResources): true,
			string(CapabilitySampling):  true,
		},
		ClientInfo: &ClientInfo{
			Name:     "test-client",
			Version:  "1.0.0",
			Platform: "test-platform",
		},
		Trace: "verbose",
		FeatureOptions: map[string]interface{}{
			"option1": "value1",
			"option2": 42,
		},
	}

	// Verify struct fields
	if params.ProtocolVersion != ProtocolRevision {
		t.Errorf("Expected ProtocolVersion to be %q, got %q", ProtocolRevision, params.ProtocolVersion)
	}

	if params.Name != "test-client" {
		t.Errorf("Expected Name to be 'test-client', got %q", params.Name)
	}

	if params.Version != "1.0.0" {
		t.Errorf("Expected Version to be '1.0.0', got %q", params.Version)
	}

	if !params.Capabilities[string(CapabilityTools)] {
		t.Error("Expected CapabilityTools to be true")
	}

	if params.ClientInfo == nil {
		t.Fatal("Expected ClientInfo to not be nil")
	}

	if params.ClientInfo.Name != "test-client" {
		t.Errorf("Expected ClientInfo.Name to be 'test-client', got %q", params.ClientInfo.Name)
	}

	if params.ClientInfo.Platform != "test-platform" {
		t.Errorf("Expected ClientInfo.Platform to be 'test-platform', got %q", params.ClientInfo.Platform)
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal InitializeParams: %v", err)
	}

	var decoded InitializeParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal InitializeParams: %v", err)
	}

	// Verify decoded data
	if decoded.ProtocolVersion != params.ProtocolVersion {
		t.Errorf("Expected decoded ProtocolVersion to be %q, got %q", params.ProtocolVersion, decoded.ProtocolVersion)
	}

	if decoded.Name != params.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", params.Name, decoded.Name)
	}

	if decoded.ClientInfo == nil {
		t.Fatal("Expected decoded ClientInfo to not be nil")
	}

	if decoded.ClientInfo.Name != params.ClientInfo.Name {
		t.Errorf("Expected decoded ClientInfo.Name to be %q, got %q", params.ClientInfo.Name, decoded.ClientInfo.Name)
	}
}

func TestInitializeResult(t *testing.T) {
	// Test InitializeResult struct and JSON serialization
	result := InitializeResult{
		ProtocolVersion: ProtocolRevision,
		Name:            "test-server",
		Version:         "1.0.0",
		Capabilities: map[string]bool{
			string(CapabilityTools):     true,
			string(CapabilityResources): true,
		},
		ServerInfo: &ServerInfo{
			Name:        "test-server",
			Version:     "1.0.0",
			Description: "Test Server",
			Homepage:    "https://example.com",
		},
		FeatureOptions: map[string]interface{}{
			"option1": "value1",
			"option2": 42,
		},
	}

	// Verify struct fields
	if result.ProtocolVersion != ProtocolRevision {
		t.Errorf("Expected ProtocolVersion to be %q, got %q", ProtocolRevision, result.ProtocolVersion)
	}

	if result.Name != "test-server" {
		t.Errorf("Expected Name to be 'test-server', got %q", result.Name)
	}

	if result.Version != "1.0.0" {
		t.Errorf("Expected Version to be '1.0.0', got %q", result.Version)
	}

	if !result.Capabilities[string(CapabilityTools)] {
		t.Error("Expected CapabilityTools to be true")
	}

	if result.ServerInfo == nil {
		t.Fatal("Expected ServerInfo to not be nil")
	}

	if result.ServerInfo.Name != "test-server" {
		t.Errorf("Expected ServerInfo.Name to be 'test-server', got %q", result.ServerInfo.Name)
	}

	// Test JSON serialization
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal InitializeResult: %v", err)
	}

	var decoded InitializeResult
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal InitializeResult: %v", err)
	}

	// Verify decoded data
	if decoded.ProtocolVersion != result.ProtocolVersion {
		t.Errorf("Expected decoded ProtocolVersion to be %q, got %q", result.ProtocolVersion, decoded.ProtocolVersion)
	}

	if decoded.Name != result.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", result.Name, decoded.Name)
	}

	if decoded.ServerInfo == nil {
		t.Fatal("Expected decoded ServerInfo to not be nil")
	}

	if decoded.ServerInfo.Name != result.ServerInfo.Name {
		t.Errorf("Expected decoded ServerInfo.Name to be %q, got %q", result.ServerInfo.Name, decoded.ServerInfo.Name)
	}
}

func TestServerInfo(t *testing.T) {
	// Test ServerInfo struct
	info := ServerInfo{
		Name:        "test-server",
		Version:     "1.0.0",
		Description: "Test Server",
		Homepage:    "https://example.com",
	}

	// Verify fields
	if info.Name != "test-server" {
		t.Errorf("Expected Name to be 'test-server', got %q", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("Expected Version to be '1.0.0', got %q", info.Version)
	}

	if info.Description != "Test Server" {
		t.Errorf("Expected Description to be 'Test Server', got %q", info.Description)
	}

	if info.Homepage != "https://example.com" {
		t.Errorf("Expected Homepage to be 'https://example.com', got %q", info.Homepage)
	}

	// Test JSON serialization
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("Failed to marshal ServerInfo: %v", err)
	}

	var decoded ServerInfo
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ServerInfo: %v", err)
	}

	// Verify decoded data
	if decoded.Name != info.Name {
		t.Errorf("Expected decoded Name to be %q, got %q", info.Name, decoded.Name)
	}

	if decoded.Version != info.Version {
		t.Errorf("Expected decoded Version to be %q, got %q", info.Version, decoded.Version)
	}
}

func TestSetCapabilityParams(t *testing.T) {
	// Test SetCapabilityParams struct
	params := SetCapabilityParams{
		Capability: string(CapabilityTools),
		Value:      true,
	}

	// Verify fields
	if params.Capability != string(CapabilityTools) {
		t.Errorf("Expected Capability to be %q, got %q", CapabilityTools, params.Capability)
	}

	if !params.Value {
		t.Error("Expected Value to be true")
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal SetCapabilityParams: %v", err)
	}

	var decoded SetCapabilityParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal SetCapabilityParams: %v", err)
	}

	// Verify decoded data
	if decoded.Capability != params.Capability {
		t.Errorf("Expected decoded Capability to be %q, got %q", params.Capability, decoded.Capability)
	}

	if decoded.Value != params.Value {
		t.Errorf("Expected decoded Value to be %v, got %v", params.Value, decoded.Value)
	}
}

func TestLogParams(t *testing.T) {
	// Test LogParams struct
	params := LogParams{
		Level:   LogLevelInfo,
		Message: "Test message",
		Source:  "test-source",
		Data:    json.RawMessage(`{"key": "value"}`),
	}

	// Verify fields
	if params.Level != LogLevelInfo {
		t.Errorf("Expected Level to be %q, got %q", LogLevelInfo, params.Level)
	}

	if params.Message != "Test message" {
		t.Errorf("Expected Message to be 'Test message', got %q", params.Message)
	}

	if params.Source != "test-source" {
		t.Errorf("Expected Source to be 'test-source', got %q", params.Source)
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal LogParams: %v", err)
	}

	var decoded LogParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal LogParams: %v", err)
	}

	// Verify decoded data
	if decoded.Level != params.Level {
		t.Errorf("Expected decoded Level to be %q, got %q", params.Level, decoded.Level)
	}

	if decoded.Message != params.Message {
		t.Errorf("Expected decoded Message to be %q, got %q", params.Message, decoded.Message)
	}
}

func TestProgressParams(t *testing.T) {
	// Test ProgressParams struct
	params := ProgressParams{
		ID:        "task-1",
		Message:   "Processing...",
		Percent:   50.0,
		Completed: false,
	}

	// Verify fields
	if params.ID != "task-1" {
		t.Errorf("Expected ID to be 'task-1', got %v", params.ID)
	}

	if params.Message != "Processing..." {
		t.Errorf("Expected Message to be 'Processing...', got %q", params.Message)
	}

	if params.Percent != 50.0 {
		t.Errorf("Expected Percent to be 50.0, got %f", params.Percent)
	}

	if params.Completed {
		t.Error("Expected Completed to be false")
	}

	// Test JSON serialization
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Failed to marshal ProgressParams: %v", err)
	}

	var decoded ProgressParams
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProgressParams: %v", err)
	}

	// Verify decoded data
	if decoded.ID != params.ID {
		t.Errorf("Expected decoded ID to be %v, got %v", params.ID, decoded.ID)
	}

	if decoded.Message != params.Message {
		t.Errorf("Expected decoded Message to be %q, got %q", params.Message, decoded.Message)
	}

	if decoded.Percent != params.Percent {
		t.Errorf("Expected decoded Percent to be %f, got %f", params.Percent, decoded.Percent)
	}

	if decoded.Completed != params.Completed {
		t.Errorf("Expected decoded Completed to be %v, got %v", params.Completed, decoded.Completed)
	}
}
