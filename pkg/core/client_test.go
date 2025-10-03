package core

import (
	"context"
	"testing"

	"gomini/pkg/gomini"
	"gomini/pkg/gomini/providers"
)

// MockProvider implements providers.LLMProvider for testing
type MockProvider struct {
	providerType providers.ProviderType
	responses    []gomini.StreamEvent
	callCount    int
}

func (m *MockProvider) SendMessage(ctx context.Context, request *gomini.ChatRequest) (*gomini.ChatResponse, error) {
	return &gomini.ChatResponse{
		Provider: m.providerType,
		Model:    request.Model,
		Choices: []gomini.Choice{
			gomini.NewAssistantMessage("Mock response"),
		},
	}, nil
}

func (m *MockProvider) SendMessageStream(ctx context.Context, request *gomini.ChatRequest) <-chan providers.StreamEvent {
	resultChan := make(chan providers.StreamEvent, len(m.responses))
	
	go func() {
		defer close(resultChan)
		
		for _, event := range m.responses {
			// Convert gomini.StreamEvent to providers.StreamEvent
			providerEvent := providers.StreamEvent{
				Type:      providers.EventType(event.Type),
				Provider:  event.Provider,
				Model:     event.Model,
				Data:      event.Data,
				Error:     event.Error,
				Timestamp: event.Timestamp,
				RequestID: event.RequestID,
				Metadata: providers.EventMeta{
					FinishReason: event.Metadata.FinishReason,
					Usage:        event.Metadata.Usage,
				},
			}
			resultChan <- providerEvent
		}
	}()
	
	m.callCount++
	return resultChan
}

func (m *MockProvider) GenerateJSON(ctx context.Context, request *gomini.JSONRequest) (*gomini.JSONResponse, error) {
	return &gomini.JSONResponse{}, nil
}

func (m *MockProvider) ListModels(ctx context.Context) ([]gomini.Model, error) {
	return []gomini.Model{}, nil
}

func (m *MockProvider) GetCapabilities() providers.ProviderCapabilities {
	return providers.ProviderCapabilities{
		Models:             []string{"test-model"},
		MaxContextSize:     4096,
		SupportsStreaming:  true,
		SupportsFunctions:  true,
		SupportsJSONMode:   true,
	}
}

func (m *MockProvider) GetProviderType() providers.ProviderType {
	return m.providerType
}

func (m *MockProvider) Close() error {
	return nil
}

func TestClient_SessionTurnLimits(t *testing.T) {
	config := gomini.NewConfig()
	config.MaxSessionTurns = 3 // Set low limit for testing
	config.Providers[providers.ProviderOpenAI] = &gomini.ProviderConfig{
		Enabled: true,
		APIKey:  "test-key",
	}
	config.DefaultProvider = providers.ProviderOpenAI

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Replace provider with mock
	mockProvider := &MockProvider{
		providerType: providers.ProviderOpenAI,
		responses: []gomini.StreamEvent{
			{
				Type: gomini.EventContent,
				Data: gomini.ContentEvent{Text: "Hello", Delta: true},
			},
			{
				Type: gomini.EventFinished,
			},
		},
	}
	client.currentProvider = mockProvider

	// First three calls should work
	for i := 0; i < 3; i++ {
		streamChan := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
			Messages: []gomini.Message{
				gomini.NewUserMessage("Test message"),
			},
			Model: "test-model",
		}, "test-prompt")

		eventCount := 0
		for event := range streamChan {
			eventCount++
			if event.Type == gomini.EventMaxSessionTurns {
				t.Errorf("Unexpected max session turns event on turn %d", i+1)
			}
		}
		
		if eventCount == 0 {
			t.Errorf("No events received on turn %d", i+1)
		}
	}

	// Fourth call should trigger max session turns
	streamChan := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Test message"),
		},
		Model: "test-model",
	}, "test-prompt")

	foundMaxTurnsEvent := false
	for event := range streamChan {
		if event.Type == gomini.EventMaxSessionTurns {
			foundMaxTurnsEvent = true
			if data, ok := event.Data.(gomini.MaxSessionTurnsEvent); ok {
				if data.CurrentTurns != 4 {
					t.Errorf("Expected current turns to be 4, got %d", data.CurrentTurns)
				}
				if data.MaxTurns != 3 {
					t.Errorf("Expected max turns to be 3, got %d", data.MaxTurns)
				}
			} else {
				t.Error("Event data is not MaxSessionTurnsEvent")
			}
		}
	}

	if !foundMaxTurnsEvent {
		t.Error("Expected max session turns event not found")
	}
}

