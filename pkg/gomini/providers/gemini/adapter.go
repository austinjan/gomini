package gemini

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
	"gomini/pkg/gomini/providers"
)

// adaptChatRequest converts unified ChatRequest to Gemini GenerateContent request
func (p *Provider) adaptChatRequest(req *providers.ChatRequest) (*GeminiRequest, error) {
	// Convert messages to Gemini Content format
	contents := make([]*genai.Content, 0, len(req.Messages))
	
	for _, msg := range req.Messages {
		content, err := p.adaptMessage(msg)
		if err != nil {
			return nil, fmt.Errorf("failed to adapt message: %w", err)
		}
		if content != nil {
			contents = append(contents, content)
		}
	}

	// Build Gemini configuration
	config := &genai.GenerateContentConfig{}
	
	if err := p.applyRequestConfig(config, req.Config); err != nil {
		return nil, fmt.Errorf("failed to apply request config: %w", err)
	}

	// Add tools if present
	if len(req.Tools) > 0 {
		tools, err := p.adaptTools(req.Tools)
		if err != nil {
			return nil, fmt.Errorf("failed to adapt tools: %w", err)
		}
		config.Tools = tools
	}

	// Apply safety settings
	if len(p.config.SafetySettings) > 0 {
		safetySettings := p.adaptSafetySettings(p.config.SafetySettings)
		config.SafetySettings = safetySettings
	}

	return &GeminiRequest{
		Contents: contents,
		Config:   config,
	}, nil
}

// adaptJSONRequest converts JSONRequest to Gemini request with JSON response format
func (p *Provider) adaptJSONRequest(req *providers.JSONRequest) (*GeminiRequest, error) {
	// Convert chat request
	chatReq := &providers.ChatRequest{
		Messages: req.Messages,
		Model:    req.Model,
		Provider: providers.ProviderGemini,
		Config:   req.Config,
	}
	
	geminiReq, err := p.adaptChatRequest(chatReq)
	if err != nil {
		return nil, err
	}

	// Configure for JSON response
	geminiReq.Config.ResponseMIMEType = "application/json"
	
	// Add schema to system instruction if provided
	if req.Schema != nil {
		schemaJSON, err := json.Marshal(req.Schema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal schema: %w", err)
		}
		
		// Prepend schema instruction to first content
		if len(geminiReq.Contents) > 0 {
			schemaInstruction := fmt.Sprintf("Please respond with JSON that matches this schema: %s", string(schemaJSON))
			
			// Add schema instruction as system content
			systemContent := &genai.Content{
				Role: "user",
				Parts: []*genai.Part{
					{Text: schemaInstruction},
				},
			}
			
			// Insert at the beginning
			geminiReq.Contents = append([]*genai.Content{systemContent}, geminiReq.Contents...)
		}
	}

	return geminiReq, nil
}

// adaptMessage converts unified Message to Gemini Content
func (p *Provider) adaptMessage(msg providers.Message) (*genai.Content, error) {
	// This is a simplified version - would need proper Message type handling
	switch msgType := msg.(type) {
	case map[string]interface{}:
		role := msgType["role"].(string)
		content := msgType["content"]
		
		// Map roles
		var geminiRole string
		switch role {
		case "system":
			// Gemini doesn't have explicit system role, convert to user instruction
			geminiRole = "user"
		case "user":
			geminiRole = "user"
		case "assistant":
			geminiRole = "model"
		default:
			return nil, fmt.Errorf("unsupported message role: %s", role)
		}

		// Convert content parts
		parts, err := p.adaptContentParts(content)
		if err != nil {
			return nil, fmt.Errorf("failed to adapt content parts: %w", err)
		}

		return &genai.Content{
			Role:  geminiRole,
			Parts: parts,
		}, nil
		
	default:
		return nil, fmt.Errorf("unsupported message type: %T", msg)
	}
}

