package gomini

import (
	"fmt"
	"net/http"
	"strings"
	"time"
	
	"gomini/pkg/gomini/providers"
)

// ErrorCode represents different types of errors
type ErrorCode string

const (
	// Authentication errors
	ErrorInvalidAPIKey      ErrorCode = "invalid_api_key"
	ErrorInvalidAuth        ErrorCode = "invalid_auth"
	ErrorAuthRequired       ErrorCode = "auth_required"
	
	// Request errors  
	ErrorInvalidRequest     ErrorCode = "invalid_request"
	ErrorInvalidModel       ErrorCode = "invalid_model"
	ErrorInvalidParameters  ErrorCode = "invalid_parameters"
	ErrorRequestTooLarge    ErrorCode = "request_too_large"
	ErrorUnsupportedFeature ErrorCode = "unsupported_feature"
	
	// Rate limiting errors
	ErrorRateLimit          ErrorCode = "rate_limit"
	ErrorQuotaExceeded      ErrorCode = "quota_exceeded"
	ErrorTooManyRequests    ErrorCode = "too_many_requests"
	
	// Server errors
	ErrorServerError        ErrorCode = "server_error"
	ErrorServiceUnavailable ErrorCode = "service_unavailable"
	ErrorTimeout            ErrorCode = "timeout"
	ErrorInternalError      ErrorCode = "internal_error"
	
	// Content errors
	ErrorContentFiltered    ErrorCode = "content_filtered"
	ErrorSafetyViolation    ErrorCode = "safety_violation"
	ErrorTokenLimitExceeded ErrorCode = "token_limit_exceeded"
	
	// Provider errors
	ErrorProviderNotFound   ErrorCode = "provider_not_found"
	ErrorProviderDisabled   ErrorCode = "provider_disabled"
	ErrorProviderSwitch     ErrorCode = "provider_switch"
	ErrorAllProvidersFailed ErrorCode = "all_providers_failed"
	
	// Network errors
	ErrorNetworkError       ErrorCode = "network_error"
	ErrorConnectionFailed   ErrorCode = "connection_failed"
	ErrorDNSError          ErrorCode = "dns_error"
	
	// Validation errors
	ErrorValidation        ErrorCode = "validation_error"
	ErrorMissingField      ErrorCode = "missing_field"
	ErrorInvalidFormat     ErrorCode = "invalid_format"
	
	// Unknown errors
	ErrorUnknown           ErrorCode = "unknown_error"
)