func TestClient_PromptIDReset(t *testing.T) {
	config := gomini.NewConfig()
	config.Providers[providers.ProviderOpenAI] = &gomini.ProviderConfig{
		Enabled: true,
		APIKey:  "test-key",
	}
	config.DefaultProvider = providers.ProviderOpenAI

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Replace provider with mock
	mockProvider := &MockProvider{
		providerType: providers.ProviderOpenAI,
		responses: []gomini.StreamEvent{
			{
				Type: gomini.EventFinished,
			},
		},
	}
	client.currentProvider = mockProvider

	// First call with prompt ID 1
	streamChan1 := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Test message"),
		},
		Model: "test-model",
	}, "prompt-1")
	
	// Consume the stream to ensure the goroutine executes
	for range streamChan1 {
		// Consume all events
	}

	if client.lastPromptID != "prompt-1" {
		t.Errorf("Expected last prompt ID to be 'prompt-1', got %s", client.lastPromptID)
	}

	if client.sessionTurnCount != 1 {
		t.Errorf("Expected session turn count to be 1, got %d", client.sessionTurnCount)
	}

	// Second call with same prompt ID should increment turn count
	streamChan2 := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Test message"),
		},
		Model: "test-model",
	}, "prompt-1")
	
	// Consume the stream to ensure the goroutine executes
	for range streamChan2 {
		// Consume all events
	}

	if client.sessionTurnCount != 2 {
		t.Errorf("Expected session turn count to be 2, got %d", client.sessionTurnCount)
	}

	// Call with new prompt ID should reset turn count
	streamChan3 := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Test message"),
		},
		Model: "test-model",
	}, "prompt-2")
	
	// Consume the stream to ensure the goroutine executes
	for range streamChan3 {
		// Consume all events
	}

	if client.lastPromptID != "prompt-2" {
		t.Errorf("Expected last prompt ID to be 'prompt-2', got %s", client.lastPromptID)
	}

	if client.sessionTurnCount != 1 {
		t.Errorf("Expected session turn count to be reset to 1, got %d", client.sessionTurnCount)
	}
}

func TestClient_LoopDetectionIntegration(t *testing.T) {
	config := gomini.NewConfig()
	config.LoopDetectionEnabled = true
	config.Providers[providers.ProviderOpenAI] = &gomini.ProviderConfig{
		Enabled: true,
		APIKey:  "test-key",
	}
	config.DefaultProvider = providers.ProviderOpenAI

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create mock provider that sends repeating tool calls
	mockProvider := &MockProvider{
		providerType: providers.ProviderOpenAI,
		responses: make([]gomini.StreamEvent, 0),
	}

	// Add multiple identical tool call events to trigger loop detection
	for i := 0; i < TOOL_CALL_LOOP_THRESHOLD+1; i++ {
		mockProvider.responses = append(mockProvider.responses, gomini.StreamEvent{
			Type: gomini.EventToolCall,
			Data: gomini.ToolCallEvent{
				CallID:    "call-1",
				ToolName:  "repeat_tool",
				Arguments: map[string]interface{}{"arg": "value"},
			},
		})
	}

	client.currentProvider = mockProvider

	streamChan := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Test message"),
		},
		Model: "test-model",
	}, "test-prompt")

	foundLoopEvent := false
	eventCount := 0
	for event := range streamChan {
		eventCount++
		if event.Type == gomini.EventLoopDetected {
			foundLoopEvent = true
			if data, ok := event.Data.(gomini.LoopDetectedEvent); ok {
				if data.LoopType != gomini.LoopTypeToolCall {
					t.Errorf("Expected loop type to be ToolCall, got %s", data.LoopType)
				}
			} else {
				t.Error("Event data is not LoopDetectedEvent")
			}
			break
		}
	}

	if !foundLoopEvent {
		t.Error("Expected loop detected event not found")
	}

	// Should receive some events before loop detection kicks in
	if eventCount < TOOL_CALL_LOOP_THRESHOLD {
		t.Errorf("Expected at least %d events before loop detection, got %d", 
			TOOL_CALL_LOOP_THRESHOLD, eventCount)
	}
}

func TestClient_DisabledLoopDetection(t *testing.T) {
	config := gomini.NewConfig()
	config.LoopDetectionEnabled = false // Disable loop detection
	config.Providers[providers.ProviderOpenAI] = &gomini.ProviderConfig{
		Enabled: true,
		APIKey:  "test-key",
	}
	config.DefaultProvider = providers.ProviderOpenAI

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create mock provider that sends repeating tool calls
	mockProvider := &MockProvider{
		providerType: providers.ProviderOpenAI,
		responses: make([]gomini.StreamEvent, 0),
	}

	// Add many identical tool call events
	for i := 0; i < TOOL_CALL_LOOP_THRESHOLD*2; i++ {
		mockProvider.responses = append(mockProvider.responses, gomini.StreamEvent{
			Type: gomini.EventToolCall,
			Data: gomini.ToolCallEvent{
				CallID:    "call-1",
				ToolName:  "repeat_tool",
				Arguments: map[string]interface{}{"arg": "value"},
			},
		})
	}

	client.currentProvider = mockProvider

	streamChan := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Test message"),
		},
		Model: "test-model",
	}, "test-prompt")

	eventCount := 0
	for event := range streamChan {
		eventCount++
		if event.Type == gomini.EventLoopDetected {
			t.Error("Loop detection should be disabled")
		}
	}

	// Should receive all events since loop detection is disabled
	expectedEvents := TOOL_CALL_LOOP_THRESHOLD * 2
	if eventCount != expectedEvents {
		t.Errorf("Expected %d events, got %d", expectedEvents, eventCount)
	}
}