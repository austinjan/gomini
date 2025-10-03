package gomini

import (
	"time"
	
	"gomini/pkg/gomini/providers"
)

// EventType defines the type of streaming event
type EventType string

const (
	// Content events
	EventContent  EventType = "content"  // Text content chunk
	EventThought  EventType = "thought"  // Thinking content (Gemini)
	EventCitation EventType = "citation" // Source citation
	
	// Tool/Function calling events
	EventToolCall     EventType = "tool_call"     // Assistant wants to call a tool
	EventToolResponse EventType = "tool_response" // Tool call response
	EventToolConfirm  EventType = "tool_confirm"  // Tool call needs confirmation
	
	// Control events
	EventFinished       EventType = "finished"        // Generation completed
	EventError          EventType = "error"           // An error occurred
	EventRetry          EventType = "retry"           // Retrying request
	EventProviderSwitch EventType = "provider_switch" // Switched to different provider
	EventRateLimit      EventType = "rate_limit"      // Hit rate limit
	EventCancel         EventType = "cancel"          // Request was cancelled
	
	// Loop detection and session management events
	EventLoopDetected     EventType = "loop_detected"     // Loop detected in conversation
	EventMaxSessionTurns  EventType = "max_session_turns" // Session turn limit reached
	EventChatCompressed   EventType = "chat_compressed"   // Chat history was compressed
	
	// Meta events
	EventUsage    EventType = "usage"    // Token usage information
	EventMetadata EventType = "metadata" // Additional metadata
	EventDebug    EventType = "debug"    // Debug information
)

// StreamEvent represents a single event in the streaming response
type StreamEvent struct {
	Type      EventType    `json:"type"`
	Provider  providers.ProviderType `json:"provider"`
	Model     string       `json:"model,omitempty"`
	Data      interface{}  `json:"data,omitempty"`
	Error     error        `json:"error,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	RequestID string       `json:"request_id,omitempty"`
	Metadata  EventMeta    `json:"metadata,omitempty"`
}

// EventMeta contains metadata about the event
type EventMeta struct {
	ChoiceIndex    int               `json:"choice_index,omitempty"`
	FinishReason   providers.FinishReason      `json:"finish_reason,omitempty"`
	Usage          *providers.Usage            `json:"usage,omitempty"`
	ExtraData      map[string]interface{} `json:"extra_data,omitempty"`
}

// ContentEvent represents text content data
type ContentEvent struct {
	Text     string `json:"text"`
	Delta    bool   `json:"delta"`    // True if this is a delta (partial) update
	Complete bool   `json:"complete"` // True if this completes the content
}

// ThoughtEvent represents thinking content (Gemini-specific)
type ThoughtEvent struct {
	Subject     string `json:"subject"`
	Description string `json:"description"`
	Text        string `json:"text,omitempty"` // Raw thought text
}

// CitationEvent represents source citations
type CitationEvent struct {
	Sources []Citation `json:"sources"`
}

// Citation represents a single citation
type Citation struct {
	Title string `json:"title,omitempty"`
	URI   string `json:"uri"`
	Index int    `json:"index,omitempty"`
}

// ToolCallEvent represents a tool/function call request
type ToolCallEvent struct {
	CallID     string                 `json:"call_id"`
	ToolName   string                 `json:"tool_name"`
	Arguments  map[string]interface{} `json:"arguments"`
	Reasoning  string                 `json:"reasoning,omitempty"`  // Why the tool was called
	Confidence float64                `json:"confidence,omitempty"` // Confidence in the call
}

// ToolResponseEvent represents the response from a tool call
type ToolResponseEvent struct {
	CallID    string      `json:"call_id"`
	ToolName  string      `json:"tool_name"`
	Result    interface{} `json:"result"`
	Success   bool        `json:"success"`
	Duration  time.Duration `json:"duration,omitempty"`
	Cached    bool        `json:"cached,omitempty"` // If result was cached
}

// ToolConfirmEvent represents a tool call that needs user confirmation
type ToolConfirmEvent struct {
	CallID      string                 `json:"call_id"`
	ToolName    string                 `json:"tool_name"`
	Arguments   map[string]interface{} `json:"arguments"`
	Description string                 `json:"description"`     // Human-readable description
	Risk        string                 `json:"risk,omitempty"`   // Risk level: low, medium, high
	Impact      string                 `json:"impact,omitempty"` // Impact description
}

// ErrorEvent represents error information
type ErrorEvent struct {
	Code       string                 `json:"code,omitempty"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Retryable  bool                   `json:"retryable"`
	RetryAfter *time.Duration         `json:"retry_after,omitempty"`
}

