# SendMessage and SendMessageStream Implementation Comparison

## Overview
This document compares the `SendMessage` and `SendMessageStream` functions between the TypeScript version (`packages/core/src/core/client.ts`) and the Go version (`gomini/pkg/core/client.go`) to identify missing functionality.

## TypeScript Version Features (`packages/core/src/core/client.ts`)

The TypeScript version has a comprehensive `sendMessageStream` function (lines 424-544) with advanced features:

### 1. Loop Detection & Session Management
- **Prompt ID tracking**: `this.lastPromptId !== prompt_id` (line 431)
- **Loop detection reset**: `this.loopDetector.reset(prompt_id)` (line 432)
- **Session turn counting**: `this.sessionTurnCount++` (line 435)
- **Max turn limits**: `this.config.getMaxSessionTurns()` (line 437-442)
- **Bounded turns**: `Math.min(turns, MAX_TURNS)` to prevent infinite loops (line 444)

## Implementation Plan: Loop Detection & Session Management

### Overview
Based on analysis of the TypeScript `LoopDetectionService` and Go codebase, here's a comprehensive plan to implement the missing loop detection and session management features:

### Phase 1: Core Infrastructure 

#### 1.1 New Event Types
Add missing event types to `events.go`:
- `EventLoopDetected` - When a loop is detected
- `EventMaxSessionTurns` - When session turn limit is reached  
- `EventChatCompressed` - When chat compression occurs

#### 1.2 Loop Detection Service
Create `pkg/core/loop_detection.go`:
- `LoopDetectionService` struct with Go equivalent of TypeScript functionality
- Tool call loop detection (using SHA256 hashing like TypeScript)
- Content loop detection with sliding window analysis
- LLM-based loop detection using JSON generation
- Constants: `TOOL_CALL_LOOP_THRESHOLD=5`, `CONTENT_LOOP_THRESHOLD=10`, etc.

#### 1.3 Configuration Extensions
Extend `config.go` to add session management settings:
- `MaxSessionTurns int` - Maximum turns per session
- `SkipNextSpeakerCheck bool` - Skip automatic continuation logic
- `LoopDetectionEnabled bool` - Enable/disable loop detection, to detect same tool is called repeatedly, detect same content is repeated, or detect LLM is generating same response repeatedly

### Phase 2: Session Management

#### 2.1 Client State Enhancement
Modify `pkg/core/client.go` to add:
- `sessionTurnCount int` - Track turns in current session
- `lastPromptId string` - Track prompt ID changes
- `loopDetector *LoopDetectionService` - Instance of loop detection service

#### 2.2 Enhanced SendMessageStream
Upgrade `SendMessageStream` to include:
- Prompt ID tracking and comparison
- Session turn counting with MAX_TURNS=100 limit
- Loop detection at turn start and during streaming
- Bounded turns prevention: `min(requestedTurns, MAX_TURNS)`

### Phase 3: Integration & Error Handling

#### 3.1 Event Integration
- Emit `EventLoopDetected` when loops are found
- Emit `EventMaxSessionTurns` when session limits hit
- Integrate loop detection events into existing streaming

#### 3.2 Context Cancellation
- Proper `context.Context` handling for abort signals
- Graceful shutdown when loops detected
- Early return mechanisms for session limits

### Phase 4: Testing & Documentation

#### 4.1 Unit Tests
- Test loop detection algorithms
- Test session management limits
- Test event emission and handling
- Mock provider interactions

#### 4.2 Integration Tests
- End-to-end conversation flow testing
- Loop prevention in real scenarios
- Configuration validation

### Key Design Decisions

1. **Go Idiomatic Patterns**: Use `context.Context` instead of `AbortSignal`, channels for streaming
2. **Memory Management**: Implement content history truncation (MAX_HISTORY_LENGTH=1000)
3. **Concurrency Safety**: Use mutexes where needed for shared state
4. **Error Handling**: Proper Go error handling with wrapped errors
5. **Configuration**: Environment variable support for all new settings

