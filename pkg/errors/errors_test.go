package errors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ajitpratap0/mcp-sdk-go/pkg/protocol"
)

func TestMCPErrorInterface(t *testing.T) {
	tests := []struct {
		name     string
		err      MCPError
		wantCode int
		wantCat  Category
		wantSev  Severity
	}{
		{
			name:     "validation error",
			err:      ValidationError("test validation error"),
			wantCode: CodeValidationError,
			wantCat:  CategoryValidation,
			wantSev:  SeverityError,
		},
		{
			name:     "resource not found",
			err:      ResourceNotFound("tool", "test-tool"),
			wantCode: CodeResourceNotFound,
			wantCat:  CategoryNotFound,
			wantSev:  SeverityError,
		},
		{
			name:     "provider not configured",
			err:      ProviderNotConfigured("tools"),
			wantCode: CodeProviderNotConfigured,
			wantCat:  CategoryProvider,
			wantSev:  SeverityError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Code(); got != tt.wantCode {
				t.Errorf("Code() = %v, want %v", got, tt.wantCode)
			}
			if got := tt.err.Category(); got != tt.wantCat {
				t.Errorf("Category() = %v, want %v", got, tt.wantCat)
			}
			if got := tt.err.Severity(); got != tt.wantSev {
				t.Errorf("Severity() = %v, want %v", got, tt.wantSev)
			}

			// Test that error implements error interface
			_ = error(tt.err)

			// Test Error() method
			if msg := tt.err.Error(); msg == "" {
				t.Error("Error() returned empty string")
			}
		})
	}
}

func TestErrorContext(t *testing.T) {
	err := ValidationError("test error")

	// Test without context
	if ctx := err.Context(); ctx == nil {
		t.Error("Context() should never return nil")
	}

	// Test with context
	requestCtx := &Context{
		RequestID: "123",
		Method:    "test/method",
		SessionID: "session-456",
		Component: "test-component",
	}

	errWithCtx := err.WithContext(requestCtx)
	if got := errWithCtx.Context(); got != requestCtx {
		t.Errorf("WithContext() failed, got %v, want %v", got, requestCtx)
	}

	// Original error should be unchanged
	if err.Context().RequestID != "" {
		t.Error("Original error was modified by WithContext()")
	}
}

func TestErrorChaining(t *testing.T) {
	cause := fmt.Errorf("underlying cause")
	err := WrapError(cause, CodeInternalError, "wrapped error", CategoryInternal, SeverityError)

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestErrorData(t *testing.T) {
	validationData := &ValidationErrorData{
		Field:    "test_field",
		Value:    "invalid_value",
		Expected: "valid_value",
	}

	err := ValidationError("test error").WithData(validationData)

	if got := err.Data(); got != validationData {
		t.Errorf("Data() = %v, want %v", got, validationData)
	}
}

func TestErrorSerialization(t *testing.T) {
	err := ResourceNotFound("tool", "test-tool").
		WithContext(&Context{
			RequestID: "123",
			Method:    "test/method",
		}).
		WithDetail("Additional detail information")

	// Test ToJSON
	jsonData := err.ToJSON()
	if jsonData["code"] != CodeResourceNotFound {
		t.Errorf("ToJSON() code = %v, want %v", jsonData["code"], CodeResourceNotFound)
	}

	// Test JSON marshaling
	jsonBytes, err2 := json.Marshal(err)
	if err2 != nil {
		t.Fatalf("Failed to marshal error: %v", err2)
	}

	var unmarshaled map[string]interface{}
	if err2 := json.Unmarshal(jsonBytes, &unmarshaled); err2 != nil {
		t.Fatalf("Failed to unmarshal error: %v", err2)
	}

	if unmarshaled["code"] != float64(CodeResourceNotFound) {
		t.Errorf("Unmarshaled code = %v, want %v", unmarshaled["code"], CodeResourceNotFound)
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		err  MCPError
		data interface{}
	}{
		{
			name: "invalid parameter",
			err:  InvalidParameter("count", "not a number", "integer"),
			data: &ParameterErrorData{},
		},
		{
			name: "missing parameter",
			err:  MissingParameter("required_field"),
			data: &ParameterErrorData{},
		},
		{
			name: "invalid pagination",
			err:  InvalidPagination("limit must be positive"),
			data: &PaginationErrorData{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Category() != CategoryValidation {
				t.Errorf("Category() = %v, want %v", tt.err.Category(), CategoryValidation)
			}

			if tt.err.Data() == nil {
				t.Error("Data() should not be nil for validation errors")
			}
		})
	}
}

func TestMCPSpecificErrors(t *testing.T) {
	tests := []struct {
		name string
		err  MCPError
		code int
	}{
		{
			name: "resource not found",
			err:  ResourceNotFound("tool", "test-tool"),
			code: CodeResourceNotFound,
		},
		{
			name: "provider not configured",
			err:  ProviderNotConfigured("tools"),
			code: CodeProviderNotConfigured,
		},
		{
			name: "operation cancelled",
			err:  OperationCancelled("list_tools"),
			code: CodeOperationCancelled,
		},
		{
			name: "unauthorized",
			err:  Unauthorized("invalid token"),
			code: CodeUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code() != tt.code {
				t.Errorf("Code() = %v, want %v", tt.err.Code(), tt.code)
			}
		})
	}
}

func TestTransportErrors(t *testing.T) {
	tests := []struct {
		name string
		err  MCPError
	}{
		{
			name: "connection failed",
			err:  ConnectionFailed("http", "http://example.com", fmt.Errorf("connection refused")),
		},
		{
			name: "transport error",
			err:  TransportError("stdio", "send", fmt.Errorf("pipe broken")),
		},
		{
			name: "connection timeout",
			err:  ConnectionTimeout("http", "http://example.com", 30*time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Category() != CategoryTransport {
				t.Errorf("Category() = %v, want %v", tt.err.Category(), CategoryTransport)
			}

			if data := tt.err.Data(); data == nil {
				t.Error("Data() should not be nil for transport errors")
			}
		})
	}
}

