package openai

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"gomini/pkg/gomini/providers"
)

// adaptChatRequest converts unified ChatRequest to OpenAI ChatCompletionNewParams
func (p *Provider) adaptChatRequest(req *providers.ChatRequest) (*openai.ChatCompletionNewParams, error) {
	// Convert messages
	openaiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages))
	
	for _, msg := range req.Messages {
		openaiMsg, err := p.adaptMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to adapt message: %w", err)
		}
		openaiMessages = append(openaiMessages, openaiMsg)
	}

	// Build OpenAI request
	params := &openai.ChatCompletionNewParams{
		Messages: openai.F(openaiMessages),
		Model:    openai.F(req.Model),
	}

	// Apply request configuration
	if err := p.applyRequestConfig(params, req.Config); err != nil {
		return nil, fmt.Errorf("failed to apply request config: %w", err)
	}

	// Add tools if present
	if len(req.Tools) > 0 {
		tools, err := p.adaptTools(req.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to adapt tools: %w", err)
		}
		params.Tools = openai.F(tools)
		
		if req.ToolChoice != nil {
			toolChoice, err := p.adaptToolChoice(req.ToolChoice)
			if err != nil {
				return nil, fmt.Errorf("failed to adapt tool choice: %w", err)
			}
			// params.ToolChoice = toolChoice // Need type assertion
			_ = toolChoice // Avoid unused variable error
		}
	}

	return params, nil
}

// adaptChatRequestForStream converts unified ChatRequest to streaming OpenAI request
func (p *Provider) adaptChatRequestForStream(req *providers.ChatRequest) (*openai.ChatCompletionNewParams, error) {
	params, err := p.adaptChatRequest(req)
	if err != nil {
		return nil, err
	}
	
	// Enable streaming
	// params.Stream = openai.F(true) // Stream parameter may not be available in this version
	
	return params, nil
}

// adaptJSONRequest converts JSONRequest to OpenAI structured output request
func (p *Provider) adaptJSONRequest(req *providers.ChatRequest, schema map[string]interface{}) (*openai.ChatCompletionNewParams, error) {
	params, err := p.adaptChatRequest(req)
	if err != nil {
		return nil, err
	}

	// Configure for JSON response mode
	params.ResponseFormat = openai.F[openai.ChatCompletionNewParamsResponseFormatUnion](
		openai.ResponseFormatJSONObjectParam{
			Type: openai.F(openai.ResponseFormatJSONObjectTypeJSONObject),
		},
	)

	// Also add a system message to ensure JSON output
	systemMsg := openai.SystemMessage("You must respond with valid JSON that matches the provided schema. Do not include any other text or formatting.")
	
	// Prepend the system message to existing messages
	existingMessages := params.Messages.Value
	allMessages := append([]openai.ChatCompletionMessageParamUnion{systemMsg}, existingMessages...)
	params.Messages = openai.F(allMessages)

	return params, nil
}

// adaptMessage converts unified Message to OpenAI message format
func (p *Provider) adaptMessage(msg providers.Message) (openai.ChatCompletionMessageParamUnion, error) {
	// This is a simplified version - in reality, we'd need to handle the actual Message type
	// For now, we'll assume Message has the necessary fields
	
	// This would need proper type assertion based on the actual Message interface
	// For demonstration purposes:
	switch msgType := msg.(type) {
	case map[string]interface{}:
		role := msgType["role"].(string)
		content := msgType["content"]
		
		switch role {
		case "system":
			return openai.SystemMessage(content.(string)), nil
		case "user":
			return openai.UserMessage(content.(string)), nil
		case "assistant":
			return openai.AssistantMessage(content.(string)), nil
		default:
			return nil, fmt.Errorf("unsupported message role: %s", role)
		}
	default:
		return nil, fmt.Errorf("unsupported message type: %T", msg)
	}
}

// adaptChatResponse converts OpenAI ChatCompletion to unified ChatResponse
func (p *Provider) adaptChatResponse(resp openai.ChatCompletion, model string) *providers.ChatResponse {
	choices := make([]providers.Choice, len(resp.Choices))
	
	for i, choice := range resp.Choices {
		choices[i] = p.adaptChoice(choice)
	}

	var usage *providers.Usage
	// Check if usage data is available (Usage is not a pointer in this SDK version)
	usage = &providers.Usage{
			InputTokens:      int(resp.Usage.PromptTokens),
			OutputTokens:     int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
		}

	return &providers.ChatResponse{
		ID:       resp.ID,
		Model:    model,
		Provider: providers.ProviderOpenAI,
		Choices:  choices,
		Usage:    usage,
		Created:  resp.Created,
	}
}