// RetryEvent represents a retry attempt
type RetryEvent struct {
	Attempt      int           `json:"attempt"`
	MaxAttempts  int           `json:"max_attempts"`
	Delay        time.Duration `json:"delay"`
	Reason       string        `json:"reason"`
	NextProvider providers.ProviderType  `json:"next_provider,omitempty"`
}

// ProviderSwitchEvent represents switching to a different provider
type ProviderSwitchEvent struct {
	FromProvider providers.ProviderType `json:"from_provider"`
	ToProvider   providers.ProviderType `json:"to_provider"`
	Reason       string       `json:"reason"`
	Automatic    bool         `json:"automatic"` // True if switch was automatic
}

// RateLimitEvent represents hitting a rate limit
type RateLimitEvent struct {
	Provider   providers.ProviderType  `json:"provider"`
	ResetAt    time.Time     `json:"reset_at,omitempty"`
	ResetAfter time.Duration `json:"reset_after,omitempty"`
	Remaining  int           `json:"remaining,omitempty"`
	Limit      int           `json:"limit,omitempty"`
}

// UsageEvent represents token usage information
type UsageEvent struct {
	Usage       *providers.Usage  `json:"usage"`
	Cost        float64 `json:"cost,omitempty"`        // Estimated cost in USD
	Efficiency  float64 `json:"efficiency,omitempty"`  // Tokens per second
	Cumulative  *providers.Usage  `json:"cumulative,omitempty"`  // Session cumulative usage
}

