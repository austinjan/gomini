package openai

import (
	"context"
	"testing"
	"time"

	"gomini/pkg/gomini/providers"
)

func TestProvider_SendMessageStream_ErrorHandling(t *testing.T) {
	// Create a provider with invalid configuration to trigger network errors
	config := &Config{
		APIKey:       "invalid-key",
		BaseURL:      "https://invalid-url-that-does-not-exist.com", // This will cause DNS errors
		DefaultModel: "gpt-4o-mini",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create a test request
	request := &providers.ChatRequest{
		Messages: []providers.Message{
			map[string]interface{}{
				"role":    "user",
				"content": "Hello, world!",
			},
		},
		Model: "gpt-4o-mini",
	}

	// Test that streaming with network errors doesn't panic
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	streamChan := provider.SendMessageStream(ctx, request)

	// The stream should produce an error event, not panic
	errorReceived := false
	for event := range streamChan {
		if event.Type == providers.EventError {
			errorReceived = true
			t.Logf("Received expected error event: %v", event.Error)
			break
		}
	}

	if !errorReceived {
		t.Error("Expected to receive an error event, but none was received")
	}

	// If we reach this point, the test passed - no panic occurred
	t.Log("Success: No panic occurred during network error handling")
}

func TestProvider_SendMessage_ErrorHandling(t *testing.T) {
	// Create a provider with invalid configuration
	config := &Config{
		APIKey:       "invalid-key",
		BaseURL:      "https://invalid-url-that-does-not-exist.com",
		DefaultModel: "gpt-4o-mini",
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create a test request
	request := &providers.ChatRequest{
		Messages: []providers.Message{
			map[string]interface{}{
				"role":    "user",
				"content": "Hello, world!",
			},
		},
		Model: "gpt-4o-mini",
	}

	// Test that non-streaming with network errors doesn't panic
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = provider.SendMessage(ctx, request)

	// We expect an error, not a panic
	if err == nil {
		t.Error("Expected an error due to invalid configuration, but none was received")
	} else {
		t.Logf("Received expected error: %v", err)
	}

	// If we reach this point, the test passed - no panic occurred
	t.Log("Success: No panic occurred during network error handling")
}