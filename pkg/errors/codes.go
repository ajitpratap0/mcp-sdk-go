package errors

// JSON-RPC 2.0 Standard Error Codes
// These map to the existing protocol error codes
const (
	// ParseError indicates invalid JSON was received by the server
	CodeParseError int = -32700

	// InvalidRequest indicates the JSON sent is not a valid Request object
	CodeInvalidRequest int = -32600

	// MethodNotFound indicates the method does not exist / is not available
	CodeMethodNotFound int = -32601

	// InvalidParams indicates invalid method parameter(s)
	CodeInvalidParams int = -32602

	// InternalError indicates internal JSON-RPC error
	CodeInternalError int = -32603
)

// MCP-Specific Error Codes
// These map to existing MCP protocol error codes and extend them
const (
	// Server Initialization Errors (-32000 to -32099)
	CodeServerInitError int = -32000 // Error during server initialization
	CodeServerNotReady  int = -32001 // Server not ready to handle requests

	// Authentication and Authorization Errors (-32100 to -32199)
	CodeUnauthorized      int = -32100 // Client is not authorized
	CodeAuthRequired      int = -32101 // Authentication required
	CodeInvalidToken      int = -32102 // Invalid authentication token
	CodeTokenExpired      int = -32103 // Authentication token expired
	CodeInsufficientPerms int = -32104 // Insufficient permissions

	// Resource Errors (-32200 to -32299)
	CodeResourceNotFound    int = -32200 // Requested resource not found
	CodeResourceUnavailable int = -32201 // Resource temporarily unavailable
	CodeResourceConflict    int = -32202 // Resource conflict (e.g., already exists)
	CodeResourceLocked      int = -32203 // Resource is locked

	// Operation Errors (-32300 to -32399)
	CodeOperationCancelled    int = -32300 // Operation was cancelled
	CodeOperationTimeout      int = -32301 // Operation timed out
	CodeOperationFailed       int = -32302 // Operation failed
	CodeOperationNotSupported int = -32303 // Operation not supported

	// Capability Errors (-32400 to -32499)
	CodeInvalidCapability  int = -32400 // Invalid or unsupported capability
	CodeCapabilityRequired int = -32401 // Required capability not enabled
	CodeCapabilityMismatch int = -32402 // Capability version mismatch

	// Transport Errors (-32500 to -32599)
	CodeTransportError    int = -32500 // Generic transport error
	CodeConnectionFailed  int = -32501 // Failed to establish connection
	CodeConnectionLost    int = -32502 // Connection lost during operation
	CodeConnectionTimeout int = -32503 // Connection timed out

	// Provider Errors (-32650 to -32699)
	CodeProviderNotConfigured int = -32650 // Provider not configured
	CodeProviderUnavailable   int = -32651 // Provider unavailable
	CodeProviderError         int = -32652 // Provider-specific error

	// Validation Errors (-32750 to -32799)
	CodeValidationError   int = -32750 // Generic validation error
	CodeMissingParameter  int = -32751 // Required parameter missing
	CodeInvalidParameter  int = -32752 // Parameter has invalid value
	CodeParameterTooLarge int = -32753 // Parameter value too large
	CodeParameterTooSmall int = -32754 // Parameter value too small
	CodeInvalidFormat     int = -32755 // Parameter has invalid format

	// Pagination Errors (-32800 to -32899)
	CodeInvalidPagination int = -32800 // Invalid pagination parameters
	CodeInvalidCursor     int = -32801 // Invalid pagination cursor
	CodeInvalidLimit      int = -32802 // Invalid pagination limit

	// Protocol Errors (-32900 to -32999)
	CodeProtocolError      int = -32900 // Generic protocol error
	CodeVersionMismatch    int = -32901 // Protocol version mismatch
	CodeInvalidSequence    int = -32902 // Invalid message sequence
	CodeUnsupportedFeature int = -32903 // Unsupported protocol feature
)

// ErrorCodeInfo provides human-readable information about error codes
type ErrorCodeInfo struct {
	Code        int
	Name        string
	Description string
	Category    Category
	Severity    Severity
}