func TestErrorConversion(t *testing.T) {
	t.Run("ToJSONRPCResponse", func(t *testing.T) {
		err := ResourceNotFound("tool", "test-tool")
		resp, convErr := ToJSONRPCResponse(err, "123")

		if convErr != nil {
			t.Fatalf("ToJSONRPCResponse() error = %v", convErr)
		}

		if resp.Error == nil {
			t.Fatal("Response should contain error")
		}

		if resp.Error.Code != CodeResourceNotFound {
			t.Errorf("Error code = %v, want %v", resp.Error.Code, CodeResourceNotFound)
		}
	})

	t.Run("FromJSONRPCError", func(t *testing.T) {
		jsonrpcErr := &protocol.Error{
			Code:    CodeInvalidParams,
			Message: "Invalid parameters",
			Data:    map[string]string{"field": "test"},
		}

		err := FromJSONRPCError(jsonrpcErr)
		if err.Code() != CodeInvalidParams {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidParams)
		}

		if err.Message() != "Invalid parameters" {
			t.Errorf("Message() = %v, want %v", err.Message(), "Invalid parameters")
		}
	})

	t.Run("ConvertStandardError", func(t *testing.T) {
		tests := []struct {
			name     string
			err      error
			wantCode int
		}{
			{
				name:     "context canceled",
				err:      context.Canceled,
				wantCode: CodeOperationCancelled,
			},
			{
				name:     "context deadline exceeded",
				err:      context.DeadlineExceeded,
				wantCode: CodeOperationTimeout,
			},
			{
				name:     "json syntax error",
				err:      &json.SyntaxError{},
				wantCode: CodeParseError,
			},
			{
				name:     "generic error",
				err:      fmt.Errorf("generic error"),
				wantCode: CodeInternalError,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				converted := ConvertStandardError(tt.err)
				if converted.Code() != tt.wantCode {
					t.Errorf("Code() = %v, want %v", converted.Code(), tt.wantCode)
				}
			})
		}
	})
}

func TestErrorRegistry(t *testing.T) {
	tests := []struct {
		code     int
		wantName string
		wantCat  Category
	}{
		{
			code:     CodeResourceNotFound,
			wantName: "ResourceNotFound",
			wantCat:  CategoryNotFound,
		},
		{
			code:     CodeInvalidParams,
			wantName: "InvalidParams",
			wantCat:  CategoryValidation,
		},
		{
			code:     CodeInternalError,
			wantName: "InternalError",
			wantCat:  CategoryInternal,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			if name := GetErrorCodeName(tt.code); name != tt.wantName {
				t.Errorf("GetErrorCodeName() = %v, want %v", name, tt.wantName)
			}

			if cat := GetErrorCodeCategory(tt.code); cat != tt.wantCat {
				t.Errorf("GetErrorCodeCategory() = %v, want %v", cat, tt.wantCat)
			}

			if info, exists := GetErrorCodeInfo(tt.code); !exists {
				t.Errorf("GetErrorCodeInfo() should exist for code %d", tt.code)
			} else if info.Name != tt.wantName {
				t.Errorf("ErrorCodeInfo.Name = %v, want %v", info.Name, tt.wantName)
			}
		})
	}
}

func TestErrorHelpers(t *testing.T) {
	t.Run("AsMCPError", func(t *testing.T) {
		mcpErr := ValidationError("test error")

		// Test with MCPError
		if extracted, ok := AsMCPError(mcpErr); !ok || extracted != mcpErr {
			t.Error("AsMCPError() failed for MCPError")
		}

		// Test with regular error
		regularErr := fmt.Errorf("regular error")
		if _, ok := AsMCPError(regularErr); ok {
			t.Error("AsMCPError() should return false for regular errors")
		}

		// Test with nil
		if _, ok := AsMCPError(nil); ok {
			t.Error("AsMCPError() should return false for nil")
		}
	})

	t.Run("IsCategory", func(t *testing.T) {
		err := ValidationError("test error")

		if !IsCategory(err, CategoryValidation) {
			t.Error("IsCategory() should return true for matching category")
		}

		if IsCategory(err, CategoryTransport) {
			t.Error("IsCategory() should return false for non-matching category")
		}

		if IsCategory(fmt.Errorf("regular error"), CategoryValidation) {
			t.Error("IsCategory() should return false for regular errors")
		}
	})

	t.Run("IsCode", func(t *testing.T) {
		err := ResourceNotFound("tool", "test")

		if !IsCode(err, CodeResourceNotFound) {
			t.Error("IsCode() should return true for matching code")
		}

		if IsCode(err, CodeInvalidParams) {
			t.Error("IsCode() should return false for non-matching code")
		}
	})

	t.Run("IsRetryableError", func(t *testing.T) {
		retryableErr := ConnectionTimeout("http", "http://example.com", 30*time.Second)
		nonRetryableErr := ResourceNotFound("tool", "test")

		if !IsRetryableError(retryableErr) {
			t.Error("IsRetryableError() should return true for timeout errors")
		}

		if IsRetryableError(nonRetryableErr) {
			t.Error("IsRetryableError() should return false for not found errors")
		}
	})
}

func TestCombineErrors(t *testing.T) {
	t.Run("multiple errors", func(t *testing.T) {
		err1 := ValidationError("first error")
		err2 := ResourceNotFound("tool", "test")
		err3 := fmt.Errorf("regular error")

		combined := CombineErrors([]error{err1, err2, err3})

		if combined == nil {
			t.Fatal("CombineErrors() should not return nil")
		}

		if combined.Code() != CodeInternalError {
			t.Errorf("Combined error code = %v, want %v", combined.Code(), CodeInternalError)
		}

		data := combined.Data()
		if data == nil {
			t.Fatal("Combined error should have data")
		}

		dataMap, ok := data.(map[string]interface{})
		if !ok {
			t.Fatal("Combined error data should be a map")
		}

		if count := dataMap["count"]; count != 3 {
			t.Errorf("Combined error count = %v, want %v", count, 3)
		}
	})

	t.Run("single error", func(t *testing.T) {
		err := ValidationError("single error")
		combined := CombineErrors([]error{err})

		if combined != err {
			t.Error("CombineErrors() should return the single error unchanged")
		}
	})

	t.Run("no errors", func(t *testing.T) {
		combined := CombineErrors([]error{})
		if combined != nil {
			t.Error("CombineErrors() should return nil for empty slice")
		}
	})
}

func TestErrorDetails(t *testing.T) {
	err := ValidationError("base error").
		WithDetail("first detail").
		WithDetail("second detail")

	details := err.Details()
	expected := "first detail; second detail"
	if details != expected {
		t.Errorf("Details() = %v, want %v", details, expected)
	}
}

func BenchmarkErrorCreation(b *testing.B) {
	b.Run("NewError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = NewError(CodeValidationError, "test error", CategoryValidation, SeverityError)
		}
	})

	b.Run("ValidationError", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ValidationError("test error")
		}
	})

	b.Run("ResourceNotFound", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = ResourceNotFound("tool", "test-tool")
		}
	})
}