### Implementation Order

1. **Events & Types** (1-2 hours) - Add missing event types and data structures
2. **Loop Detection Core** (4-6 hours) - Implement the core loop detection algorithms  
3. **Session Management** (2-3 hours) - Add session tracking to client
4. **Integration** (2-3 hours) - Wire everything together in SendMessageStream
5. **Testing** (3-4 hours) - Comprehensive test coverage
6. **Documentation** (1 hour) - Update docs and examples

**Total Estimated Time: 13-19 hours**

This plan provides feature parity with the TypeScript implementation while following Go best practices and maintaining compatibility with the existing codebase.

### 2. Chat Compression
- **Automatic compression**: `await this.tryCompressChat(prompt_id)` (line 452)
- **Token threshold checking**: Uses `COMPRESSION_TOKEN_THRESHOLD = 0.7` (line 103)
- **Compression events**: `GeminiEventType.ChatCompressed` (line 455)
- **Compression status tracking**: `CompressionStatus.COMPRESSED` (line 454)

### 3. IDE Context Integration
- **Context tracking**: `this.getIdeContextParts()` (line 472-474)
- **Delta calculation**: Compares `this.lastSentIdeContext` vs current (line 311-422)
- **Tool call awareness**: Prevents context updates during pending tool calls (line 466-470)
- **Full vs incremental**: `this.forceFullIdeContext` flag (line 473)

### 4. Advanced Turn Management
- **Multi-turn conversations**: Recursive `sendMessageStream` calls (line 534-540)
- **Next speaker checking**: `await checkNextSpeaker()` (line 517-521)
- **Auto-continuation**: "Please continue" messages (line 531)
- **Turn boundary detection**: Checks finish reasons and tool calls (line 504)

### 5. Model Switching Detection
- **Original model tracking**: `originalModel || this.config.getModel()` (line 450)
- **Switch detection**: `currentModel !== initialModel` (line 507)
- **Fallback prevention**: Stops execution after quota-based model switches (line 508-511)

### 6. Enhanced Error Handling
- **Abort signal propagation**: Throughout the function (line 426, 487, 504)
- **Loop detection**: `this.loopDetector.addAndCheck(event)` (line 495)
- **Error event handling**: `event.type === GeminiEventType.Error` (line 500)

## Go Version Implementation (`gomini/pkg/core/client.go`)

### SendMessage (lines 127-137)
```go
func (c *Client) SendMessage(ctx context.Context, request *gomini.ChatRequest) (*gomini.ChatResponse, error) {
    // Basic provider switching
    if request.Provider != "" && providers.ProviderType(request.Provider) != c.providerType {
        if err := c.SwitchProvider(providers.ProviderType(request.Provider)); err != nil {
            return nil, fmt.Errorf("failed to switch to provider %s: %w", request.Provider, err)
        }
    }
    // Simple delegation
    return c.currentProvider.SendMessage(ctx, request)
}
```

### SendMessageStream (lines 140-177)
```go
func (c *Client) SendMessageStream(ctx context.Context, request *gomini.ChatRequest) <-chan gomini.StreamEvent {
    resultChan := make(chan gomini.StreamEvent, 10)
    
    go func() {
        defer close(resultChan)
        
        // Basic provider switching
        if request.Provider != "" && providers.ProviderType(request.Provider) != c.providerType {
            if err := c.SwitchProvider(providers.ProviderType(request.Provider)); err != nil {
                resultChan <- gomini.NewErrorEvent(c.providerType, request.Model, 
                    fmt.Errorf("failed to switch provider: %w", err), false)
                return
            }
        }

        // Simple event forwarding with conversion
        providerChan := c.currentProvider.SendMessageStream(ctx, request)
        for event := range providerChan {
            gominiEvent := gomini.StreamEvent{
                Type:      gomini.EventType(event.Type),
                Provider:  event.Provider,
                Model:     event.Model,
                Data:      c.convertEventData(event.Type, event.Data),
                Error:     event.Error,
                Timestamp: event.Timestamp,
                RequestID: event.RequestID,
                Metadata:  gomini.EventMeta{
                    FinishReason: event.Metadata.FinishReason,
                    Usage:        event.Metadata.Usage,
                },
            }
            resultChan <- gominiEvent
        }
    }()
    
    return resultChan
}
```