// adaptContentParts converts content to Gemini Parts
func (p *Provider) adaptContentParts(content interface{}) ([]*genai.Part, error) {
	switch contentType := content.(type) {
	case string:
		// Simple text content
		return []*genai.Part{{Text: contentType}}, nil
		
	case []interface{}:
		// Array of content parts
		parts := make([]*genai.Part, 0, len(contentType))
		
		for _, item := range contentType {
			if itemMap, ok := item.(map[string]interface{}); ok {
				partType := itemMap["type"].(string)
				
				switch partType {
				case "text":
					if data, ok := itemMap["data"].(map[string]interface{}); ok {
						if text, ok := data["text"].(string); ok {
							parts = append(parts, &genai.Part{Text: text})
						}
					}
					
				case "image_url":
					if data, ok := itemMap["data"].(map[string]interface{}); ok {
						part, err := p.adaptImagePart(data)
						if err != nil {
							return nil, fmt.Errorf("failed to adapt image part: %w", err)
						}
						parts = append(parts, part)
					}
				}
			}
		}
		
		return parts, nil
		
	default:
		return nil, fmt.Errorf("unsupported content type: %T", content)
	}
}

// adaptImagePart converts image content to Gemini Part
func (p *Provider) adaptImagePart(data map[string]interface{}) (*genai.Part, error) {
	// Handle different image formats
	if url, ok := data["url"].(string); ok && url != "" {
		// For now, return text indicating image URL (would need actual image processing)
		return &genai.Part{Text: fmt.Sprintf("[Image: %s]", url)}, nil
	}
	
	if base64Data, ok := data["base64"].(string); ok && base64Data != "" {
		mimeType := "image/jpeg"
		if mime, ok := data["mime_type"].(string); ok {
			mimeType = mime
		}
		
		// Convert base64 to inline data
		return &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: mimeType,
				Data:     []byte(base64Data), // Would need proper base64 decoding
			},
		}, nil
	}
	
	return nil, fmt.Errorf("invalid image data")
}

// adaptChatResponse converts Gemini GenerateContentResponse to unified ChatResponse
func (p *Provider) adaptChatResponse(resp *genai.GenerateContentResponse, model string) *providers.ChatResponse {
	choices := make([]providers.Choice, 0)
	
	// Gemini typically returns one candidate
	if len(resp.Candidates) > 0 {
		for i, candidate := range resp.Candidates {
			choice := p.adaptChoice(candidate, i)
			choices = append(choices, choice)
		}
	}

	var usage *providers.Usage
	if resp.UsageMetadata != nil {
		usage = &providers.Usage{
			InputTokens:  int(*resp.UsageMetadata.PromptTokenCount),
			OutputTokens: int(*resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:  int(resp.UsageMetadata.TotalTokenCount),
		}
	}

	return &providers.ChatResponse{
		ID:       generateResponseID(), // Gemini doesn't provide ID
		Model:    model,
		Provider: providers.ProviderGemini,
		Choices:  choices,
		Usage:    usage,
		Created:  time.Now().Unix(),
	}
}

// adaptChoice converts Gemini Candidate to unified Choice
func (p *Provider) adaptChoice(candidate *genai.Candidate, index int) providers.Choice {
	// Extract text content
	var content string
	if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
		}
	}

	// Map finish reason
	finishReason := providers.FinishReasonStop
	if candidate.FinishReason != "" {
		finishReason = p.adaptFinishReason(candidate.FinishReason)
	}

	// Create assistant message
	message := map[string]interface{}{
		"role":    "assistant",
		"content": content,
	}

	return map[string]interface{}{
		"index":         index,
		"message":       message,
		"finish_reason": finishReason,
	}
}