// Test error code registry functions
func TestErrorCodeRegistry(t *testing.T) {
	t.Run("GetErrorCodeDescription", func(t *testing.T) {
		tests := []struct {
			code        int
			wantDesc    string
			unknownCode bool
		}{
			{CodeResourceNotFound, "Resource not found", false},
			{CodeInvalidParams, "Invalid method parameters", false},
			{CodeInternalError, "Internal JSON-RPC error", false},
			{-99999, "Unknown error", true}, // Unknown code
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
				desc := GetErrorCodeDescription(tt.code)
				if tt.unknownCode {
					if desc != "Unknown error" {
						t.Errorf("GetErrorCodeDescription() = %v, want 'Unknown error'", desc)
					}
				} else {
					if desc != tt.wantDesc {
						t.Errorf("GetErrorCodeDescription() = %v, want %v", desc, tt.wantDesc)
					}
				}
			})
		}
	})

	t.Run("GetErrorCodeSeverity", func(t *testing.T) {
		tests := []struct {
			code         int
			wantSeverity Severity
		}{
			{CodeServerInitError, SeverityCritical},
			{CodeResourceNotFound, SeverityError},
			{CodeTokenExpired, SeverityWarning},
			{CodeOperationCancelled, SeverityInfo},
			{-99999, SeverityError}, // Unknown code defaults to Error
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
				severity := GetErrorCodeSeverity(tt.code)
				if severity != tt.wantSeverity {
					t.Errorf("GetErrorCodeSeverity() = %v, want %v", severity, tt.wantSeverity)
				}
			})
		}
	})

	t.Run("ListErrorCodes", func(t *testing.T) {
		codes := ListErrorCodes()
		if len(codes) == 0 {
			t.Error("ListErrorCodes() should return non-empty list")
		}

		// Check that we have all major error codes
		foundCodes := make(map[int]bool)
		for _, info := range codes {
			foundCodes[info.Code] = true
		}

		expectedCodes := []int{
			CodeParseError, CodeInvalidRequest, CodeMethodNotFound,
			CodeInvalidParams, CodeInternalError, CodeResourceNotFound,
			CodeOperationCancelled, CodeTransportError,
		}

		for _, code := range expectedCodes {
			if !foundCodes[code] {
				t.Errorf("ListErrorCodes() missing expected code %d", code)
			}
		}
	})

	t.Run("IsStandardJSONRPCCode", func(t *testing.T) {
		tests := []struct {
			code         int
			wantStandard bool
		}{
			{CodeParseError, true},
			{CodeInvalidRequest, true},
			{CodeMethodNotFound, true},
			{CodeInvalidParams, true},
			{CodeInternalError, true},
			{CodeResourceNotFound, true}, // MCP-specific but in range
			{-32100, true},               // In range
			{-32769, false},              // Out of range
			{0, false},                   // Not negative
			{-1999, false},               // Too high
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
				result := IsStandardJSONRPCCode(tt.code)
				if result != tt.wantStandard {
					t.Errorf("IsStandardJSONRPCCode(%d) = %v, want %v", tt.code, result, tt.wantStandard)
				}
			})
		}
	})

	t.Run("IsMCPSpecificCode", func(t *testing.T) {
		tests := []struct {
			code    int
			wantMCP bool
		}{
			{CodeResourceNotFound, true},
			{CodeOperationCancelled, true},
			{CodeTransportError, true},
			{-32000, true},  // Edge case
			{-32999, true},  // Edge case
			{-31999, false}, // Out of range (above -32000)
			{-33000, false}, // Out of range
			{0, false},      // Not negative
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
				result := IsMCPSpecificCode(tt.code)
				if result != tt.wantMCP {
					t.Errorf("IsMCPSpecificCode(%d) = %v, want %v", tt.code, result, tt.wantMCP)
				}
			})
		}
	})
}