## Missing Functionality in Go Version

### 1. Loop Detection Service L
- **Missing**: Loop detection mechanism
- **Missing**: Prompt ID tracking for loop prevention
- **Missing**: `LoopDetectionService` equivalent
- **Impact**: No protection against infinite conversation loops

### 2. Session Management L
- **Missing**: Session turn counting (`sessionTurnCount`)
- **Missing**: Maximum session turn limits
- **Missing**: Session timeout handling
- **Impact**: No conversation length controls

### 3. Chat Compression L
- **Missing**: Automatic chat compression when approaching token limits
- **Missing**: Token counting and threshold checking
- **Missing**: Compression status tracking or events
- **Missing**: `tryCompressChat` equivalent function
- **Impact**: No memory management for long conversations

### 4. IDE Context Integration L
- **Missing**: IDE context tracking (`getIdeContextParts`)
- **Missing**: Delta calculation for context changes
- **Missing**: Full vs incremental context sending logic
- **Missing**: Prevention of context updates during tool calls
- **Impact**: No IDE integration capabilities

### 5. Advanced Turn Management L
- **Missing**: Multi-turn conversation support with recursion
- **Missing**: "Next speaker" checking logic (`checkNextSpeaker`)
- **Missing**: Automatic continuation with "Please continue"
- **Missing**: Bounded turns to prevent infinite loops
- **Impact**: Limited conversation flow control

### 6. Model Switching Detection L
- **Missing**: Tracking of original vs current model
- **Missing**: Prevention of unwanted execution after fallbacks
- **Impact**: No protection against quota-based model switching issues

### 7. Enhanced Event System L
- **Missing**: Specialized event types:
  - `ChatCompressed`
  - `LoopDetected` 
  - `MaxSessionTurns`
- **Missing**: Compression info events
- **Missing**: Advanced event metadata
- **Impact**: Limited event visibility and debugging

### 8. Advanced Error Handling L
- **Missing**: Comprehensive abort signal propagation
- **Missing**: Model switching detection during calls
- **Missing**: Advanced error recovery mechanisms
- **Impact**: Less robust error handling

### 9. Tool Call Management L
- **Missing**: Pending tool call detection
- **Missing**: Tool call response validation
- **Missing**: Tool execution flow control
- **Impact**: No tool integration capabilities

### 10. Configuration Integration L
- **Missing**: Integration with IDE mode settings
- **Missing**: Debug mode support
- **Missing**: Skip next speaker check configuration
- **Impact**: Limited configurability

## Summary

The Go version implements only basic functionality:
-  Provider switching
-  Basic event streaming  
-  Simple error handling
-  Event data conversion

The TypeScript version implements a sophisticated conversation management system with:
- **Advanced conversation flow control**
- **Memory management through compression**
- **Loop detection and prevention** 
- **IDE integration capabilities**
- **Comprehensive error handling**
- **Multi-turn conversation support**

## Implementation Priority

### High Priority (Core Features)
1. **Loop Detection Service** - Critical for preventing infinite loops
2. **Session Management** - Important for conversation control
3. **Enhanced Event System** - Foundation for other features

### Medium Priority (Advanced Features)  
4. **Chat Compression** - Important for long conversations
5. **Advanced Turn Management** - Improves conversation flow
6. **Model Switching Detection** - Prevents quota-related issues

### Low Priority (Integration Features)
7. **IDE Context Integration** - Only needed for IDE integration
8. **Tool Call Management** - Only needed for tool support
9. **Configuration Integration** - Nice to have for flexibility

The Go version is currently a basic wrapper, while the TypeScript version is a production-ready conversation management system.