// adaptChoice converts OpenAI Choice to unified Choice
func (p *Provider) adaptChoice(choice openai.ChatCompletionChoice) providers.Choice {
	// This is a placeholder - would need proper Choice type definition
	return map[string]interface{}{
		"index":         choice.Index,
		"message":       p.adaptAssistantMessage(choice.Message),
		"finish_reason": p.adaptFinishReason(choice.FinishReason),
	}
}

// adaptAssistantMessage converts OpenAI assistant message to unified format
func (p *Provider) adaptAssistantMessage(msg openai.ChatCompletionMessage) interface{} {
	// Placeholder implementation
	return map[string]interface{}{
		"role":    "assistant",
		"content": msg.Content,
		// Handle tool calls, function calls, etc.
	}
}

// adaptFinishReason converts OpenAI finish reason to unified format
func (p *Provider) adaptFinishReason(reason openai.ChatCompletionChoicesFinishReason) providers.FinishReason {
	switch reason {
	case openai.ChatCompletionChoicesFinishReasonStop:
		return providers.FinishReasonStop
	case openai.ChatCompletionChoicesFinishReasonLength:
		return providers.FinishReasonLength
	case openai.ChatCompletionChoicesFinishReasonFunctionCall:
		return providers.FinishReasonFunctionCall
	case openai.ChatCompletionChoicesFinishReasonToolCalls:
		return providers.FinishReasonToolCalls
	case openai.ChatCompletionChoicesFinishReasonContentFilter:
		return providers.FinishReasonContentFilter
	default:
		return providers.FinishReasonError
	}
}

// adaptStreamChunk converts OpenAI streaming chunk to unified StreamEvent
func (p *Provider) adaptStreamChunk(chunk openai.ChatCompletionChunk, model string) *providers.StreamEvent {
	if len(chunk.Choices) == 0 {
		return nil
	}

	choice := chunk.Choices[0]
	
	// Handle content delta
	if choice.Delta.Content != "" {
		return &providers.StreamEvent{
			Type:     providers.EventContent,
			Provider: providers.ProviderOpenAI,
			Model:    model,
			Data: providers.ContentEvent{
				Text:  choice.Delta.Content,
				Delta: true,
			},
			Timestamp: time.Now(),
		}
	}

	// Handle finish reason
	if choice.FinishReason != "" {
		finishReason := p.adaptFinishReason(openai.ChatCompletionChoicesFinishReason(choice.FinishReason))
		return &providers.StreamEvent{
			Type:     providers.EventFinished,
			Provider: providers.ProviderOpenAI,
			Model:    model,
			Metadata: providers.EventMeta{
				FinishReason: finishReason,
			},
			Timestamp: time.Now(),
		}
	}

	// Handle tool calls
	if len(choice.Delta.ToolCalls) > 0 {
		// Convert tool calls to events
		// This would need more detailed implementation
		return &providers.StreamEvent{
			Type:      providers.EventToolCall,
			Provider:  providers.ProviderOpenAI,
			Model:     model,
			Timestamp: time.Now(),
			// Tool call data would go here
		}
	}

	return nil
}

// adaptJSONResponse converts OpenAI response to unified JSONResponse
func (p *Provider) adaptJSONResponse(resp openai.ChatCompletion, model string, schema map[string]interface{}) (*providers.JSONResponse, error) {
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := resp.Choices[0].Message.Content
	if content == "" {
		return nil, fmt.Errorf("empty content in response")
	}

	// Extract JSON from markdown code blocks if present
	jsonContent := p.extractJSONFromMarkdown(content)

	// Parse JSON content
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonContent), &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	var usage *providers.Usage
	// Check if usage data is available (Usage is not a pointer in this SDK version)
	usage = &providers.Usage{
			InputTokens:      int(resp.Usage.PromptTokens),
			OutputTokens:     int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
		}

	return &providers.JSONResponse{
		ID:       resp.ID,
		Model:    model,
		Provider: providers.ProviderOpenAI,
		Data:     jsonData,
		Usage:    usage,
		Created:  resp.Created,
	}, nil
}