// Test all MCP-specific error constructors
func TestMCPSpecificErrorConstructors(t *testing.T) {
	t.Run("ResourceNotFoundByURI", func(t *testing.T) {
		uri := "file:///test/resource"
		err := ResourceNotFoundByURI(uri)
		if err.Code() != CodeResourceNotFound {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeResourceNotFound)
		}
		if err.Category() != CategoryNotFound {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryNotFound)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if resData, ok := data.(*ResourceErrorData); !ok {
			t.Error("Data() should be *ResourceErrorData")
		} else if resData.URI != uri {
			t.Errorf("Data.URI = %v, want %v", resData.URI, uri)
		}
	})

	t.Run("ResourceUnavailable", func(t *testing.T) {
		err := ResourceUnavailable("database", "test-db", "maintenance")
		if err.Code() != CodeResourceUnavailable {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeResourceUnavailable)
		}
		if err.Severity() != SeverityWarning {
			t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityWarning)
		}
	})

	t.Run("ResourceConflict", func(t *testing.T) {
		err := ResourceConflict("user", "user123", "already exists")
		if err.Code() != CodeResourceConflict {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeResourceConflict)
		}
		if err.Category() != CategoryValidation {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryValidation)
		}
	})

	t.Run("ResourceLocked", func(t *testing.T) {
		err := ResourceLocked("file", "test.txt")
		if err.Code() != CodeResourceLocked {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeResourceLocked)
		}
		if err.Severity() != SeverityWarning {
			t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityWarning)
		}
	})

	t.Run("ProviderUnavailable", func(t *testing.T) {
		err := ProviderUnavailable("database", "connection lost")
		if err.Code() != CodeProviderUnavailable {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeProviderUnavailable)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if provData, ok := data.(*ProviderErrorData); !ok {
			t.Error("Data() should be *ProviderErrorData")
		} else if provData.Available {
			t.Error("Data.Available should be false")
		}
	})

	t.Run("ProviderError", func(t *testing.T) {
		cause := fmt.Errorf("database connection failed")
		err := ProviderError("database", "query", cause)
		if err.Code() != CodeProviderError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeProviderError)
		}
		if err.Unwrap() != cause {
			t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
		}
	})

	t.Run("InvalidCapability", func(t *testing.T) {
		err := InvalidCapability("tools", "not supported")
		if err.Code() != CodeInvalidCapability {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidCapability)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if capData, ok := data.(*CapabilityErrorData); !ok {
			t.Error("Data() should be *CapabilityErrorData")
		} else if capData.Supported {
			t.Error("Data.Supported should be false")
		}
	})

	t.Run("CapabilityRequired", func(t *testing.T) {
		err := CapabilityRequired("sampling")
		if err.Code() != CodeCapabilityRequired {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeCapabilityRequired)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if capData, ok := data.(*CapabilityErrorData); !ok {
			t.Error("Data() should be *CapabilityErrorData")
		} else if !capData.Required {
			t.Error("Data.Required should be true")
		}
	})

	t.Run("CapabilityMismatch", func(t *testing.T) {
		err := CapabilityMismatch("mcp", "1.0", "0.9")
		if err.Code() != CodeCapabilityMismatch {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeCapabilityMismatch)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if capData, ok := data.(*CapabilityErrorData); !ok {
			t.Error("Data() should be *CapabilityErrorData")
		} else if capData.Version != "0.9" {
			t.Errorf("Data.Version = %v, want '0.9'", capData.Version)
		}
	})

	t.Run("OperationTimeout", func(t *testing.T) {
		err := OperationTimeout("list_tools", "30s")
		if err.Code() != CodeOperationTimeout {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeOperationTimeout)
		}
		if err.Category() != CategoryTimeout {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryTimeout)
		}
	})

	t.Run("OperationFailed", func(t *testing.T) {
		err := OperationFailed("call_tool", "invalid input")
		if err.Code() != CodeOperationFailed {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeOperationFailed)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if opData, ok := data.(*OperationErrorData); !ok {
			t.Error("Data() should be *OperationErrorData")
		} else if opData.Retryable {
			t.Error("Data.Retryable should be false")
		}
	})

	t.Run("OperationNotSupported", func(t *testing.T) {
		err := OperationNotSupported("streaming", "stdio")
		if err.Code() != CodeOperationNotSupported {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeOperationNotSupported)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if opData, ok := data.(*OperationErrorData); !ok {
			t.Error("Data() should be *OperationErrorData")
		} else if opData.Component != "stdio" {
			t.Errorf("Data.Component = %v, want 'stdio'", opData.Component)
		}
	})

	t.Run("AuthRequired", func(t *testing.T) {
		err := AuthRequired("bearer")
		if err.Code() != CodeAuthRequired {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeAuthRequired)
		}
		if err.Category() != CategoryAuth {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryAuth)
		}
	})

	t.Run("InvalidToken", func(t *testing.T) {
		err := InvalidToken("jwt", "expired")
		if err.Code() != CodeInvalidToken {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidToken)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if authData, ok := data.(*AuthErrorData); !ok {
			t.Error("Data() should be *AuthErrorData")
		} else if authData.TokenType != "jwt" {
			t.Errorf("Data.TokenType = %v, want 'jwt'", authData.TokenType)
		}
	})

	t.Run("TokenExpired", func(t *testing.T) {
		err := TokenExpired("session")
		if err.Code() != CodeTokenExpired {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTokenExpired)
		}
		if err.Severity() != SeverityWarning {
			t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityWarning)
		}
	})

	t.Run("InsufficientPermissions", func(t *testing.T) {
		perms := []string{"read", "write"}
		err := InsufficientPermissions(perms, "missing write access")
		if err.Code() != CodeInsufficientPerms {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInsufficientPerms)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if authData, ok := data.(*AuthErrorData); !ok {
			t.Error("Data() should be *AuthErrorData")
		} else if len(authData.Permissions) != 2 {
			t.Errorf("Data.Permissions length = %v, want 2", len(authData.Permissions))
		}
	})

	t.Run("ServerInitError", func(t *testing.T) {
		cause := fmt.Errorf("config error")
		err := ServerInitError("invalid config", cause)
		if err.Code() != CodeServerInitError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeServerInitError)
		}
		if err.Severity() != SeverityCritical {
			t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityCritical)
		}
		if err.Unwrap() != cause {
			t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
		}
	})

	t.Run("ServerNotReady", func(t *testing.T) {
		err := ServerNotReady("initializing")
		if err.Code() != CodeServerNotReady {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeServerNotReady)
		}
		if err.Category() != CategoryInternal {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryInternal)
		}
	})

	t.Run("ProtocolError", func(t *testing.T) {
		err := ProtocolError("invalid message format")
		if err.Code() != CodeProtocolError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeProtocolError)
		}
		if err.Category() != CategoryProtocol {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryProtocol)
		}
	})

	t.Run("VersionMismatch", func(t *testing.T) {
		err := VersionMismatch("2.0", "1.0")
		if err.Code() != CodeVersionMismatch {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeVersionMismatch)
		}
		if !strings.Contains(err.Message(), "expected 2.0") {
			t.Errorf("Message should contain expected version")
		}
	})

	t.Run("InvalidSequence", func(t *testing.T) {
		err := InvalidSequence("initialize", "call_tool")
		if err.Code() != CodeInvalidSequence {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidSequence)
		}
	})

	t.Run("UnsupportedFeature", func(t *testing.T) {
		err := UnsupportedFeature("batch_requests")
		if err.Code() != CodeUnsupportedFeature {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeUnsupportedFeature)
		}
	})
}

