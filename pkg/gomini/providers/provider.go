package providers

import (
	"context"
	"fmt"
	"time"
)

// ProviderType identifies the LLM provider
type ProviderType string

const (
	ProviderOpenAI ProviderType = "openai"
	ProviderGemini ProviderType = "gemini"
)

// LLMProvider defines the unified interface for all LLM providers
type LLMProvider interface {
	// SendMessage sends a chat completion request and returns the full response
	SendMessage(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
	
	// SendMessageStream sends a chat completion request and returns a stream of events
	SendMessageStream(ctx context.Context, req *ChatRequest) <-chan StreamEvent
	
	// GenerateJSON generates structured JSON output based on a schema
	GenerateJSON(ctx context.Context, req *JSONRequest) (*JSONResponse, error)
	
	// ListModels returns available models for this provider
	ListModels(ctx context.Context) ([]Model, error)
	
	// GetCapabilities returns the capabilities of this provider
	GetCapabilities() ProviderCapabilities
	
	// GetProviderType returns the provider type
	GetProviderType() ProviderType
	
	// Close closes the provider and cleans up resources
	Close() error
}

// Model represents an available model
type Model struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Provider     ProviderType      `json:"provider"`
	Capabilities ModelCapabilities `json:"capabilities"`
	ContextSize  int               `json:"context_size"`
	Cost         *ModelCost        `json:"cost,omitempty"`
}

// ModelCapabilities defines what a model can do
type ModelCapabilities struct {
	TextGeneration   bool `json:"text_generation"`
	ImageInput       bool `json:"image_input"`
	ImageGeneration  bool `json:"image_generation"`
	FunctionCalling  bool `json:"function_calling"`
	JSONMode         bool `json:"json_mode"`
	SystemMessage    bool `json:"system_message"`
	Streaming        bool `json:"streaming"`
	ThinkingMode     bool `json:"thinking_mode,omitempty"`     // Gemini-specific
	StructuredOutput bool `json:"structured_output,omitempty"` // OpenAI-specific
}

// ModelCost represents the cost structure for a model
type ModelCost struct {
	InputTokens  float64 `json:"input_tokens"`  // Cost per 1K input tokens
	OutputTokens float64 `json:"output_tokens"` // Cost per 1K output tokens
	Currency     string  `json:"currency"`      // USD, etc.
}

// ProviderCapabilities defines what a provider supports
type ProviderCapabilities struct {
	Models              []string          `json:"models"`
	MaxContextSize      int               `json:"max_context_size"`
	SupportedMimeTypes  []string          `json:"supported_mime_types"`
	SupportsStreaming   bool              `json:"supports_streaming"`
	SupportsVision      bool              `json:"supports_vision"`
	SupportsFunctions   bool              `json:"supports_functions"`
	SupportsJSONMode    bool              `json:"supports_json_mode"`
	RateLimit           *RateLimit        `json:"rate_limit,omitempty"`
	SpecificFeatures    map[string]string `json:"specific_features,omitempty"`
}

// RateLimit defines the rate limiting for a provider
type RateLimit struct {
	RequestsPerMinute int           `json:"requests_per_minute"`
	RequestsPerDay    int           `json:"requests_per_day"`
	TokensPerMinute   int           `json:"tokens_per_minute"`
	ResetWindow       time.Duration `json:"reset_window"`
}

// Usage represents token usage statistics
type Usage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	TotalTokens      int `json:"total_tokens"`
	CompletionTokens int `json:"completion_tokens,omitempty"` // OpenAI terminology
	PromptTokens     int `json:"prompt_tokens,omitempty"`     // OpenAI terminology
}

// FinishReason indicates why generation stopped
type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonFunctionCall  FinishReason = "function_call"
	FinishReasonToolCalls     FinishReason = "tool_calls"
	FinishReasonContentFilter FinishReason = "content_filter"
	FinishReasonError         FinishReason = "error"
)

// Core message and content types

type Message interface{}

type RequestConfig interface{}

type Tool interface{}

type Choice interface{}

// Common types that providers need to work with

type ChatRequest struct {
	Messages    []Message     `json:"messages"`
	Model       string        `json:"model"`
	Provider    ProviderType  `json:"provider,omitempty"`
	Config      RequestConfig `json:"config,omitempty"`
	Tools       []Tool        `json:"tools,omitempty"`
	ToolChoice  interface{}   `json:"tool_choice,omitempty"`
}

type ChatResponse struct {
	ID       string       `json:"id"`
	Model    string       `json:"model"`
	Provider ProviderType `json:"provider"`
	Choices  []Choice     `json:"choices"`
	Usage    *Usage       `json:"usage,omitempty"`
	Created  int64        `json:"created,omitempty"`
}

type JSONRequest struct {
	Messages []Message              `json:"messages"`
	Model    string                 `json:"model"`
	Provider ProviderType           `json:"provider,omitempty"`
	Schema   map[string]interface{} `json:"schema"`
	Config   RequestConfig          `json:"config,omitempty"`
}

type JSONResponse struct {
	ID       string                 `json:"id"`
	Model    string                 `json:"model"`
	Provider ProviderType           `json:"provider"`
	Data     map[string]interface{} `json:"data"`
	Usage    *Usage                 `json:"usage,omitempty"`
	Created  int64                  `json:"created,omitempty"`
}

// Forward declarations and helper functions

// NewLLMError creates a new LLMError (to be implemented in errors.go)
func NewLLMError(code string, message string, provider ProviderType, cause error) error {
	// Placeholder implementation
	return fmt.Errorf("[%s:%s] %s", provider, code, message)
}

// WrapProviderError wraps a provider error (to be implemented in errors.go)  
func WrapProviderError(err error, provider ProviderType, model string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("[%s:%s] %w", provider, model, err)
}

// Event types and helper functions
type EventType string

const (
	EventContent        EventType = "content"
	EventThought        EventType = "thought"
	EventToolCall       EventType = "tool_call"
	EventFinished       EventType = "finished"
	EventError          EventType = "error"
	EventProviderSwitch EventType = "provider_switch"
)

type StreamEvent struct {
	Type      EventType   `json:"type"`
	Provider  ProviderType `json:"provider"`
	Model     string      `json:"model,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Error     error       `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	RequestID string      `json:"request_id,omitempty"`
	Metadata  EventMeta   `json:"metadata,omitempty"`
}

type EventMeta struct {
	FinishReason FinishReason `json:"finish_reason,omitempty"`
	Usage        *Usage       `json:"usage,omitempty"`
}

type ContentEvent struct {
	Text     string `json:"text"`
	Delta    bool   `json:"delta"`
	Complete bool   `json:"complete"`
}

type ThoughtEvent struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Text        string `json:"text,omitempty"`
}

type SafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// Helper functions for creating events
func NewErrorEvent(provider ProviderType, model string, err error, retryable bool) StreamEvent {
	return StreamEvent{
		Type:      EventError,
		Provider:  provider,
		Model:     model,
		Error:     err,
		Timestamp: time.Now(),
	}
}

func NewContentEvent(provider ProviderType, model, text string, delta bool) StreamEvent {
	return StreamEvent{
		Type:      EventContent,
		Provider:  provider,
		Model:     model,
		Data:      ContentEvent{Text: text, Delta: delta},
		Timestamp: time.Now(),
	}
}

// Error codes (to match main errors.go)
const (
	ErrorInvalidAPIKey  = "invalid_api_key"
	ErrorInvalidAuth    = "invalid_auth" 
	ErrorInvalidRequest = "invalid_request"
	ErrorProviderNotFound = "provider_not_found"
)