// errorCodeRegistry maps error codes to their information
var errorCodeRegistry = map[int]ErrorCodeInfo{
	// JSON-RPC Standard Errors
	CodeParseError:     {CodeParseError, "ParseError", "Invalid JSON was received", CategoryProtocol, SeverityError},
	CodeInvalidRequest: {CodeInvalidRequest, "InvalidRequest", "Invalid Request object", CategoryProtocol, SeverityError},
	CodeMethodNotFound: {CodeMethodNotFound, "MethodNotFound", "Method does not exist", CategoryProtocol, SeverityError},
	CodeInvalidParams:  {CodeInvalidParams, "InvalidParams", "Invalid method parameters", CategoryValidation, SeverityError},
	CodeInternalError:  {CodeInternalError, "InternalError", "Internal JSON-RPC error", CategoryInternal, SeverityError},

	// Server Errors
	CodeServerInitError: {CodeServerInitError, "ServerInitError", "Server initialization failed", CategoryInternal, SeverityCritical},
	CodeServerNotReady:  {CodeServerNotReady, "ServerNotReady", "Server not ready", CategoryInternal, SeverityError},

	// Authentication Errors
	CodeUnauthorized:      {CodeUnauthorized, "Unauthorized", "Client not authorized", CategoryAuth, SeverityError},
	CodeAuthRequired:      {CodeAuthRequired, "AuthRequired", "Authentication required", CategoryAuth, SeverityError},
	CodeInvalidToken:      {CodeInvalidToken, "InvalidToken", "Invalid authentication token", CategoryAuth, SeverityError},
	CodeTokenExpired:      {CodeTokenExpired, "TokenExpired", "Authentication token expired", CategoryAuth, SeverityWarning},
	CodeInsufficientPerms: {CodeInsufficientPerms, "InsufficientPermissions", "Insufficient permissions", CategoryAuth, SeverityError},

	// Resource Errors
	CodeResourceNotFound:    {CodeResourceNotFound, "ResourceNotFound", "Resource not found", CategoryNotFound, SeverityError},
	CodeResourceUnavailable: {CodeResourceUnavailable, "ResourceUnavailable", "Resource unavailable", CategoryNotFound, SeverityWarning},
	CodeResourceConflict:    {CodeResourceConflict, "ResourceConflict", "Resource conflict", CategoryValidation, SeverityError},
	CodeResourceLocked:      {CodeResourceLocked, "ResourceLocked", "Resource locked", CategoryValidation, SeverityWarning},

	// Operation Errors
	CodeOperationCancelled:    {CodeOperationCancelled, "OperationCancelled", "Operation cancelled", CategoryCancelled, SeverityInfo},
	CodeOperationTimeout:      {CodeOperationTimeout, "OperationTimeout", "Operation timed out", CategoryTimeout, SeverityError},
	CodeOperationFailed:       {CodeOperationFailed, "OperationFailed", "Operation failed", CategoryInternal, SeverityError},
	CodeOperationNotSupported: {CodeOperationNotSupported, "OperationNotSupported", "Operation not supported", CategoryValidation, SeverityError},

	// Capability Errors
	CodeInvalidCapability:  {CodeInvalidCapability, "InvalidCapability", "Invalid capability", CategoryValidation, SeverityError},
	CodeCapabilityRequired: {CodeCapabilityRequired, "CapabilityRequired", "Required capability not enabled", CategoryValidation, SeverityError},
	CodeCapabilityMismatch: {CodeCapabilityMismatch, "CapabilityMismatch", "Capability version mismatch", CategoryValidation, SeverityError},

	// Transport Errors
	CodeTransportError:    {CodeTransportError, "TransportError", "Transport error", CategoryTransport, SeverityError},
	CodeConnectionFailed:  {CodeConnectionFailed, "ConnectionFailed", "Connection failed", CategoryTransport, SeverityCritical},
	CodeConnectionLost:    {CodeConnectionLost, "ConnectionLost", "Connection lost", CategoryTransport, SeverityError},
	CodeConnectionTimeout: {CodeConnectionTimeout, "ConnectionTimeout", "Connection timeout", CategoryTransport, SeverityError},

	// Provider Errors
	CodeProviderNotConfigured: {CodeProviderNotConfigured, "ProviderNotConfigured", "Provider not configured", CategoryProvider, SeverityError},
	CodeProviderUnavailable:   {CodeProviderUnavailable, "ProviderUnavailable", "Provider unavailable", CategoryProvider, SeverityError},
	CodeProviderError:         {CodeProviderError, "ProviderError", "Provider error", CategoryProvider, SeverityError},

	// Validation Errors
	CodeValidationError:   {CodeValidationError, "ValidationError", "Validation error", CategoryValidation, SeverityError},
	CodeMissingParameter:  {CodeMissingParameter, "MissingParameter", "Required parameter missing", CategoryValidation, SeverityError},
	CodeInvalidParameter:  {CodeInvalidParameter, "InvalidParameter", "Invalid parameter value", CategoryValidation, SeverityError},
	CodeParameterTooLarge: {CodeParameterTooLarge, "ParameterTooLarge", "Parameter value too large", CategoryValidation, SeverityError},
	CodeParameterTooSmall: {CodeParameterTooSmall, "ParameterTooSmall", "Parameter value too small", CategoryValidation, SeverityError},
	CodeInvalidFormat:     {CodeInvalidFormat, "InvalidFormat", "Invalid parameter format", CategoryValidation, SeverityError},

	// Pagination Errors
	CodeInvalidPagination: {CodeInvalidPagination, "InvalidPagination", "Invalid pagination parameters", CategoryValidation, SeverityError},
	CodeInvalidCursor:     {CodeInvalidCursor, "InvalidCursor", "Invalid pagination cursor", CategoryValidation, SeverityError},
	CodeInvalidLimit:      {CodeInvalidLimit, "InvalidLimit", "Invalid pagination limit", CategoryValidation, SeverityError},

	// Protocol Errors
	CodeProtocolError:      {CodeProtocolError, "ProtocolError", "Protocol error", CategoryProtocol, SeverityError},
	CodeVersionMismatch:    {CodeVersionMismatch, "VersionMismatch", "Protocol version mismatch", CategoryProtocol, SeverityError},
	CodeInvalidSequence:    {CodeInvalidSequence, "InvalidSequence", "Invalid message sequence", CategoryProtocol, SeverityError},
	CodeUnsupportedFeature: {CodeUnsupportedFeature, "UnsupportedFeature", "Unsupported protocol feature", CategoryProtocol, SeverityError},
}

