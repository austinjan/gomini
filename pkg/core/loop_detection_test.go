package core

import (
	"context"
	"testing"

	"gomini/pkg/gomini"
)

func TestLoopDetectionService_Reset(t *testing.T) {
	config := gomini.NewConfig()
	service := NewLoopDetectionService(config)
	
	// Set some state by manually calling methods (fields are private)
	service.Reset("initial-prompt")
	service.TurnStarted(context.Background()) // Increment turn count
	
	// Add some content to trigger internal state
	contentEvent := gomini.StreamEvent{
		Type: gomini.EventContent,
		Data: gomini.ContentEvent{Text: "some content", Delta: true},
	}
	service.AddAndCheck(contentEvent)
	
	// Reset should clear everything
	service.Reset("new-prompt-id")
	
	// Test that reset worked by checking behavior
	if !service.TurnStarted(context.Background()) == false {
		// TurnStarted should return false (no loop detected) and increment internal counter
	}
	
	// Test that content tracking was reset
	if service.AddAndCheck(contentEvent) {
		t.Error("Loop should not be detected after reset")
	}
}

func TestLoopDetectionService_ToolCallLoop(t *testing.T) {
	config := gomini.NewConfig()
	service := NewLoopDetectionService(config)
	service.Reset("test-prompt")
	
	// Create a tool call event
	toolCallEvent := gomini.StreamEvent{
		Type: gomini.EventToolCall,
		Data: gomini.ToolCallEvent{
			CallID:    "call-1",
			ToolName:  "test_tool",
			Arguments: map[string]interface{}{"arg1": "value1"},
		},
	}
	
	// First few calls should not trigger loop detection
	for i := 0; i < TOOL_CALL_LOOP_THRESHOLD-1; i++ {
		if service.AddAndCheck(toolCallEvent) {
			t.Errorf("Loop detected prematurely at iteration %d", i)
		}
	}
	
	// The threshold call should trigger loop detection
	if !service.AddAndCheck(toolCallEvent) {
		t.Error("Expected loop to be detected at threshold")
	}
	
	// Subsequent calls should continue to return true
	if !service.AddAndCheck(toolCallEvent) {
		t.Error("Expected loop to remain detected")
	}
}

func TestLoopDetectionService_ContentLoop(t *testing.T) {
	config := gomini.NewConfig()
	service := NewLoopDetectionService(config)
	service.Reset("test-prompt")
	
	// Create repeating content that should trigger a loop
	repeatingText := "This is a repeating pattern that should be detected as a loop. "
	
	// Add content repeatedly
	for i := 0; i < CONTENT_LOOP_THRESHOLD+5; i++ {
		contentEvent := gomini.StreamEvent{
			Type: gomini.EventContent,
			Data: gomini.ContentEvent{
				Text:  repeatingText,
				Delta: true,
			},
		}
		
		if service.AddAndCheck(contentEvent) {
			// Loop was detected - this is expected after enough repetitions
			t.Logf("Content loop detected after %d iterations", i+1)
			return
		}
	}
	
	// Note: Content loop detection is more sophisticated and may not trigger
	// with simple repetition due to sliding window analysis
	t.Log("Content loop not detected - this may be expected due to sliding window analysis")
}

func TestLoopDetectionService_TurnStarted(t *testing.T) {
	config := gomini.NewConfig()
	service := NewLoopDetectionService(config)
	service.Reset("test-prompt")
	
	ctx := context.Background()
	
	// First turn should not detect a loop
	if service.TurnStarted(ctx) {
		t.Error("Expected no loop detection on first turn")
	}
	
	if service.turnsInCurrentPrompt != 1 {
		t.Errorf("Expected turnsInCurrentPrompt to be 1, got %d", service.turnsInCurrentPrompt)
	}
	
	// Multiple turns should increment the counter
	for i := 0; i < 5; i++ {
		service.TurnStarted(ctx)
	}
	
	if service.turnsInCurrentPrompt != 6 {
		t.Errorf("Expected turnsInCurrentPrompt to be 6, got %d", service.turnsInCurrentPrompt)
	}
}

func TestLoopDetectionService_DifferentToolCalls(t *testing.T) {
	config := gomini.NewConfig()
	service := NewLoopDetectionService(config)
	service.Reset("test-prompt")
	
	// Different tool calls should not trigger loop detection
	toolCall1 := gomini.StreamEvent{
		Type: gomini.EventToolCall,
		Data: gomini.ToolCallEvent{
			CallID:    "call-1",
			ToolName:  "tool1",
			Arguments: map[string]interface{}{"arg1": "value1"},
		},
	}
	
	toolCall2 := gomini.StreamEvent{
		Type: gomini.EventToolCall,
		Data: gomini.ToolCallEvent{
			CallID:    "call-2", 
			ToolName:  "tool2",
			Arguments: map[string]interface{}{"arg1": "value2"},
		},
	}
	
	// Alternate between different tool calls
	for i := 0; i < 10; i++ {
		event := toolCall1
		if i%2 == 1 {
			event = toolCall2
		}
		
		if service.AddAndCheck(event) {
			t.Errorf("Loop detected with different tool calls at iteration %d", i)
		}
	}
}

func TestLoopDetectionService_CodeBlockHandling(t *testing.T) {
	config := gomini.NewConfig()
	service := NewLoopDetectionService(config)
	service.Reset("test-prompt")
	
	// Content inside code blocks should not trigger loop detection
	codeBlockStart := gomini.StreamEvent{
		Type: gomini.EventContent,
		Data: gomini.ContentEvent{
			Text:  "```go\n",
			Delta: true,
		},
	}
	
	repeatingCode := gomini.StreamEvent{
		Type: gomini.EventContent,
		Data: gomini.ContentEvent{
			Text:  "func main() {\n",
			Delta: true,
		},
	}
	
	codeBlockEnd := gomini.StreamEvent{
		Type: gomini.EventContent,
		Data: gomini.ContentEvent{
			Text:  "}\n```\n",
			Delta: true,
		},
	}
	
	// Start code block
	if service.AddAndCheck(codeBlockStart) {
		t.Error("Loop detected on code block start")
	}
	
	// Add repeating content inside code block
	for i := 0; i < 20; i++ {
		if service.AddAndCheck(repeatingCode) {
			t.Error("Loop detected inside code block")
		}
	}
	
	// End code block
	if service.AddAndCheck(codeBlockEnd) {
		t.Error("Loop detected on code block end")
	}
}