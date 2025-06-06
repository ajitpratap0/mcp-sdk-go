package errors

import (
	"context"
	"encoding/json"
	"fmt"
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