// GetErrorCodeInfo returns information about an error code
func GetErrorCodeInfo(code int) (ErrorCodeInfo, bool) {
	info, exists := errorCodeRegistry[code]
	return info, exists
}

// GetErrorCodeName returns the name of an error code
func GetErrorCodeName(code int) string {
	if info, exists := errorCodeRegistry[code]; exists {
		return info.Name
	}
	return "UnknownError"
}

// GetErrorCodeDescription returns the description of an error code
func GetErrorCodeDescription(code int) string {
	if info, exists := errorCodeRegistry[code]; exists {
		return info.Description
	}
	return "Unknown error"
}

// GetErrorCodeCategory returns the category of an error code
func GetErrorCodeCategory(code int) Category {
	if info, exists := errorCodeRegistry[code]; exists {
		return info.Category
	}
	return CategoryInternal
}

// GetErrorCodeSeverity returns the severity of an error code
func GetErrorCodeSeverity(code int) Severity {
	if info, exists := errorCodeRegistry[code]; exists {
		return info.Severity
	}
	return SeverityError
}

// ListErrorCodes returns all registered error codes
func ListErrorCodes() []ErrorCodeInfo {
	codes := make([]ErrorCodeInfo, 0, len(errorCodeRegistry))
	for _, info := range errorCodeRegistry {
		codes = append(codes, info)
	}
	return codes
}

// IsStandardJSONRPCCode checks if a code is a standard JSON-RPC error code
func IsStandardJSONRPCCode(code int) bool {
	return code >= -32768 && code <= -32000
}

// IsMCPSpecificCode checks if a code is MCP-specific
func IsMCPSpecificCode(code int) bool {
	return code >= -32999 && code <= -32000
}