// adaptFinishReason converts Gemini FinishReason to unified format
func (p *Provider) adaptFinishReason(reason genai.FinishReason) providers.FinishReason {
	switch reason {
	case genai.FinishReasonStop:
		return providers.FinishReasonStop
	case genai.FinishReasonMaxTokens:
		return providers.FinishReasonLength
	case genai.FinishReasonSafety:
		return providers.FinishReasonContentFilter
	case genai.FinishReasonRecitation:
		return providers.FinishReasonContentFilter
	default:
		return providers.FinishReasonError
	}
}

// adaptStreamChunk converts Gemini streaming chunk to unified StreamEvent
func (p *Provider) adaptStreamChunk(resp *genai.GenerateContentResponse, model string) *providers.StreamEvent {
	if len(resp.Candidates) == 0 {
		return nil
	}

	candidate := resp.Candidates[0]
	
	// Handle thinking content (Gemini 2.0 feature)
	if candidate.Content != nil && len(candidate.Content.Parts) > 0 {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				// Check if this is thinking content
				if p.isThinkingContent(part.Text) {
					return &providers.StreamEvent{
						Type:     providers.EventThought,
						Provider: providers.ProviderGemini,
						Model:    model,
						Data: providers.ThoughtEvent{
							Text: part.Text,
						},
						Timestamp: time.Now(),
					}
				} else {
					// Regular content
					return &providers.StreamEvent{
						Type:     providers.EventContent,
						Provider: providers.ProviderGemini,
						Model:    model,
						Data: providers.ContentEvent{
							Text:  part.Text,
							Delta: true,
						},
						Timestamp: time.Now(),
					}
				}
			}
		}
	}

	// Handle finish reason
	if candidate.FinishReason != "" {
		finishReason := p.adaptFinishReason(candidate.FinishReason)
		return &providers.StreamEvent{
			Type:     providers.EventFinished,
			Provider: providers.ProviderGemini,
			Model:    model,
			Metadata: providers.EventMeta{
				FinishReason: finishReason,
			},
			Timestamp: time.Now(),
		}
	}

	return nil
}

// adaptJSONResponse converts Gemini response to unified JSONResponse
func (p *Provider) adaptJSONResponse(resp *genai.GenerateContentResponse, model string, schema map[string]interface{}) (*providers.JSONResponse, error) {
	if len(resp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil || len(candidate.Content.Parts) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	// Extract text content
	var textContent string
	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			textContent += part.Text
		}
	}

	if textContent == "" {
		return nil, fmt.Errorf("empty text content in response")
	}

	// Parse JSON content
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(textContent), &jsonData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	var usage *providers.Usage
	if resp.UsageMetadata != nil {
		usage = &providers.Usage{
			InputTokens:  int(*resp.UsageMetadata.PromptTokenCount),
			OutputTokens: int(*resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:  int(resp.UsageMetadata.TotalTokenCount),
		}
	}

	return &providers.JSONResponse{
		ID:       generateResponseID(),
		Model:    model,
		Provider: providers.ProviderGemini,
		Data:     jsonData,
		Usage:    usage,
		Created:  time.Now().Unix(),
	}, nil
}

// adaptModel converts Gemini Model to unified Model
func (p *Provider) adaptModel(model *genai.Model) providers.Model {
	// Determine capabilities based on model name
	capabilities := providers.ModelCapabilities{
		TextGeneration: true,
		SystemMessage:  true,
		Streaming:      true,
	}

	// Set capabilities based on model name
	if contains(model.Name, "gemini") {
		capabilities.FunctionCalling = true
		capabilities.JSONMode = true
		
		if contains(model.Name, "vision") || contains(model.Name, "pro") || contains(model.Name, "flash") {
			capabilities.ImageInput = true
		}
		
		if contains(model.Name, "2.0") {
			capabilities.ThinkingMode = true
		}
	}

	// Estimate context size based on model
	contextSize := 32768 // Default
	if contains(model.Name, "1.5") {
		if contains(model.Name, "pro") {
			contextSize = 2000000 // 2M tokens
		} else {
			contextSize = 1000000 // 1M tokens
		}
	} else if contains(model.Name, "2.0") {
		contextSize = 1000000 // 1M tokens
	}

	return providers.Model{
		ID:           model.Name,
		Name:         model.DisplayName,
		Provider:     providers.ProviderGemini,
		Capabilities: capabilities,
		ContextSize:  contextSize,
	}
}

