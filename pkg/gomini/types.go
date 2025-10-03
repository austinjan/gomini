package gomini

import (
	"gomini/pkg/gomini/providers"
)

// Type aliases for compatibility with providers package
// This allows the main package to use the same types as providers
type (
	// Core types from providers package
	Message = providers.Message
	RequestConfig = providers.RequestConfig  
	Tool = providers.Tool
	Choice = providers.Choice
	ProviderType = providers.ProviderType
	
	// Request/Response types
	ChatRequest = providers.ChatRequest
	ChatResponse = providers.ChatResponse
	JSONRequest = providers.JSONRequest
	JSONResponse = providers.JSONResponse
	// StreamEvent = providers.StreamEvent // Defined in events.go
	
	// Model and capability types
	Model = providers.Model
	ModelCapabilities = providers.ModelCapabilities
	ProviderCapabilities = providers.ProviderCapabilities
	
	// Safety and configuration types
	SafetySetting = providers.SafetySetting
	Usage = providers.Usage
	FinishReason = providers.FinishReason
	
	// Event types (some defined in events.go)
	// EventMeta = providers.EventMeta // Defined in events.go
)

// Provider constants for convenience
const (
	ProviderOpenAI = providers.ProviderOpenAI
	ProviderGemini = providers.ProviderGemini
)

// Additional helper types specific to main package can be defined here
// For now, we rely on the providers package types for foundational functionality

// Helper functions for creating messages and content
func NewUserMessage(content string) Message {
	return map[string]interface{}{
		"role":    "user",
		"content": content,
	}
}

func NewSystemMessage(content string) Message {
	return map[string]interface{}{
		"role":    "system", 
		"content": content,
	}
}

func NewAssistantMessage(content string) Message {
	return map[string]interface{}{
		"role":    "assistant",
		"content": content,
	}
}