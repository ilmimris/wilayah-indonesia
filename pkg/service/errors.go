// Package service provides business logic for the wilayah-indonesia API.
package service

import "fmt"

// Error represents a service error with a code and message.
type Error struct {
	Code    string
	Message string
}

// Error returns the error message.
func (e *Error) Error() string {
	return e.Message
}

// Error codes
const (
	ErrCodeInvalidInput    = "INVALID_INPUT"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeDatabaseFailure = "DATABASE_FAILURE"
)

// NewError creates a new service error with the specified code and message.
func NewError(code string, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}

// NewErrorf creates a new service error with the specified code and formatted message.
func NewErrorf(code string, format string, args ...interface{}) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// IsError checks if an error is a service error with the specified code.
func IsError(err error, code string) bool {
	if err == nil {
		return false
	}
	if svcErr, ok := err.(*Error); ok {
		return svcErr.Code == code
	}
	return false
}
