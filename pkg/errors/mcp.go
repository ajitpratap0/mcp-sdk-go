package errors

import (
	"fmt"
)

// ResourceErrorData contains structured data for resource-related errors
type ResourceErrorData struct {
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id,omitempty"`
	URI          string `json:"uri,omitempty"`
	Available    bool   `json:"available"`
	Reason       string `json:"reason,omitempty"`
}

// ProviderErrorData contains structured data for provider-related errors
type ProviderErrorData struct {
	ProviderType string `json:"provider_type"`
	Operation    string `json:"operation,omitempty"`
	Available    bool   `json:"available"`
	Configured   bool   `json:"configured"`
	Reason       string `json:"reason,omitempty"`
}

// CapabilityErrorData contains structured data for capability-related errors
type CapabilityErrorData struct {
	Capability string `json:"capability"`
	Required   bool   `json:"required"`
	Supported  bool   `json:"supported"`
	Version    string `json:"version,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// OperationErrorData contains structured data for operation-related errors
type OperationErrorData struct {
	Operation string `json:"operation"`
	Component string `json:"component,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Retryable bool   `json:"retryable"`
}

// AuthErrorData contains structured data for authentication errors
type AuthErrorData struct {
	Required    bool     `json:"required"`
	TokenType   string   `json:"token_type,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
	Reason      string   `json:"reason,omitempty"`
}

// Resource Errors

// ResourceNotFound creates an error for resources that cannot be found
func ResourceNotFound(resourceType, resourceID string) MCPError {
	return NewError(
		CodeResourceNotFound,
		fmt.Sprintf("%s '%s' not found", resourceType, resourceID),
		CategoryNotFound,
		SeverityError,
	).WithData(&ResourceErrorData{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Available:    false,
	})
}

// ResourceNotFoundByURI creates an error for resources not found by URI
func ResourceNotFoundByURI(uri string) MCPError {
	return NewError(
		CodeResourceNotFound,
		fmt.Sprintf("Resource at URI '%s' not found", uri),
		CategoryNotFound,
		SeverityError,
	).WithData(&ResourceErrorData{
		URI:       uri,
		Available: false,
	})
}

// ResourceUnavailable creates an error for temporarily unavailable resources
func ResourceUnavailable(resourceType, resourceID, reason string) MCPError {
	return NewError(
		CodeResourceUnavailable,
		fmt.Sprintf("%s '%s' is temporarily unavailable: %s", resourceType, resourceID, reason),
		CategoryNotFound,
		SeverityWarning,
	).WithData(&ResourceErrorData{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Available:    false,
		Reason:       reason,
	})
}

// ResourceConflict creates an error for resource conflicts (e.g., already exists)
func ResourceConflict(resourceType, resourceID, reason string) MCPError {
	return NewError(
		CodeResourceConflict,
		fmt.Sprintf("%s '%s' conflict: %s", resourceType, resourceID, reason),
		CategoryValidation,
		SeverityError,
	).WithData(&ResourceErrorData{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Available:    true,
		Reason:       reason,
	})
}

// ResourceLocked creates an error for locked resources
func ResourceLocked(resourceType, resourceID string) MCPError {
	return NewError(
		CodeResourceLocked,
		fmt.Sprintf("%s '%s' is locked and cannot be modified", resourceType, resourceID),
		CategoryValidation,
		SeverityWarning,
	).WithData(&ResourceErrorData{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Available:    true,
		Reason:       "locked",
	})
}

// Provider Errors

// ProviderNotConfigured creates an error for unconfigured providers
func ProviderNotConfigured(providerType string) MCPError {
	return NewError(
		CodeProviderNotConfigured,
		fmt.Sprintf("%s provider is not configured", providerType),
		CategoryProvider,
		SeverityError,
	).WithData(&ProviderErrorData{
		ProviderType: providerType,
		Configured:   false,
		Available:    false,
	})
}

// ProviderUnavailable creates an error for unavailable providers
func ProviderUnavailable(providerType, reason string) MCPError {
	return NewError(
		CodeProviderUnavailable,
		fmt.Sprintf("%s provider is unavailable: %s", providerType, reason),
		CategoryProvider,
		SeverityError,
	).WithData(&ProviderErrorData{
		ProviderType: providerType,
		Configured:   true,
		Available:    false,
		Reason:       reason,
	})
}

// ProviderError creates an error for provider-specific errors
func ProviderError(providerType, operation string, cause error) MCPError {
	message := fmt.Sprintf("%s provider error during %s", providerType, operation)
	if cause != nil {
		message = fmt.Sprintf("%s: %s", message, cause.Error())
	}

	return WrapError(
		cause,
		CodeProviderError,
		message,
		CategoryProvider,
		SeverityError,
	).WithData(&ProviderErrorData{
		ProviderType: providerType,
		Operation:    operation,
		Configured:   true,
		Available:    true,
		Reason:       cause.Error(),
	})
}

// Capability Errors

// InvalidCapability creates an error for invalid or unsupported capabilities
func InvalidCapability(capability string, reason string) MCPError {
	return NewError(
		CodeInvalidCapability,
		fmt.Sprintf("Invalid capability '%s': %s", capability, reason),
		CategoryValidation,
		SeverityError,
	).WithData(&CapabilityErrorData{
		Capability: capability,
		Supported:  false,
		Reason:     reason,
	})
}

// CapabilityRequired creates an error for missing required capabilities
func CapabilityRequired(capability string) MCPError {
	return NewError(
		CodeCapabilityRequired,
		fmt.Sprintf("Required capability '%s' is not enabled", capability),
		CategoryValidation,
		SeverityError,
	).WithData(&CapabilityErrorData{
		Capability: capability,
		Required:   true,
		Supported:  false,
	})
}

// CapabilityMismatch creates an error for capability version mismatches
func CapabilityMismatch(capability, expectedVersion, actualVersion string) MCPError {
	return NewError(
		CodeCapabilityMismatch,
		fmt.Sprintf("Capability '%s' version mismatch: expected %s, got %s", capability, expectedVersion, actualVersion),
		CategoryValidation,
		SeverityError,
	).WithData(&CapabilityErrorData{
		Capability: capability,
		Version:    actualVersion,
		Supported:  true,
		Reason:     fmt.Sprintf("expected %s, got %s", expectedVersion, actualVersion),
	})
}

// Operation Errors

// OperationCancelled creates an error for cancelled operations
func OperationCancelled(operation string) MCPError {
	return NewError(
		CodeOperationCancelled,
		fmt.Sprintf("Operation '%s' was cancelled", operation),
		CategoryCancelled,
		SeverityInfo,
	).WithData(&OperationErrorData{
		Operation: operation,
		Retryable: true,
		Reason:    "cancelled by user or system",
	})
}

// OperationTimeout creates an error for operations that timed out
func OperationTimeout(operation string, timeout string) MCPError {
	return NewError(
		CodeOperationTimeout,
		fmt.Sprintf("Operation '%s' timed out after %s", operation, timeout),
		CategoryTimeout,
		SeverityError,
	).WithData(&OperationErrorData{
		Operation: operation,
		Retryable: true,
		Reason:    fmt.Sprintf("timeout after %s", timeout),
	})
}

// OperationFailed creates an error for failed operations
func OperationFailed(operation string, reason string) MCPError {
	return NewError(
		CodeOperationFailed,
		fmt.Sprintf("Operation '%s' failed: %s", operation, reason),
		CategoryInternal,
		SeverityError,
	).WithData(&OperationErrorData{
		Operation: operation,
		Retryable: false,
		Reason:    reason,
	})
}

// OperationNotSupported creates an error for unsupported operations
func OperationNotSupported(operation string, component string) MCPError {
	message := fmt.Sprintf("Operation '%s' is not supported", operation)
	if component != "" {
		message = fmt.Sprintf("Operation '%s' is not supported by %s", operation, component)
	}

	return NewError(
		CodeOperationNotSupported,
		message,
		CategoryValidation,
		SeverityError,
	).WithData(&OperationErrorData{
		Operation: operation,
		Component: component,
		Retryable: false,
		Reason:    "not supported",
	})
}

// Authentication Errors

// Unauthorized creates an error for unauthorized access
func Unauthorized(reason string) MCPError {
	return NewError(
		CodeUnauthorized,
		fmt.Sprintf("Unauthorized: %s", reason),
		CategoryAuth,
		SeverityError,
	).WithData(&AuthErrorData{
		Required: true,
		Reason:   reason,
	})
}

// AuthRequired creates an error when authentication is required
func AuthRequired(tokenType string) MCPError {
	return NewError(
		CodeAuthRequired,
		"Authentication required",
		CategoryAuth,
		SeverityError,
	).WithData(&AuthErrorData{
		Required:  true,
		TokenType: tokenType,
	})
}

// InvalidToken creates an error for invalid authentication tokens
func InvalidToken(tokenType, reason string) MCPError {
	return NewError(
		CodeInvalidToken,
		fmt.Sprintf("Invalid %s token: %s", tokenType, reason),
		CategoryAuth,
		SeverityError,
	).WithData(&AuthErrorData{
		Required:  true,
		TokenType: tokenType,
		Reason:    reason,
	})
}

// TokenExpired creates an error for expired authentication tokens
func TokenExpired(tokenType string) MCPError {
	return NewError(
		CodeTokenExpired,
		fmt.Sprintf("%s token has expired", tokenType),
		CategoryAuth,
		SeverityWarning,
	).WithData(&AuthErrorData{
		Required:  true,
		TokenType: tokenType,
		Reason:    "expired",
	})
}

// InsufficientPermissions creates an error for insufficient permissions
func InsufficientPermissions(requiredPermissions []string, reason string) MCPError {
	return NewError(
		CodeInsufficientPerms,
		fmt.Sprintf("Insufficient permissions: %s", reason),
		CategoryAuth,
		SeverityError,
	).WithData(&AuthErrorData{
		Required:    true,
		Permissions: requiredPermissions,
		Reason:      reason,
	})
}

// Server Errors

// ServerInitError creates an error for server initialization failures
func ServerInitError(reason string, cause error) MCPError {
	return WrapError(
		cause,
		CodeServerInitError,
		fmt.Sprintf("Server initialization failed: %s", reason),
		CategoryInternal,
		SeverityCritical,
	)
}

// ServerNotReady creates an error when the server is not ready to handle requests
func ServerNotReady(reason string) MCPError {
	return NewError(
		CodeServerNotReady,
		fmt.Sprintf("Server not ready: %s", reason),
		CategoryInternal,
		SeverityError,
	)
}

// Protocol Errors

// ProtocolError creates a generic protocol error
func ProtocolError(reason string) MCPError {
	return NewError(
		CodeProtocolError,
		fmt.Sprintf("Protocol error: %s", reason),
		CategoryProtocol,
		SeverityError,
	)
}

// VersionMismatch creates an error for protocol version mismatches
func VersionMismatch(expected, actual string) MCPError {
	return NewError(
		CodeVersionMismatch,
		fmt.Sprintf("Protocol version mismatch: expected %s, got %s", expected, actual),
		CategoryProtocol,
		SeverityError,
	)
}

// InvalidSequence creates an error for invalid message sequences
func InvalidSequence(expected, actual string) MCPError {
	return NewError(
		CodeInvalidSequence,
		fmt.Sprintf("Invalid message sequence: expected %s, got %s", expected, actual),
		CategoryProtocol,
		SeverityError,
	)
}

// UnsupportedFeature creates an error for unsupported protocol features
func UnsupportedFeature(feature string) MCPError {
	return NewError(
		CodeUnsupportedFeature,
		fmt.Sprintf("Unsupported protocol feature: %s", feature),
		CategoryProtocol,
		SeverityError,
	)
}
