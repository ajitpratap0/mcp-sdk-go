package errors

import (
	"fmt"
	"reflect"
)

// ValidationErrorData contains structured data for validation errors
type ValidationErrorData struct {
	Field      string      `json:"field"`
	Value      interface{} `json:"value,omitempty"`
	Expected   string      `json:"expected"`
	Got        string      `json:"got,omitempty"`
	Constraint string      `json:"constraint,omitempty"`
}

// ParameterErrorData contains structured data for parameter-related errors
type ParameterErrorData struct {
	Parameter string      `json:"parameter"`
	Value     interface{} `json:"value,omitempty"`
	Type      string      `json:"type,omitempty"`
	Required  bool        `json:"required,omitempty"`
	Reason    string      `json:"reason,omitempty"`
}

// PaginationErrorData contains structured data for pagination errors
type PaginationErrorData struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Reason string `json:"reason"`
}

// ValidationError creates a generic validation error
func ValidationError(message string) MCPError {
	return NewError(CodeValidationError, message, CategoryValidation, SeverityError)
}

// ValidationErrorf creates a generic validation error with formatting
func ValidationErrorf(format string, args ...interface{}) MCPError {
	return NewErrorf(CodeValidationError, CategoryValidation, SeverityError, format, args...)
}

// InvalidParameter creates an error for invalid parameter values
func InvalidParameter(param string, value interface{}, expected string) MCPError {
	var got string
	if value != nil {
		got = fmt.Sprintf("%T", value)
		if str, ok := value.(string); ok && len(str) < 100 {
			got = fmt.Sprintf("%s(%q)", got, str)
		}
	} else {
		got = "nil"
	}

	return NewError(
		CodeInvalidParameter,
		fmt.Sprintf("Invalid parameter '%s': expected %s, got %s", param, expected, got),
		CategoryValidation,
		SeverityError,
	).WithData(&ParameterErrorData{
		Parameter: param,
		Value:     value,
		Type:      got,
		Reason:    fmt.Sprintf("expected %s", expected),
	})
}

// MissingParameter creates an error for missing required parameters
func MissingParameter(param string) MCPError {
	return NewError(
		CodeMissingParameter,
		fmt.Sprintf("Missing required parameter: %s", param),
		CategoryValidation,
		SeverityError,
	).WithData(&ParameterErrorData{
		Parameter: param,
		Required:  true,
	})
}

// InvalidParameterType creates an error for incorrect parameter types
func InvalidParameterType(param string, value interface{}, expectedType string) MCPError {
	actualType := "nil"
	if value != nil {
		actualType = reflect.TypeOf(value).String()
	}

	return NewError(
		CodeInvalidParameter,
		fmt.Sprintf("Invalid type for parameter '%s': expected %s, got %s", param, expectedType, actualType),
		CategoryValidation,
		SeverityError,
	).WithData(&ParameterErrorData{
		Parameter: param,
		Value:     value,
		Type:      actualType,
		Reason:    fmt.Sprintf("expected %s", expectedType),
	})
}

// ParameterTooLarge creates an error for parameter values that are too large
func ParameterTooLarge(param string, value interface{}, maxValue interface{}) MCPError {
	return NewError(
		CodeParameterTooLarge,
		fmt.Sprintf("Parameter '%s' value %v exceeds maximum allowed value %v", param, value, maxValue),
		CategoryValidation,
		SeverityError,
	).WithData(&ParameterErrorData{
		Parameter: param,
		Value:     value,
		Reason:    fmt.Sprintf("exceeds maximum %v", maxValue),
	})
}

// ParameterTooSmall creates an error for parameter values that are too small
func ParameterTooSmall(param string, value interface{}, minValue interface{}) MCPError {
	return NewError(
		CodeParameterTooSmall,
		fmt.Sprintf("Parameter '%s' value %v is below minimum allowed value %v", param, value, minValue),
		CategoryValidation,
		SeverityError,
	).WithData(&ParameterErrorData{
		Parameter: param,
		Value:     value,
		Reason:    fmt.Sprintf("below minimum %v", minValue),
	})
}

// InvalidFormat creates an error for incorrectly formatted parameters
func InvalidFormat(param string, value interface{}, expectedFormat string) MCPError {
	return NewError(
		CodeInvalidFormat,
		fmt.Sprintf("Parameter '%s' has invalid format: expected %s", param, expectedFormat),
		CategoryValidation,
		SeverityError,
	).WithData(&ParameterErrorData{
		Parameter: param,
		Value:     value,
		Reason:    fmt.Sprintf("expected format: %s", expectedFormat),
	})
}