// Test transport error constructors
func TestTransportErrorConstructors(t *testing.T) {
	t.Run("ConnectionLost", func(t *testing.T) {
		cause := fmt.Errorf("connection reset by peer")
		err := ConnectionLost("http", "http://example.com", cause)
		if err.Code() != CodeConnectionLost {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeConnectionLost)
		}
		if err.Category() != CategoryTransport {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryTransport)
		}
		if err.Unwrap() != cause {
			t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
		}
	})

	t.Run("HTTPTransportError", func(t *testing.T) {
		cause := fmt.Errorf("server error")
		err := HTTPTransportError("send", "http://api.test", 500, cause)
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if tData, ok := data.(*TransportErrorData); !ok {
			t.Error("Data() should be *TransportErrorData")
		} else {
			if tData.StatusCode != 500 {
				t.Errorf("Data.StatusCode = %v, want 500", tData.StatusCode)
			}
			if !tData.Retryable {
				t.Error("Data.Retryable should be true for 500 status")
			}
			if !tData.Connected {
				t.Error("Data.Connected should be true when status code > 0")
			}
		}

		// Test non-retryable status code
		err2 := HTTPTransportError("send", "http://api.test", 404, cause)
		if data := err2.Data(); data != nil {
			if tData, ok := data.(*TransportErrorData); ok && tData.Retryable {
				t.Error("Data.Retryable should be false for 404 status")
			}
		}
	})

	t.Run("StdioTransportError", func(t *testing.T) {
		cause := fmt.Errorf("pipe broken")
		err := StdioTransportError("read", cause)
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if tData, ok := data.(*TransportErrorData); !ok {
			t.Error("Data() should be *TransportErrorData")
		} else {
			if tData.Transport != "stdio" {
				t.Errorf("Data.Transport = %v, want 'stdio'", tData.Transport)
			}
			if tData.Retryable {
				t.Error("Data.Retryable should be false for stdio")
			}
			if !tData.Connected {
				t.Error("Data.Connected should be true for stdio")
			}
		}
	})

	t.Run("StreamableHTTPError", func(t *testing.T) {
		cause := fmt.Errorf("stream error")
		err := StreamableHTTPError("read", "http://stream.test", "stream-123", cause)
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if dataMap, ok := data.(map[string]interface{}); !ok {
			t.Error("Data() should be map with stream_id")
		} else {
			if dataMap["stream_id"] != "stream-123" {
				t.Errorf("Data stream_id = %v, want 'stream-123'", dataMap["stream_id"])
			}
			if dataMap["transport"] != "streamable_http" {
				t.Errorf("Data transport = %v, want 'streamable_http'", dataMap["transport"])
			}
		}

		// Test without stream ID
		err2 := StreamableHTTPError("write", "http://stream.test", "", cause)
		if data := err2.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if _, ok := data.(*TransportErrorData); !ok {
			t.Error("Data() should be *TransportErrorData when no stream ID")
		}
	})

	t.Run("EventSourceError", func(t *testing.T) {
		cause := fmt.Errorf("connection failed")
		err := EventSourceError("http://events.test", "stream closed", cause)
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if tData, ok := data.(*TransportErrorData); !ok {
			t.Error("Data() should be *TransportErrorData")
		} else if tData.Transport != "sse" {
			t.Errorf("Data.Transport = %v, want 'sse'", tData.Transport)
		}
	})

	t.Run("MessageSendError", func(t *testing.T) {
		cause := fmt.Errorf("send failed")
		err := MessageSendError("websocket", "request", cause)
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if tData, ok := data.(*TransportErrorData); !ok {
			t.Error("Data() should be *TransportErrorData")
		} else if tData.Operation != "send_message" {
			t.Errorf("Data.Operation = %v, want 'send_message'", tData.Operation)
		}
	})

	t.Run("MessageReceiveError", func(t *testing.T) {
		cause := fmt.Errorf("receive failed")
		err := MessageReceiveError("tcp", "response", cause)
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if tData, ok := data.(*TransportErrorData); !ok {
			t.Error("Data() should be *TransportErrorData")
		} else if tData.Operation != "receive_message" {
			t.Errorf("Data.Operation = %v, want 'receive_message'", tData.Operation)
		}
	})

	t.Run("TransportNotInitialized", func(t *testing.T) {
		err := TransportNotInitialized("http")
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if tData, ok := data.(*TransportErrorData); !ok {
			t.Error("Data() should be *TransportErrorData")
		} else if tData.Retryable {
			t.Error("Data.Retryable should be false")
		}
	})

	t.Run("TransportAlreadyRunning", func(t *testing.T) {
		err := TransportAlreadyRunning("stdio")
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if err.Severity() != SeverityWarning {
			t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityWarning)
		}
	})

	t.Run("TransportNotRunning", func(t *testing.T) {
		err := TransportNotRunning("http")
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if err.Severity() != SeverityError {
			t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityError)
		}
	})

	t.Run("InvalidTransportConfiguration", func(t *testing.T) {
		err := InvalidTransportConfiguration("http", "timeout", "must be positive")
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if !strings.Contains(err.Message(), "timeout") {
			t.Error("Message should contain parameter name")
		}
	})

	t.Run("MessageTooLarge", func(t *testing.T) {
		err := MessageTooLarge("websocket", 1000000, 65536)
		if err.Code() != CodeTransportError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeTransportError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if tData, ok := data.(*TransportErrorData); !ok {
			t.Error("Data() should be *TransportErrorData")
		} else if tData.Retryable {
			t.Error("Data.Retryable should be false for message too large")
		}
	})

	t.Run("ResponseTimeout", func(t *testing.T) {
		err := ResponseTimeout("http", "req-123", 30*time.Second)
		if err.Code() != CodeOperationTimeout {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeOperationTimeout)
		}
		if err.Category() != CategoryTimeout {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryTimeout)
		}
		if !strings.Contains(err.Message(), "req-123") {
			t.Error("Message should contain request ID")
		}
	})
}