// DebugEvent represents debug information
type DebugEvent struct {
	Level   string                 `json:"level"`   // debug, info, warn, error
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// LoopType defines the type of loop detected
type LoopType string

const (
	LoopTypeToolCall    LoopType = "tool_call"    // Consecutive identical tool calls
	LoopTypeContent     LoopType = "content"      // Repetitive content patterns
	LoopTypeLLMDetected LoopType = "llm_detected" // LLM-detected cognitive loop
)

// LoopDetectedEvent represents a detected conversation loop
type LoopDetectedEvent struct {
	LoopType     LoopType `json:"loop_type"`
	PromptID     string   `json:"prompt_id"`
	Description  string   `json:"description,omitempty"`
	TurnCount    int      `json:"turn_count,omitempty"`    // Number of turns when detected
	RepeatCount  int      `json:"repeat_count,omitempty"`  // How many repetitions detected
}

// MaxSessionTurnsEvent represents reaching session turn limits
type MaxSessionTurnsEvent struct {
	CurrentTurns int    `json:"current_turns"`
	MaxTurns     int    `json:"max_turns"`
	PromptID     string `json:"prompt_id"`
}

// ChatCompressedEvent represents chat history compression (future use)
type ChatCompressedEvent struct {
	OriginalTokens int     `json:"original_tokens"`
	NewTokens      int     `json:"new_tokens"`
	CompressionRatio float64 `json:"compression_ratio"`
	PromptID       string  `json:"prompt_id"`
}

// Helper functions for creating events

// NewContentEvent creates a content event
func NewContentEvent(provider providers.ProviderType, model, text string, delta bool) StreamEvent {
	return StreamEvent{
		Type:      EventContent,
		Provider:  provider,
		Model:     model,
		Data:      ContentEvent{Text: text, Delta: delta},
		Timestamp: time.Now(),
	}
}

// NewThoughtEvent creates a thought event
func NewThoughtEvent(provider providers.ProviderType, model, subject, description string) StreamEvent {
	return StreamEvent{
		Type:      EventThought,
		Provider:  provider,
		Model:     model,
		Data:      ThoughtEvent{Subject: subject, Description: description},
		Timestamp: time.Now(),
	}
}

// NewToolCallEvent creates a tool call event
func NewToolCallEvent(provider providers.ProviderType, model, callID, toolName string, args map[string]interface{}) StreamEvent {
	return StreamEvent{
		Type:     EventToolCall,
		Provider: provider,
		Model:    model,
		Data: ToolCallEvent{
			CallID:    callID,
			ToolName:  toolName,
			Arguments: args,
		},
		Timestamp: time.Now(),
	}
}

// NewErrorEvent creates an error event
func NewErrorEvent(provider providers.ProviderType, model string, err error, retryable bool) StreamEvent {
	return StreamEvent{
		Type:     EventError,
		Provider: provider,
		Model:    model,
		Error:    err,
		Data: ErrorEvent{
			Message:   err.Error(),
			Retryable: retryable,
		},
		Timestamp: time.Now(),
	}
}

// NewFinishedEvent creates a finished event
func NewFinishedEvent(provider providers.ProviderType, model string, reason providers.FinishReason, usage *providers.Usage) StreamEvent {
	return StreamEvent{
		Type:     EventFinished,
		Provider: provider,
		Model:    model,
		Data:     nil,
		Metadata: EventMeta{
			FinishReason: reason,
			Usage:        usage,
		},
		Timestamp: time.Now(),
	}
}

// NewProviderSwitchEvent creates a provider switch event
func NewProviderSwitchEvent(from, to providers.ProviderType, reason string, automatic bool) StreamEvent {
	return StreamEvent{
		Type: EventProviderSwitch,
		Data: ProviderSwitchEvent{
			FromProvider: from,
			ToProvider:   to,
			Reason:       reason,
			Automatic:    automatic,
		},
		Timestamp: time.Now(),
	}
}

// NewUsageEvent creates a usage event
func NewUsageEvent(provider providers.ProviderType, model string, usage *providers.Usage, cost float64) StreamEvent {
	return StreamEvent{
		Type:     EventUsage,
		Provider: provider,
		Model:    model,
		Data: UsageEvent{
			Usage: usage,
			Cost:  cost,
		},
		Timestamp: time.Now(),
	}
}

// NewDebugEvent creates a debug event
func NewDebugEvent(provider providers.ProviderType, level, message string, data map[string]interface{}) StreamEvent {
	return StreamEvent{
		Type:     EventDebug,
		Provider: provider,
		Data: DebugEvent{
			Level:   level,
			Message: message,
			Data:    data,
		},
		Timestamp: time.Now(),
	}
}

// NewLoopDetectedEvent creates a loop detected event
func NewLoopDetectedEvent(provider providers.ProviderType, model string, loopType LoopType, promptID string, description string, turnCount, repeatCount int) StreamEvent {
	return StreamEvent{
		Type:     EventLoopDetected,
		Provider: provider,
		Model:    model,
		Data: LoopDetectedEvent{
			LoopType:     loopType,
			PromptID:     promptID,
			Description:  description,
			TurnCount:    turnCount,
			RepeatCount:  repeatCount,
		},
		Timestamp: time.Now(),
	}
}

// NewMaxSessionTurnsEvent creates a max session turns event
func NewMaxSessionTurnsEvent(provider providers.ProviderType, model string, currentTurns, maxTurns int, promptID string) StreamEvent {
	return StreamEvent{
		Type:     EventMaxSessionTurns,
		Provider: provider,
		Model:    model,
		Data: MaxSessionTurnsEvent{
			CurrentTurns: currentTurns,
			MaxTurns:     maxTurns,
			PromptID:     promptID,
		},
		Timestamp: time.Now(),
	}
}

// NewChatCompressedEvent creates a chat compressed event
func NewChatCompressedEvent(provider providers.ProviderType, model string, originalTokens, newTokens int, promptID string) StreamEvent {
	compressionRatio := 0.0
	if originalTokens > 0 {
		compressionRatio = float64(newTokens) / float64(originalTokens)
	}
	
	return StreamEvent{
		Type:     EventChatCompressed,
		Provider: provider,
		Model:    model,
		Data: ChatCompressedEvent{
			OriginalTokens:   originalTokens,
			NewTokens:        newTokens,
			CompressionRatio: compressionRatio,
			PromptID:         promptID,
		},
		Timestamp: time.Now(),
	}
}