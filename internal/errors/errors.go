package errors

import "fmt"

// ErrCode represents an error code
type ErrCode string

const (
	ErrCodeNotFound     ErrCode = "NOT_FOUND"
	ErrCodeUnauthorized ErrCode = "UNAUTHORIZED"
	ErrCodeRateLimited  ErrCode = "RATE_LIMITED"
	ErrCodeInternal     ErrCode = "INTERNAL_ERROR"
	ErrCodeBadRequest   ErrCode = "BAD_REQUEST"
	ErrCodeForbidden    ErrCode = "FORBIDDEN"
)

// AppError represents an application error
type AppError struct {
	Code    ErrCode
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Code:    ErrCodeNotFound,
		Message: fmt.Sprintf("%s not found", resource),
	}
}

// NewUnauthorizedError creates a new unauthorized error
func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeUnauthorized,
		Message: message,
	}
}

// NewRateLimitedError creates a new rate limited error
func NewRateLimitedError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeRateLimited,
		Message: message,
	}
}

// NewInternalError creates a new internal error
func NewInternalError(message string, err error) *AppError {
	return &AppError{
		Code:    ErrCodeInternal,
		Message: message,
		Err:     err,
	}
}

// NewBadRequestError creates a new bad request error
func NewBadRequestError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeBadRequest,
		Message: message,
	}
}

// NewForbiddenError creates a new forbidden error
func NewForbiddenError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeForbidden,
		Message: message,
	}
}

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrCodeNotFound
	}
	return false
}

// IsRateLimited checks if the error is a rate limited error
func IsRateLimited(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == ErrCodeRateLimited
	}
	return false
}