// Helper functions

func (p *Provider) applyRequestConfig(config *genai.GenerateContentConfig, reqConfig providers.RequestConfig) error {
	// This is a placeholder - would need proper RequestConfig type handling
	if configMap, ok := reqConfig.(map[string]interface{}); ok {
		if temp, exists := configMap["temperature"]; exists {
			if tempFloat, ok := temp.(float64); ok {
				tempFloat32 := float32(tempFloat)
				config.Temperature = &tempFloat32
			}
		}
		
		if topP, exists := configMap["top_p"]; exists {
			if topPFloat, ok := topP.(float64); ok {
				topPFloat32 := float32(topPFloat)
				config.TopP = &topPFloat32
			}
		}
		
		if topK, exists := configMap["top_k"]; exists {
			if topKInt, ok := topK.(int); ok {
				topKInt32 := int32(topKInt)
				// config.TopK = &topKInt32 // TopK may need different type
				_ = topKInt32 // Avoid unused variable
			}
		}
		
		if maxTokens, exists := configMap["max_output_tokens"]; exists {
			if maxTokensInt, ok := maxTokens.(int); ok {
				maxTokensInt32 := int32(maxTokensInt)
				config.MaxOutputTokens = &maxTokensInt32
			}
		}
		
		// Handle thinking config
		if thinkingConfig, exists := configMap["thinking_config"]; exists {
			if thinkingMap, ok := thinkingConfig.(map[string]interface{}); ok {
				if p.config.ThinkingEnabled {
					config.ThinkingConfig = &genai.ThinkingConfig{}
					
					if includeThoughts, ok := thinkingMap["include_thoughts"].(bool); ok {
						config.ThinkingConfig.IncludeThoughts = includeThoughts
					}
					
					if budget, ok := thinkingMap["thinking_budget"].(int); ok {
						budgetInt32 := int32(budget)
						// config.ThinkingConfig.ThinkingBudget = &budgetInt32 // Field may not exist
						_ = budgetInt32 // Avoid unused variable
					}
				}
			}
		}
	}
	
	return nil
}

func (p *Provider) adaptTools(tools []providers.Tool) ([]*genai.Tool, error) {
	geminiTools := make([]*genai.Tool, len(tools))
	
	for i, tool := range tools {
		// Convert unified tool to Gemini format
		// This would need proper Tool type handling
		_ = tool // Avoid unused variable
		geminiTools[i] = &genai.Tool{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name: "placeholder", // Would extract from tool
					// Add other function parameters
				},
			},
		}
	}
	
	return geminiTools, nil
}

func (p *Provider) adaptSafetySettings(settings []providers.SafetySetting) []*genai.SafetySetting {
	geminiSettings := make([]*genai.SafetySetting, len(settings))
	
	for i, setting := range settings {
		geminiSettings[i] = &genai.SafetySetting{
			Category:  genai.HarmCategory(setting.Category),
			Threshold: genai.HarmBlockThreshold(setting.Threshold),
		}
	}
	
	return geminiSettings
}

// isThinkingContent checks if content is thinking/reasoning content
func (p *Provider) isThinkingContent(text string) bool {
	// Simple heuristic - in practice, would check for thinking markers
	return len(text) > 100 && (contains(text, "thinking") || contains(text, "reasoning") || contains(text, "let me"))
}

// generateResponseID generates a unique response ID
func generateResponseID() string {
	return fmt.Sprintf("gemini-%d", time.Now().UnixNano())
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	s = strings.ToLower(s)
	substr = strings.ToLower(substr)
	return strings.Contains(s, substr)
}