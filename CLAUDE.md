# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gomini is a unified Go client for multiple Large Language Model (LLM) providers, currently supporting OpenAI and Google Gemini. The project provides a single, consistent interface for working with different LLM providers, enabling easy provider switching, fallback support, and cost optimization.

## Build and Test Commands

```bash
# Install dependencies
make deps

# Build the example application
make build

# Run the example (requires environment variables - see below)
make example

# Run all tests
make test

# Format code
make format

# Run linters (requires golangci-lint)
make lint

# Run all checks (format, lint, test)
make check

# Run a specific test
go test -v ./pkg/core -run TestLoopDetection

# Run a single test file
go test -v ./pkg/core/loop_detection_test.go ./pkg/core/loop_detection.go
```

## Required Environment Variables

Set these before running examples or tests that interact with providers:

```bash
# For OpenAI
export OPENAI_API_KEY="your_openai_api_key"

# For Gemini (choose one)
export GEMINI_API_KEY="your_gemini_api_key"
# OR for Vertex AI
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT="your-project"
export GOOGLE_CLOUD_LOCATION="us-central1"

# Optional global settings
export GOMINI_DEFAULT_PROVIDER="openai"  # or "gemini"
export GOMINI_DEBUG="true"
export GOMINI_MAX_SESSION_TURNS="100"
export GOMINI_LOOP_DETECTION_ENABLED="true"
```

## Architecture

### Core Package Structure

The codebase is organized into two main package hierarchies:

1. **`pkg/gomini`** - High-level API that consumers interact with
   - `types.go` - Type aliases that expose provider types to consumers
   - `config.go` - Multi-provider configuration with environment variable support
   - `events.go` - Unified streaming event system
   - `errors.go` - Provider-agnostic error handling

2. **`pkg/core`** - Core client implementation and advanced features
   - `client.go` - Main unified client with provider switching and routing
   - `loop_detection.go` - Sophisticated loop detection for tool calls and content repetition
   - Session management and turn counting

3. **`pkg/gomini/providers`** - Provider-specific implementations
   - `provider.go` - Core `LLMProvider` interface that all providers implement
   - `openai/` - OpenAI provider using official SDK
   - `gemini/` - Google Gemini provider using GenAI SDK

### Key Design Patterns

**Provider Pattern**: All LLM providers implement the `LLMProvider` interface (pkg/gomini/providers/provider.go:18), which defines methods like `SendMessage`, `SendMessageStream`, `GenerateJSON`, `ListModels`, and `GetCapabilities`.

**Adapter Pattern**: Each provider has an `adapter.go` file that converts between the unified types (in `providers.ChatRequest`, `providers.ChatResponse`) and provider-specific SDK types.

**Client Coordination**: The `core.Client` (pkg/core/client.go:20) orchestrates provider switching, streaming with loop detection, and session management. It wraps a single active provider and can switch providers dynamically based on request parameters.

**Loop Detection Service**: The `LoopDetectionService` (pkg/core/loop_detection.go:33) implements sophisticated algorithms to detect:
- Tool call loops (same tool+args repeated 5+ times)
- Content repetition loops (50-char chunks repeated 10+ times within sliding window)
- Future: LLM-based cognitive loop detection

### Type System

The type system uses aliases to maintain a clean separation:
- `pkg/gomini/providers` defines the canonical types (`ChatRequest`, `StreamEvent`, etc.)
- `pkg/gomini` re-exports these as type aliases for consumer convenience
- This allows internal packages to import `providers` while consumers import `gomini`

## Key Implementation Details

### Streaming and Loop Detection

When streaming responses via `SendMessageStream` (pkg/core/client.go:151):
1. Session turn count is incremented per turn
2. Loop detector checks for repetitive patterns in each event
3. If a loop is detected, a `LoopDetected` event is emitted and the stream terminates
4. Tool calls reset content tracking; content events build up history for analysis

### Provider Switching

The client maintains a single active provider at a time. To switch providers:
1. Call `SwitchProvider(providerType)` explicitly, OR
2. Pass `Provider` field in `ChatRequest` - client auto-switches
3. Old provider is closed before new provider is initialized

### Configuration

Configuration follows a hierarchy:
1. Create base `Config` with `NewConfig()`
2. Load environment variables with `LoadFromEnv()`
3. Override specific fields programmatically if needed
4. Call `Validate()` before use

Each provider has a `ProviderConfig` that includes:
- Common fields (APIKey, Endpoint, DefaultModel)
- Provider-specific nested config (OpenAI, Gemini)
- Rate limiting, extra headers, etc.

### Error Handling

Errors are categorized and wrapped:
- Provider-specific errors are mapped to unified error codes
- Use `WrapProviderError` to add context (provider, model)
- Error codes include: `invalid_api_key`, `invalid_auth`, `invalid_request`, etc.

## Development Workflow

1. **Adding a new provider**:
   - Create `pkg/gomini/providers/{name}/` directory
   - Implement `LLMProvider` interface in `provider.go`
   - Create adapters in `adapter.go` to convert to/from SDK types
   - Add provider type constant to `pkg/gomini/providers/provider.go`
   - Update `core.Client.initializeProvider` switch statement

2. **Adding new event types**:
   - Define in `pkg/gomini/providers/provider.go` (EventType constant)
   - Add corresponding data struct (e.g., `ToolCallEvent`)
   - Update `core.Client.convertEventData` to handle new type

3. **Testing**:
   - Unit tests live alongside implementation files (`*_test.go`)
   - Tests use table-driven approach where possible
   - Mock providers can be created by implementing `LLMProvider` interface
