# Gomini - Unified Go LLM Client

A unified Go client for multiple Large Language Model (LLM) providers, starting with OpenAI and Google Gemini.

## Phase 1: Provider-Agnostic Foundation ✅

This phase establishes the core architecture for a unified LLM client that can work with multiple providers through a single interface.

### What's Implemented

#### Core Components

1. **LLMProvider Interface** (`provider.go`)
   - Unified interface for all LLM providers
   - Methods: `SendMessage`, `SendMessageStream`, `GenerateJSON`, `ListModels`, `GetCapabilities`
   - Provider capabilities and model definitions

2. **Unified Types** (`types.go`)
   - Abstract message and content types that work with both OpenAI and Gemini
   - Request/response structures with provider-specific adaptations
   - Helper functions for creating messages and content

3. **Configuration System** (`config.go`)
   - Multi-provider configuration with environment variable support
   - Provider-specific settings (OpenAI, Gemini, Vertex AI)
   - Router configuration for provider selection strategies

4. **Streaming Events** (`events.go`)
   - Unified event system for streaming responses
   - Rich event types: content, tool calls, errors, provider switches
   - Helper functions for creating events

5. **Error Handling** (`errors.go`)
   - Provider-agnostic error classification and handling
   - Automatic error mapping from provider-specific to unified errors
   - Retry logic and error categorization

#### Key Features

- **Provider Abstraction**: Single interface works with multiple providers
- **Configuration Flexibility**: Support for API keys, Vertex AI, custom endpoints
- **Rich Event System**: Comprehensive streaming with metadata
- **Error Resilience**: Intelligent error classification and retry strategies
- **Type Safety**: Strong typing throughout the API

### Environment Variables

```bash
# OpenAI Configuration
OPENAI_API_KEY=sk-...
OPENAI_ORGANIZATION=org-...
OPENAI_BASE_URL=https://api.openai.com/v1

# Gemini Configuration  
GEMINI_API_KEY=...
GOOGLE_API_KEY=...

# Vertex AI Configuration
GOOGLE_GENAI_USE_VERTEXAI=true
GOOGLE_CLOUD_PROJECT=your-project
GOOGLE_CLOUD_LOCATION=us-central1

# Global Settings
GOMINI_DEFAULT_PROVIDER=openai  # or gemini
GOMINI_ROUTER_STRATEGY=lowest_cost
GOMINI_COST_OPTIMIZED=true
GOMINI_DEBUG=true
GOMINI_REQUEST_TIMEOUT=30s
GOMINI_MAX_RETRIES=3
```

### Usage Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "gomini/pkg/gomini"
)

func main() {
    // Create client from environment variables
    client, err := gomini.NewClientFromEnv()
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()
    
    // List available providers and models
    fmt.Printf("Enabled providers: %v\n", client.GetEnabledProviders())
    
    models, err := client.ListModels(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Available models: %d\n", len(models))
    
    // Send a simple message
    response, err := client.SendMessage(context.Background(), &gomini.ChatRequest{
        Messages: []gomini.Message{
            gomini.NewUserMessage("Hello, how are you?"),
        },
        Model: "gpt-4o", // Will auto-route to OpenAI
    })
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Response from %s: %s\n", response.Provider, response.Choices[0].Message.Content)
    
    // Stream a message from Gemini
    streamChan := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
        Messages: []gomini.Message{
            gomini.NewUserMessage("Write a haiku about Go programming"),
        },
        Model:    "gemini-2.0-flash-exp",
        Provider: "gemini", // Force Gemini provider
    })
    
    for event := range streamChan {
        if event.Type == gomini.EventContent {
            if contentData, ok := event.Data.(gomini.ContentEvent); ok {
                fmt.Print(contentData.Text)
            }
        }
    }
    fmt.Println()
}
```

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Unified Client                           │
├─────────────────────────────────────────────────────────────┤
│                  LLMProvider Interface                     │
├─────────────────────┬───────────────────────────────────────┤
│   OpenAI Provider   │           Gemini Provider             │
│                     │                                       │
│ • OpenAI Go SDK     │ • Google GenAI SDK                   │
│ • GPT-4, GPT-3.5    │ • Gemini 2.0, Gemini Pro            │
│ • Function Calling  │ • Function Calling                   │
│ • Vision            │ • Vision                              │
│ • Structured Output │ • Thinking Mode                      │
└─────────────────────┴───────────────────────────────────────┘
```

### Next Steps (Phase 2)

Ready to implement:
- `GeminiProvider` - Implementation using Google GenAI SDK
- `OpenAIProvider` - Implementation using OpenAI Go SDK
- Provider factory and registration
- Basic message/response adaptation

## Dependencies

```go
require (
    github.com/openai/openai-go v0.1.0-alpha.42
    google.golang.org/genai v0.5.0
)
```

## Project Structure

```
gomini/
├── go.mod                    # Go module with dependencies
├── cmd/                      # Command-line applications
│   └── example/
│       └── main.go          # Example usage
├── pkg/
│   └── gomini/              # Main reusable package
│       ├── client.go        # Main unified LLM client
│       ├── types.go         # Unified message/request types
│       ├── config.go        # Multi-provider configuration
│       ├── events.go        # Streaming event system
│       ├── errors.go        # Provider-agnostic errors
│       └── providers/       # Provider implementations
│           ├── provider.go  # Core LLMProvider interface
│           ├── openai/
│           │   ├── provider.go # OpenAI provider implementation
│           │   └── adapter.go  # OpenAI request/response adapters
│           └── gemini/
│               ├── provider.go # Gemini provider implementation
│               └── adapter.go  # Gemini request/response adapters
├── README.md                # This file
└── plan.md                 # Complete implementation plan
```

## Benefits

1. **Future-Proof**: Easy to add new providers (Anthropic, Cohere, etc.)
2. **Cost Optimization**: Route to cheapest provider automatically
3. **Reliability**: Fallback support prevents single-provider outages  
4. **Feature Leverage**: Use best capabilities from each provider
5. **Developer Experience**: Single API instead of learning multiple SDKs

## Quick Start

### 1. Set Environment Variables

```bash
# For OpenAI
export OPENAI_API_KEY="your_openai_api_key"

# For Gemini
export GEMINI_API_KEY="your_gemini_api_key"
# OR for Vertex AI
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT="your-project"
export GOOGLE_CLOUD_LOCATION="us-central1"
```

### 2. Run the Example

```bash
# Using Make
make example

# Or directly with Go
go run ./cmd/example/main.go
```

### 3. Use as Library

```bash
go mod init your-project
go get gomini/pkg/gomini
```

Then in your code:
```go
import "gomini/pkg/gomini"

client, err := gomini.NewClientFromEnv()
// ... use client
```

## Development

```bash
# Install dependencies
make deps

# Run tests
make test

# Format code
make format

# Run linters
make lint

# Run all checks
make check
```

---