// Test additional conversion functions
func TestAdditionalConversions(t *testing.T) {
	t.Run("ToJSONRPCError", func(t *testing.T) {
		err := ResourceNotFound("tool", "test")
		jsonrpcErr := ToJSONRPCError(err)
		if jsonrpcErr == nil {
			t.Fatal("ToJSONRPCError() should not return nil")
		}
		if jsonrpcErr.Code != CodeResourceNotFound {
			t.Errorf("Code = %v, want %v", jsonrpcErr.Code, CodeResourceNotFound)
		}
		if jsonrpcErr.Message != err.Message() {
			t.Errorf("Message = %v, want %v", jsonrpcErr.Message, err.Message())
		}

		// Test with nil error
		if result := ToJSONRPCError(nil); result != nil {
			t.Error("ToJSONRPCError(nil) should return nil")
		}

		// Test with regular error
		regularErr := fmt.Errorf("regular error")
		jsonrpcErr2 := ToJSONRPCError(regularErr)
		if jsonrpcErr2.Code != CodeInternalError {
			t.Errorf("Regular error Code = %v, want %v", jsonrpcErr2.Code, CodeInternalError)
		}
	})

	t.Run("FromProtocolErrorObject", func(t *testing.T) {
		errorObj := &protocol.ErrorObject{
			Code:    CodeValidationError,
			Message: "Validation failed",
			Data:    map[string]string{"field": "test"},
		}

		err := FromProtocolErrorObject(errorObj)
		if err == nil {
			t.Fatal("FromProtocolErrorObject() should not return nil")
		}
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if err.Message() != "Validation failed" {
			t.Errorf("Message() = %v, want 'Validation failed'", err.Message())
		}

		// Test with nil
		if result := FromProtocolErrorObject(nil); result != nil {
			t.Error("FromProtocolErrorObject(nil) should return nil")
		}
	})

	t.Run("ToProtocolError", func(t *testing.T) {
		err := ValidationError("test error")
		protocolErr := ToProtocolError(err)
		if protocolErr == nil {
			t.Fatal("ToProtocolError() should not return nil")
		}
		if protocolErr.Code != err.Code() {
			t.Errorf("Code = %v, want %v", protocolErr.Code, err.Code())
		}

		// Test with nil
		if result := ToProtocolError(nil); result != nil {
			t.Error("ToProtocolError(nil) should return nil")
		}
	})

	t.Run("ToProtocolErrorObject", func(t *testing.T) {
		err := ValidationError("test error")
		errorObj := ToProtocolErrorObject(err)
		if errorObj == nil {
			t.Fatal("ToProtocolErrorObject() should not return nil")
		}
		if errorObj.Code != err.Code() {
			t.Errorf("Code = %v, want %v", errorObj.Code, err.Code())
		}

		// Test with nil
		if result := ToProtocolErrorObject(nil); result != nil {
			t.Error("ToProtocolErrorObject(nil) should return nil")
		}
	})

	t.Run("WrapProtocolError", func(t *testing.T) {
		cause := fmt.Errorf("underlying error")
		err := WrapProtocolError(cause, "test_method", "req-123")
		if err == nil {
			t.Fatal("WrapProtocolError() should not return nil")
		}
		if err.Code() != CodeInternalError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInternalError)
		}
		if err.Unwrap() != cause {
			t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
		}
		if ctx := err.Context(); ctx == nil {
			t.Error("Context() should not be nil")
		} else {
			if ctx.Method != "test_method" {
				t.Errorf("Context.Method = %v, want 'test_method'", ctx.Method)
			}
			if ctx.RequestID != "req-123" {
				t.Errorf("Context.RequestID = %v, want 'req-123'", ctx.RequestID)
			}
		}

		// Test with MCP error
		mcpErr := ValidationError("test")
		wrapped := WrapProtocolError(mcpErr, "method", "123")
		if wrapped.Code() != mcpErr.Code() {
			t.Errorf("Wrapped MCP error should preserve code")
		}

		// Test with nil
		if result := WrapProtocolError(nil, "method", "123"); result != nil {
			t.Error("WrapProtocolError(nil) should return nil")
		}
	})

	t.Run("CreateMethodNotFoundError", func(t *testing.T) {
		err := CreateMethodNotFoundError("unknown_method", "req-456")
		if err.Code() != CodeMethodNotFound {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeMethodNotFound)
		}
		if !strings.Contains(err.Message(), "unknown_method") {
			t.Error("Message should contain method name")
		}
		if ctx := err.Context(); ctx == nil {
			t.Error("Context() should not be nil")
		} else if ctx.RequestID != "req-456" {
			t.Errorf("Context.RequestID = %v, want 'req-456'", ctx.RequestID)
		}
	})

	t.Run("CreateInvalidParamsError", func(t *testing.T) {
		err := CreateInvalidParamsError("test_method", "req-789", "missing required param")
		if err.Code() != CodeInvalidParams {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidParams)
		}
		if !strings.Contains(err.Message(), "missing required param") {
			t.Error("Message should contain details")
		}
		if err.Details() != "missing required param" {
			t.Errorf("Details() = %v, want 'missing required param'", err.Details())
		}

		// Test without details
		err2 := CreateInvalidParamsError("method", "123", "")
		if err2.Message() != "Invalid method parameters" {
			t.Errorf("Message without details = %v, want 'Invalid method parameters'", err2.Message())
		}
	})

	t.Run("CreateParseError", func(t *testing.T) {
		err := CreateParseError("invalid JSON syntax")
		if err.Code() != CodeParseError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeParseError)
		}
		if !strings.Contains(err.Message(), "invalid JSON syntax") {
			t.Error("Message should contain details")
		}

		// Test without details
		err2 := CreateParseError("")
		if err2.Message() != "Parse error" {
			t.Errorf("Message without details = %v, want 'Parse error'", err2.Message())
		}
	})

	t.Run("CreateInvalidRequestError", func(t *testing.T) {
		err := CreateInvalidRequestError("missing method field")
		if err.Code() != CodeInvalidRequest {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidRequest)
		}
		if !strings.Contains(err.Message(), "missing method field") {
			t.Error("Message should contain details")
		}

		// Test without details
		err2 := CreateInvalidRequestError("")
		if err2.Message() != "Invalid Request" {
			t.Errorf("Message without details = %v, want 'Invalid Request'", err2.Message())
		}
	})

	t.Run("CreateInternalError", func(t *testing.T) {
		cause := fmt.Errorf("database error")
		err := CreateInternalError("user_lookup", cause)
		if err.Code() != CodeInternalError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInternalError)
		}
		if err.Unwrap() != cause {
			t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
		}
		if ctx := err.Context(); ctx == nil {
			t.Error("Context() should not be nil")
		} else if ctx.Operation != "user_lookup" {
			t.Errorf("Context.Operation = %v, want 'user_lookup'", ctx.Operation)
		}

		// Test without operation
		err2 := CreateInternalError("", cause)
		if err2.Message() != "Internal error" {
			t.Errorf("Message without operation = %v, want 'Internal error'", err2.Message())
		}
	})
}