// InvalidPagination creates an error for invalid pagination parameters
func InvalidPagination(reason string) MCPError {
	return NewError(
		CodeInvalidPagination,
		fmt.Sprintf("Invalid pagination: %s", reason),
		CategoryValidation,
		SeverityError,
	).WithData(&PaginationErrorData{
		Reason: reason,
	})
}

// InvalidPaginationCursor creates an error for invalid pagination cursors
func InvalidPaginationCursor(cursor string, reason string) MCPError {
	return NewError(
		CodeInvalidCursor,
		fmt.Sprintf("Invalid pagination cursor: %s", reason),
		CategoryValidation,
		SeverityError,
	).WithData(&PaginationErrorData{
		Cursor: cursor,
		Reason: reason,
	})
}

// InvalidPaginationLimit creates an error for invalid pagination limits
func InvalidPaginationLimit(limit int, maxLimit int) MCPError {
	var message string
	if limit <= 0 {
		message = "Pagination limit must be positive"
	} else {
		message = fmt.Sprintf("Pagination limit %d exceeds maximum allowed limit %d", limit, maxLimit)
	}

	return NewError(
		CodeInvalidLimit,
		message,
		CategoryValidation,
		SeverityError,
	).WithData(&PaginationErrorData{
		Limit:  limit,
		Reason: message,
	})
}

// InvalidFieldValue creates an error for invalid field values in structures
func InvalidFieldValue(field string, value interface{}, constraint string) MCPError {
	return NewError(
		CodeValidationError,
		fmt.Sprintf("Invalid value for field '%s': %s", field, constraint),
		CategoryValidation,
		SeverityError,
	).WithData(&ValidationErrorData{
		Field:      field,
		Value:      value,
		Constraint: constraint,
	})
}

// RequiredFieldMissing creates an error for missing required fields
func RequiredFieldMissing(field string) MCPError {
	return NewError(
		CodeValidationError,
		fmt.Sprintf("Required field '%s' is missing", field),
		CategoryValidation,
		SeverityError,
	).WithData(&ValidationErrorData{
		Field:    field,
		Expected: "required value",
		Got:      "missing",
	})
}

// InvalidEnum creates an error for invalid enumeration values
func InvalidEnum(field string, value interface{}, validValues []string) MCPError {
	return NewError(
		CodeValidationError,
		fmt.Sprintf("Invalid value for field '%s': must be one of %v", field, validValues),
		CategoryValidation,
		SeverityError,
	).WithData(&ValidationErrorData{
		Field:      field,
		Value:      value,
		Expected:   fmt.Sprintf("one of %v", validValues),
		Constraint: "enumeration",
	})
}

// StringTooLong creates an error for strings that exceed maximum length
func StringTooLong(field string, value string, maxLength int) MCPError {
	return NewError(
		CodeValidationError,
		fmt.Sprintf("Field '%s' exceeds maximum length of %d characters", field, maxLength),
		CategoryValidation,
		SeverityError,
	).WithData(&ValidationErrorData{
		Field:      field,
		Value:      value,
		Expected:   fmt.Sprintf("≤ %d characters", maxLength),
		Got:        fmt.Sprintf("%d characters", len(value)),
		Constraint: "max_length",
	})
}

// StringTooShort creates an error for strings that are below minimum length
func StringTooShort(field string, value string, minLength int) MCPError {
	return NewError(
		CodeValidationError,
		fmt.Sprintf("Field '%s' is below minimum length of %d characters", field, minLength),
		CategoryValidation,
		SeverityError,
	).WithData(&ValidationErrorData{
		Field:      field,
		Value:      value,
		Expected:   fmt.Sprintf("≥ %d characters", minLength),
		Got:        fmt.Sprintf("%d characters", len(value)),
		Constraint: "min_length",
	})
}

// InvalidRegex creates an error for strings that don't match required patterns
func InvalidRegex(field string, value string, pattern string) MCPError {
	return NewError(
		CodeValidationError,
		fmt.Sprintf("Field '%s' does not match required pattern", field),
		CategoryValidation,
		SeverityError,
	).WithData(&ValidationErrorData{
		Field:      field,
		Value:      value,
		Expected:   pattern,
		Constraint: "regex",
	})
}

// CombineValidationErrors combines multiple validation errors into a single error
func CombineValidationErrors(errors []MCPError) MCPError {
	if len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return errors[0]
	}

	messages := make([]string, len(errors))
	errorData := make([]interface{}, len(errors))

	for i, err := range errors {
		messages[i] = err.Message()
		errorData[i] = err.Data()
	}

	return NewError(
		CodeValidationError,
		fmt.Sprintf("Multiple validation errors: %v", messages),
		CategoryValidation,
		SeverityError,
	).WithData(map[string]interface{}{
		"errors": errorData,
		"count":  len(errors),
	})
}