// LLMError represents a unified error from any LLM provider
type LLMError struct {
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	Provider    providers.ProviderType           `json:"provider,omitempty"`
	Model       string                 `json:"model,omitempty"`
	HTTPStatus  int                    `json:"http_status,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Retryable   bool                   `json:"retryable"`
	RetryAfter  *time.Duration         `json:"retry_after,omitempty"`
	Cause       error                  `json:"-"` // Original error
	Timestamp   time.Time              `json:"timestamp"`
	RequestID   string                 `json:"request_id,omitempty"`
}

// Error implements the error interface
func (e *LLMError) Error() string {
	if e.Provider != "" {
		return fmt.Sprintf("[%s:%s] %s", e.Provider, e.Code, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *LLMError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error type
func (e *LLMError) Is(target error) bool {
	if t, ok := target.(*LLMError); ok {
		return e.Code == t.Code
	}
	return false
}

// IsRetryable returns true if the error is retryable
func (e *LLMError) IsRetryable() bool {
	return e.Retryable
}

// IsRateLimit returns true if the error is due to rate limiting
func (e *LLMError) IsRateLimit() bool {
	return e.Code == ErrorRateLimit || e.Code == ErrorQuotaExceeded || e.Code == ErrorTooManyRequests
}

// IsAuthError returns true if the error is authentication-related
func (e *LLMError) IsAuthError() bool {
	return e.Code == ErrorInvalidAPIKey || e.Code == ErrorInvalidAuth || e.Code == ErrorAuthRequired
}

// IsContentError returns true if the error is content-related (filtering, safety)
func (e *LLMError) IsContentError() bool {
	return e.Code == ErrorContentFiltered || e.Code == ErrorSafetyViolation
}

// IsProviderError returns true if the error is provider-related
func (e *LLMError) IsProviderError() bool {
	return e.Code == ErrorProviderNotFound || e.Code == ErrorProviderDisabled || 
		   e.Code == ErrorProviderSwitch || e.Code == ErrorAllProvidersFailed
}

// NewLLMError creates a new LLMError
func NewLLMError(code ErrorCode, message string, provider providers.ProviderType, cause error) *LLMError {
	return &LLMError{
		Code:      code,
		Message:   message,
		Provider:  provider,
		Cause:     cause,
		Timestamp: time.Now(),
		Retryable: isRetryableErrorCode(code),
	}
}

// NewLLMErrorWithDetails creates a new LLMError with additional details
func NewLLMErrorWithDetails(code ErrorCode, message string, provider providers.ProviderType, cause error, details map[string]interface{}) *LLMError {
	return &LLMError{
		Code:      code,
		Message:   message,
		Provider:  provider,
		Cause:     cause,
		Details:   details,
		Timestamp: time.Now(),
		Retryable: isRetryableErrorCode(code),
	}
}

// WrapProviderError wraps a provider-specific error into a unified LLMError
func WrapProviderError(err error, provider providers.ProviderType, model string) *LLMError {
	if err == nil {
		return nil
	}
	
	// If it's already an LLMError, just update the provider
	if llmErr, ok := err.(*LLMError); ok {
		llmErr.Provider = provider
		llmErr.Model = model
		return llmErr
	}
	
	// Map provider-specific errors to unified error codes
	code, message, httpStatus, retryable := classifyError(err, provider)
	
	return &LLMError{
		Code:       code,
		Message:    message,
		Provider:   provider,
		Model:      model,
		HTTPStatus: httpStatus,
		Cause:      err,
		Retryable:  retryable,
		Timestamp:  time.Now(),
	}
}

// classifyError attempts to classify a provider-specific error
func classifyError(err error, provider providers.ProviderType) (ErrorCode, string, int, bool) {
	errStr := strings.ToLower(err.Error())
	
	// Common HTTP status-based classification
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") {
		return ErrorInvalidAPIKey, "Invalid API key or unauthorized", 401, false
	}
	
	if strings.Contains(errStr, "400") || strings.Contains(errStr, "bad request") {
		return ErrorInvalidRequest, "Bad request", 400, false
	}
	
	if strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden") {
		return ErrorInvalidAuth, "Forbidden access", 403, false
	}
	
	if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
		return ErrorInvalidModel, "Model or resource not found", 404, false
	}
	
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "quota") {
		return ErrorRateLimit, "Rate limit or quota exceeded", 429, true
	}
	
	if strings.Contains(errStr, "500") || strings.Contains(errStr, "internal server error") {
		return ErrorServerError, "Server error", 500, true
	}
	
	if strings.Contains(errStr, "502") || strings.Contains(errStr, "bad gateway") {
		return ErrorServerError, "Bad gateway", 502, true
	}
	
	if strings.Contains(errStr, "503") || strings.Contains(errStr, "service unavailable") {
		return ErrorServiceUnavailable, "Service unavailable", 503, true
	}
	
	if strings.Contains(errStr, "504") || strings.Contains(errStr, "timeout") {
		return ErrorTimeout, "Request timeout", 504, true
	}
	
	// Content-related errors
	if strings.Contains(errStr, "content filter") || strings.Contains(errStr, "safety") {
		return ErrorContentFiltered, "Content filtered for safety", 400, false
	}
	
	if strings.Contains(errStr, "token limit") || strings.Contains(errStr, "too long") {
		return ErrorTokenLimitExceeded, "Token limit exceeded", 400, false
	}
	
	// Network errors
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "network") {
		return ErrorNetworkError, "Network connection error", 0, true
	}
	
	if strings.Contains(errStr, "dns") {
		return ErrorDNSError, "DNS resolution error", 0, true
	}
	
	// Provider-specific error handling
	switch provider {
	case ProviderOpenAI:
		return classifyOpenAIError(errStr)
	case ProviderGemini:
		return classifyGeminiError(errStr)
	}
	
	// Default to unknown error
	return ErrorUnknown, err.Error(), 0, false
}

// classifyOpenAIError handles OpenAI-specific error classification
func classifyOpenAIError(errStr string) (ErrorCode, string, int, bool) {
	if strings.Contains(errStr, "insufficient_quota") {
		return ErrorQuotaExceeded, "OpenAI quota exceeded", 429, true
	}
	
	if strings.Contains(errStr, "model_not_found") {
		return ErrorInvalidModel, "OpenAI model not found", 404, false
	}
	
	if strings.Contains(errStr, "invalid_request_error") {
		return ErrorInvalidRequest, "OpenAI invalid request", 400, false
	}
	
	if strings.Contains(errStr, "rate_limit_exceeded") {
		return ErrorRateLimit, "OpenAI rate limit exceeded", 429, true
	}
	
	return ErrorUnknown, errStr, 0, false
}

// classifyGeminiError handles Gemini-specific error classification  
func classifyGeminiError(errStr string) (ErrorCode, string, int, bool) {
	if strings.Contains(errStr, "recitation") || strings.Contains(errStr, "blocked") {
		return ErrorContentFiltered, "Gemini content blocked", 400, false
	}
	
	if strings.Contains(errStr, "safety") {
		return ErrorSafetyViolation, "Gemini safety violation", 400, false
	}
	
	if strings.Contains(errStr, "resource_exhausted") {
		return ErrorQuotaExceeded, "Gemini resource exhausted", 429, true
	}
	
	if strings.Contains(errStr, "invalid_argument") {
		return ErrorInvalidParameters, "Gemini invalid argument", 400, false
	}
	
	return ErrorUnknown, errStr, 0, false
}

// isRetryableErrorCode determines if an error code is retryable
func isRetryableErrorCode(code ErrorCode) bool {
	retryableCodes := []ErrorCode{
		ErrorRateLimit,
		ErrorQuotaExceeded,
		ErrorTooManyRequests,
		ErrorServerError,
		ErrorServiceUnavailable,
		ErrorTimeout,
		ErrorNetworkError,
		ErrorConnectionFailed,
		ErrorDNSError,
	}
	
	for _, retryable := range retryableCodes {
		if code == retryable {
			return true
		}
	}
	return false
}

// HTTPStatusToErrorCode maps HTTP status codes to error codes
func HTTPStatusToErrorCode(status int) ErrorCode {
	switch status {
	case http.StatusBadRequest:
		return ErrorInvalidRequest
	case http.StatusUnauthorized:
		return ErrorInvalidAPIKey
	case http.StatusForbidden:
		return ErrorInvalidAuth
	case http.StatusNotFound:
		return ErrorInvalidModel
	case http.StatusTooManyRequests:
		return ErrorRateLimit
	case http.StatusInternalServerError:
		return ErrorServerError
	case http.StatusBadGateway:
		return ErrorServerError
	case http.StatusServiceUnavailable:
		return ErrorServiceUnavailable
	case http.StatusGatewayTimeout:
		return ErrorTimeout
	default:
		if status >= 500 {
			return ErrorServerError
		}
		return ErrorUnknown
	}
}

// Predefined error instances for common cases
var (
	ErrProviderNotFound   = NewLLMError(ErrorProviderNotFound, "Provider not found", "", nil)
	ErrProviderDisabled   = NewLLMError(ErrorProviderDisabled, "Provider is disabled", "", nil)
	ErrAllProvidersFailed = NewLLMError(ErrorAllProvidersFailed, "All providers failed", "", nil)
	ErrInvalidAPIKey      = NewLLMError(ErrorInvalidAPIKey, "Invalid API key", "", nil)
	ErrInvalidRequest     = NewLLMError(ErrorInvalidRequest, "Invalid request", "", nil)
	ErrRateLimit          = NewLLMError(ErrorRateLimit, "Rate limit exceeded", "", nil)
	ErrServerError        = NewLLMError(ErrorServerError, "Server error", "", nil)
	ErrTimeout            = NewLLMError(ErrorTimeout, "Request timeout", "", nil)
)

// ErrorMatcher provides utility functions for error matching
type ErrorMatcher struct{}

// IsTemporary checks if an error is temporary and should be retried
func (ErrorMatcher) IsTemporary(err error) bool {
	if llmErr, ok := err.(*LLMError); ok {
		return llmErr.IsRetryable()
	}
	return false
}

// IsAuthError checks if an error is authentication-related
func (ErrorMatcher) IsAuthError(err error) bool {
	if llmErr, ok := err.(*LLMError); ok {
		return llmErr.IsAuthError()
	}
	return false
}

// IsRateLimit checks if an error is rate limit-related
func (ErrorMatcher) IsRateLimit(err error) bool {
	if llmErr, ok := err.(*LLMError); ok {
		return llmErr.IsRateLimit()
	}
	return false
}

// IsContentError checks if an error is content-related
func (ErrorMatcher) IsContentError(err error) bool {
	if llmErr, ok := err.(*LLMError); ok {
		return llmErr.IsContentError()
	}
	return false
}

// Global error matcher instance
var Errors ErrorMatcher