// Test additional validation error constructors
func TestAdditionalValidationErrors(t *testing.T) {
	t.Run("ValidationErrorf", func(t *testing.T) {
		err := ValidationErrorf("Field %s is invalid: %d", "count", 42)
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if err.Category() != CategoryValidation {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryValidation)
		}
		if !strings.Contains(err.Message(), "Field count is invalid: 42") {
			t.Errorf("Message() = %v, should contain formatted text", err.Message())
		}
	})

	t.Run("InvalidParameterType", func(t *testing.T) {
		err := InvalidParameterType("age", "not a number", "int")
		if err.Code() != CodeInvalidParameter {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidParameter)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if pData, ok := data.(*ParameterErrorData); !ok {
			t.Error("Data() should be *ParameterErrorData")
		} else {
			if pData.Parameter != "age" {
				t.Errorf("Data.Parameter = %v, want 'age'", pData.Parameter)
			}
			if pData.Type != "string" {
				t.Errorf("Data.Type = %v, want 'string'", pData.Type)
			}
		}

		// Test with nil value
		err2 := InvalidParameterType("value", nil, "string")
		if data := err2.Data(); data != nil {
			if pData, ok := data.(*ParameterErrorData); ok && pData.Type != "nil" {
				t.Errorf("Data.Type for nil = %v, want 'nil'", pData.Type)
			}
		}
	})

	t.Run("ParameterTooLarge", func(t *testing.T) {
		err := ParameterTooLarge("size", 1000, 100)
		if err.Code() != CodeParameterTooLarge {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeParameterTooLarge)
		}
		if !strings.Contains(err.Message(), "exceeds maximum") {
			t.Error("Message should contain 'exceeds maximum'")
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if pData, ok := data.(*ParameterErrorData); !ok {
			t.Error("Data() should be *ParameterErrorData")
		} else if pData.Value != 1000 {
			t.Errorf("Data.Value = %v, want 1000", pData.Value)
		}
	})

	t.Run("ParameterTooSmall", func(t *testing.T) {
		err := ParameterTooSmall("count", -5, 0)
		if err.Code() != CodeParameterTooSmall {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeParameterTooSmall)
		}
		if !strings.Contains(err.Message(), "below minimum") {
			t.Error("Message should contain 'below minimum'")
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if pData, ok := data.(*ParameterErrorData); !ok {
			t.Error("Data() should be *ParameterErrorData")
		} else if pData.Value != -5 {
			t.Errorf("Data.Value = %v, want -5", pData.Value)
		}
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		err := InvalidFormat("email", "invalid-email", "email format")
		if err.Code() != CodeInvalidFormat {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidFormat)
		}
		if !strings.Contains(err.Message(), "invalid format") {
			t.Error("Message should contain 'invalid format'")
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if pData, ok := data.(*ParameterErrorData); !ok {
			t.Error("Data() should be *ParameterErrorData")
		} else if pData.Parameter != "email" {
			t.Errorf("Data.Parameter = %v, want 'email'", pData.Parameter)
		}
	})

	t.Run("InvalidPaginationCursor", func(t *testing.T) {
		err := InvalidPaginationCursor("invalid-cursor", "malformed base64")
		if err.Code() != CodeInvalidCursor {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidCursor)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if pData, ok := data.(*PaginationErrorData); !ok {
			t.Error("Data() should be *PaginationErrorData")
		} else {
			if pData.Cursor != "invalid-cursor" {
				t.Errorf("Data.Cursor = %v, want 'invalid-cursor'", pData.Cursor)
			}
			if pData.Reason != "malformed base64" {
				t.Errorf("Data.Reason = %v, want 'malformed base64'", pData.Reason)
			}
		}
	})

	t.Run("InvalidPaginationLimit", func(t *testing.T) {
		// Test limit too large
		err := InvalidPaginationLimit(1000, 100)
		if err.Code() != CodeInvalidLimit {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInvalidLimit)
		}
		if !strings.Contains(err.Message(), "exceeds maximum") {
			t.Error("Message should contain 'exceeds maximum'")
		}

		// Test negative limit
		err2 := InvalidPaginationLimit(-1, 100)
		if !strings.Contains(err2.Message(), "must be positive") {
			t.Error("Message should contain 'must be positive'")
		}

		// Test zero limit
		err3 := InvalidPaginationLimit(0, 100)
		if !strings.Contains(err3.Message(), "must be positive") {
			t.Error("Message should contain 'must be positive'")
		}
	})

	t.Run("InvalidFieldValue", func(t *testing.T) {
		err := InvalidFieldValue("status", "invalid", "must be active or inactive")
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if vData, ok := data.(*ValidationErrorData); !ok {
			t.Error("Data() should be *ValidationErrorData")
		} else {
			if vData.Field != "status" {
				t.Errorf("Data.Field = %v, want 'status'", vData.Field)
			}
			if vData.Constraint != "must be active or inactive" {
				t.Errorf("Data.Constraint = %v, want constraint text", vData.Constraint)
			}
		}
	})

	t.Run("RequiredFieldMissing", func(t *testing.T) {
		err := RequiredFieldMissing("name")
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if !strings.Contains(err.Message(), "Required field 'name' is missing") {
			t.Error("Message should contain required field text")
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if vData, ok := data.(*ValidationErrorData); !ok {
			t.Error("Data() should be *ValidationErrorData")
		} else {
			if vData.Field != "name" {
				t.Errorf("Data.Field = %v, want 'name'", vData.Field)
			}
			if vData.Expected != "required value" {
				t.Errorf("Data.Expected = %v, want 'required value'", vData.Expected)
			}
		}
	})

	t.Run("InvalidEnum", func(t *testing.T) {
		validValues := []string{"active", "inactive", "pending"}
		err := InvalidEnum("status", "unknown", validValues)
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if !strings.Contains(err.Message(), "must be one of") {
			t.Error("Message should contain enum values")
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if vData, ok := data.(*ValidationErrorData); !ok {
			t.Error("Data() should be *ValidationErrorData")
		} else if vData.Constraint != "enumeration" {
			t.Errorf("Data.Constraint = %v, want 'enumeration'", vData.Constraint)
		}
	})

	t.Run("StringTooLong", func(t *testing.T) {
		longString := "this is a very long string that exceeds the limit"
		err := StringTooLong("description", longString, 20)
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if !strings.Contains(err.Message(), "exceeds maximum length") {
			t.Error("Message should contain length validation text")
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if vData, ok := data.(*ValidationErrorData); !ok {
			t.Error("Data() should be *ValidationErrorData")
		} else {
			if vData.Constraint != "max_length" {
				t.Errorf("Data.Constraint = %v, want 'max_length'", vData.Constraint)
			}
			if !strings.Contains(vData.Got, fmt.Sprintf("%d characters", len(longString))) {
				t.Errorf("Data.Got = %v, should contain actual length", vData.Got)
			}
		}
	})

	t.Run("StringTooShort", func(t *testing.T) {
		shortString := "hi"
		err := StringTooShort("password", shortString, 8)
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if !strings.Contains(err.Message(), "below minimum length") {
			t.Error("Message should contain minimum length text")
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if vData, ok := data.(*ValidationErrorData); !ok {
			t.Error("Data() should be *ValidationErrorData")
		} else if vData.Constraint != "min_length" {
			t.Errorf("Data.Constraint = %v, want 'min_length'", vData.Constraint)
		}
	})

	t.Run("InvalidRegex", func(t *testing.T) {
		err := InvalidRegex("username", "invalid-user!", "^[a-zA-Z0-9_]+$")
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if !strings.Contains(err.Message(), "does not match required pattern") {
			t.Error("Message should contain pattern validation text")
		}
		if data := err.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if vData, ok := data.(*ValidationErrorData); !ok {
			t.Error("Data() should be *ValidationErrorData")
		} else {
			if vData.Constraint != "regex" {
				t.Errorf("Data.Constraint = %v, want 'regex'", vData.Constraint)
			}
			if vData.Expected != "^[a-zA-Z0-9_]+$" {
				t.Errorf("Data.Expected = %v, want pattern", vData.Expected)
			}
		}
	})

	t.Run("CombineValidationErrors", func(t *testing.T) {
		err1 := RequiredFieldMissing("name")
		err2 := StringTooShort("password", "123", 8)
		err3 := InvalidEnum("status", "unknown", []string{"active", "inactive"})

		combined := CombineValidationErrors([]MCPError{err1, err2, err3})
		if combined == nil {
			t.Fatal("CombineValidationErrors() should not return nil")
		}
		if combined.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", combined.Code(), CodeValidationError)
		}
		if !strings.Contains(combined.Message(), "Multiple validation errors") {
			t.Error("Message should indicate multiple errors")
		}
		if data := combined.Data(); data == nil {
			t.Error("Data() should not be nil")
		} else if dataMap, ok := data.(map[string]interface{}); !ok {
			t.Error("Data() should be a map")
		} else if count := dataMap["count"]; count != 3 {
			t.Errorf("Data count = %v, want 3", count)
		}

		// Test single error
		single := CombineValidationErrors([]MCPError{err1})
		if single != err1 {
			t.Error("Single error should be returned unchanged")
		}

		// Test empty slice
		empty := CombineValidationErrors([]MCPError{})
		if empty != nil {
			t.Error("Empty slice should return nil")
		}
	})
}

// Test additional type functions and edge cases
func TestAdditionalTypeFunctions(t *testing.T) {
	t.Run("NewErrorf", func(t *testing.T) {
		err := NewErrorf(CodeValidationError, CategoryValidation, SeverityError, "Field %s has value %d", "count", 42)
		if err.Code() != CodeValidationError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeValidationError)
		}
		if err.Category() != CategoryValidation {
			t.Errorf("Category() = %v, want %v", err.Category(), CategoryValidation)
		}
		if err.Severity() != SeverityError {
			t.Errorf("Severity() = %v, want %v", err.Severity(), SeverityError)
		}
		if !strings.Contains(err.Message(), "Field count has value 42") {
			t.Errorf("Message() = %v, should contain formatted text", err.Message())
		}
		if err.Context() == nil {
			t.Error("Context() should not be nil")
		}
		if err.Context().Timestamp.IsZero() {
			t.Error("Context timestamp should be set")
		}
	})

	t.Run("WrapErrorf", func(t *testing.T) {
		cause := fmt.Errorf("underlying error")
		err := WrapErrorf(cause, CodeInternalError, CategoryInternal, SeverityError, "Operation %s failed with code %d", "test", 500)
		if err.Code() != CodeInternalError {
			t.Errorf("Code() = %v, want %v", err.Code(), CodeInternalError)
		}
		if err.Unwrap() != cause {
			t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), cause)
		}
		if !strings.Contains(err.Message(), "Operation test failed with code 500") {
			t.Errorf("Message() = %v, should contain formatted text", err.Message())
		}
	})

	t.Run("IsMCPError", func(t *testing.T) {
		mcpErr := ValidationError("test")
		if !IsMCPError(mcpErr) {
			t.Error("IsMCPError() should return true for MCPError")
		}

		regularErr := fmt.Errorf("regular error")
		if IsMCPError(regularErr) {
			t.Error("IsMCPError() should return false for regular error")
		}

		if IsMCPError(nil) {
			t.Error("IsMCPError() should return false for nil")
		}
	})

	t.Run("MarshalJSON", func(t *testing.T) {
		err := ResourceNotFound("tool", "test-tool").WithDetail("Additional info")
		jsonBytes, marshalErr := json.Marshal(err)
		if marshalErr != nil {
			t.Fatalf("MarshalJSON() error = %v", marshalErr)
		}

		var result map[string]interface{}
		if unmarshalErr := json.Unmarshal(jsonBytes, &result); unmarshalErr != nil {
			t.Fatalf("Unmarshal error = %v", unmarshalErr)
		}

		if code, ok := result["code"].(float64); !ok || int(code) != CodeResourceNotFound {
			t.Errorf("JSON code = %v, want %v", result["code"], CodeResourceNotFound)
		}
		if message, ok := result["message"].(string); !ok || !strings.Contains(message, "not found") {
			t.Errorf("JSON message = %v, should contain 'not found'", result["message"])
		}
		if category, ok := result["category"].(string); !ok || category != string(CategoryNotFound) {
			t.Errorf("JSON category = %v, want %v", result["category"], CategoryNotFound)
		}
		if details, ok := result["details"].(string); !ok || details != "Additional info" {
			t.Errorf("JSON details = %v, want 'Additional info'", result["details"])
		}
	})

	t.Run("EdgeCases", func(t *testing.T) {
		// Test error with empty details
		err := NewError(CodeInternalError, "test", CategoryInternal, SeverityError)
		if err.Error() != "test" {
			t.Errorf("Error() without details = %v, want 'test'", err.Error())
		}

		// Test error with details
		errWithDetails := err.WithDetail("detailed info")
		if !strings.Contains(errWithDetails.Error(), "test: detailed info") {
			t.Errorf("Error() with details = %v, should contain both message and details", errWithDetails.Error())
		}

		// Test multiple details
		errMultiDetails := errWithDetails.WithDetail("more info")
		if errMultiDetails.Details() != "detailed info; more info" {
			t.Errorf("Details() = %v, want 'detailed info; more info'", errMultiDetails.Details())
		}

		// Test context with nil check
		base := &baseError{}
		if ctx := base.Context(); ctx != nil {
			t.Errorf("Context() for empty baseError should be nil, got %v", ctx)
		}

		// Test ToJSON with wrapped error (cause field)
		wrappedErr := WrapError(fmt.Errorf("cause"), CodeInternalError, "wrapped", CategoryInternal, SeverityError)
		result := wrappedErr.ToJSON()

		if result["cause"] != "cause" {
			t.Errorf("ToJSON() cause = %v, want 'cause'", result["cause"])
		}
	})
}

func BenchmarkErrorConversion(b *testing.B) {
	err := ResourceNotFound("tool", "test-tool")

	b.Run("ToJSONRPCResponse", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = ToJSONRPCResponse(err, "123")
		}
	})

	b.Run("ToJSON", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = err.ToJSON()
		}
	})
}