// adaptModel converts OpenAI Model to unified Model
func (p *Provider) adaptModel(model openai.Model) providers.Model {
	// Determine capabilities based on model ID
	capabilities := providers.ModelCapabilities{
		TextGeneration: true,
		SystemMessage:  true,
		Streaming:      true,
	}

	// Set capabilities based on model name
	if contains(model.ID, "gpt-4") {
		capabilities.FunctionCalling = true
		capabilities.JSONMode = true
		capabilities.StructuredOutput = true
		
		if contains(model.ID, "vision") || model.ID == "gpt-4o" || model.ID == "gpt-4o-mini" {
			capabilities.ImageInput = true
		}
	} else if contains(model.ID, "gpt-3.5") {
		capabilities.FunctionCalling = true
		capabilities.JSONMode = true
	}

	// Estimate context size
	contextSize := 4096 // Default
	if contains(model.ID, "gpt-4o") {
		contextSize = 128000
	} else if contains(model.ID, "gpt-4-turbo") {
		contextSize = 128000
	} else if contains(model.ID, "16k") {
		contextSize = 16384
	} else if contains(model.ID, "32k") {
		contextSize = 32768
	}

	return providers.Model{
		ID:           model.ID,
		Name:         model.ID, // OpenAI uses ID as name
		Provider:     providers.ProviderOpenAI,
		Capabilities: capabilities,
		ContextSize:  contextSize,
	}
}

// Helper functions

func (p *Provider) applyRequestConfig(params *openai.ChatCompletionNewParams, config providers.RequestConfig) error {
	// This is a placeholder - would need proper RequestConfig type handling
	if configMap, ok := config.(map[string]interface{}); ok {
		if temp, exists := configMap["temperature"]; exists {
			if tempFloat, ok := temp.(float64); ok {
				params.Temperature = openai.F(tempFloat)
			}
		}
		
		if topP, exists := configMap["top_p"]; exists {
			if topPFloat, ok := topP.(float64); ok {
				params.TopP = openai.F(topPFloat)
			}
		}
		
		if maxTokens, exists := configMap["max_tokens"]; exists {
			if maxTokensInt, ok := maxTokens.(int); ok {
				params.MaxTokens = openai.F(int64(maxTokensInt))
			}
		}
		
		if stop, exists := configMap["stop"]; exists {
			if stopSlice, ok := stop.([]string); ok {
				// params.Stop = openai.F(stopSlice) // May need different API
				_ = stopSlice // Avoid unused variable error
			}
		}
	}
	
	return nil
}

func (p *Provider) adaptTools(tools []providers.Tool) ([]openai.ChatCompletionToolParam, error) {
	openaiTools := make([]openai.ChatCompletionToolParam, len(tools))
	
	for i, tool := range tools {
		// Convert unified tool to OpenAI format
		// This would need proper Tool type handling
		_ = tool // Avoid unused variable
		openaiTools[i] = openai.ChatCompletionToolParam{
			Type: openai.F(openai.ChatCompletionToolTypeFunction),
			Function: openai.F(openai.FunctionDefinitionParam{
				Name: openai.F("placeholder"), // Would extract from tool
				// Add other function parameters
			}),
		}
	}
	
	return openaiTools, nil
}

func (p *Provider) adaptToolChoice(choice interface{}) (interface{}, error) {
	// Handle different tool choice types
	switch v := choice.(type) {
	case string:
		switch v {
		case "auto":
			return "auto", nil
		case "none":
			return "none", nil
		case "required":
			return "required", nil
		default:
			return nil, fmt.Errorf("unsupported tool choice string: %s", v)
		}
	default:
		return nil, fmt.Errorf("unsupported tool choice type: %T", choice)
	}
}

// extractJSONFromMarkdown extracts JSON content from markdown code blocks
func (p *Provider) extractJSONFromMarkdown(content string) string {
	// Check if content is wrapped in markdown code blocks
	if len(content) > 6 && content[:3] == "```" {
		// Find the end of the opening block
		start := 3
		if len(content) > 7 && content[3:7] == "json" {
			start = 7
		}
		// Skip any whitespace after the opening block
		for start < len(content) && (content[start] == '\n' || content[start] == '\r' || content[start] == ' ' || content[start] == '\t') {
			start++
		}
		
		// Find the closing ```
		end := len(content)
		if closingIdx := findClosingCodeBlock(content, start); closingIdx != -1 {
			end = closingIdx
			// Trim any trailing whitespace before the closing ```
			for end > start && (content[end-1] == '\n' || content[end-1] == '\r' || content[end-1] == ' ' || content[end-1] == '\t') {
				end--
			}
		}
		
		// Extract the JSON content
		extracted := content[start:end]
		return extracted
	}
	
	// If not wrapped in code blocks, return as-is
	return content
}

// findClosingCodeBlock finds the index of the closing ``` block
func findClosingCodeBlock(content string, start int) int {
	for i := start; i < len(content)-2; i++ {
		if content[i:i+3] == "```" {
			return i
		}
	}
	return -1
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != substr && (len(s) == len(substr) || 
		s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		(len(s) > len(substr) && s[1:len(substr)+1] == substr))
}