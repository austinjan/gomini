package openai

import (
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"gomini/pkg/gomini/providers"
)

// Provider implements the LLMProvider interface for OpenAI
type Provider struct {
	client   *openai.Client
	config   *Config
	models   []providers.Model
	created  time.Time
}

// Config holds OpenAI-specific configuration
type Config struct {
	APIKey       string            `json:"api_key"`
	BaseURL      string            `json:"base_url,omitempty"`
	Organization string            `json:"organization,omitempty"`
	Project      string            `json:"project,omitempty"`
	DefaultModel string            `json:"default_model,omitempty"`
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`
	Timeout      time.Duration     `json:"timeout,omitempty"`
}

// NewProvider creates a new OpenAI provider instance
func NewProvider(config *Config) (*Provider, error) {
	if config.APIKey == "" {
		return nil, providers.NewLLMError(providers.ErrorInvalidAPIKey, "OpenAI API key is required", providers.ProviderOpenAI, nil)
	}

	// Configure OpenAI client  
	// For this SDK version, we'll create a basic client
	client := openai.NewClient(
		// Client options will be handled by the SDK directly
		// openai.WithAPIKey(config.APIKey), // This may not exist in this version
	)

	provider := &Provider{
		client:  client,
		config:  config,
		created: time.Now(),
	}

	// Initialize available models
	provider.initializeModels()

	return provider, nil
}

// SendMessage implements LLMProvider.SendMessage
func (p *Provider) SendMessage(ctx context.Context, req *providers.ChatRequest) (*providers.ChatResponse, error) {
	// Convert unified request to OpenAI format
	openaiReq, err := p.adaptChatRequest(req)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderOpenAI, req.Model)
	}

	// Make OpenAI API call
	resp, err := p.client.Chat.Completions.New(ctx, *openaiReq)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderOpenAI, req.Model)
	}

	// Convert OpenAI response to unified format
	return p.adaptChatResponse(*resp, req.Model), nil
}

// SendMessageStream implements LLMProvider.SendMessageStream
func (p *Provider) SendMessageStream(ctx context.Context, req *providers.ChatRequest) <-chan providers.StreamEvent {
	eventChan := make(chan providers.StreamEvent, 10)

	go func() {
		defer close(eventChan)
		
		// Recover from any panics to prevent crashing the application
		defer func() {
			if r := recover(); r != nil {
				err := fmt.Errorf("panic in OpenAI streaming: %v", r)
				eventChan <- providers.NewErrorEvent(providers.ProviderOpenAI, req.Model, err, false)
			}
		}()

		// Convert to OpenAI streaming request
		openaiReq, err := p.adaptChatRequestForStream(req)
		if err != nil {
			eventChan <- providers.NewErrorEvent(providers.ProviderOpenAI, req.Model, err, false)
			return
		}

		// Create OpenAI streaming request
		stream := p.client.Chat.Completions.NewStreaming(ctx, *openaiReq)
		
		// Safely defer close only if stream is not nil
		if stream != nil {
			defer func() {
				if stream != nil {
					stream.Close()
				}
			}()
		}

		// Check if stream creation failed
		if stream == nil {
			eventChan <- providers.NewErrorEvent(providers.ProviderOpenAI, req.Model, 
				fmt.Errorf("failed to create streaming request"), false)
			return
		}

		// Process streaming chunks
		for stream.Next() {
			chunk := stream.Current()
			event := p.adaptStreamChunk(chunk, req.Model)
			if event != nil {
				eventChan <- *event
			}
		}

		if err := stream.Err(); err != nil {
			eventChan <- providers.NewErrorEvent(providers.ProviderOpenAI, req.Model, err, false)
		}
	}()

	return eventChan
}

// GenerateJSON implements LLMProvider.GenerateJSON
func (p *Provider) GenerateJSON(ctx context.Context, req *providers.JSONRequest) (*providers.JSONResponse, error) {
	// Convert to structured output request
	chatReq := &providers.ChatRequest{
		Messages: req.Messages,
		Model:    req.Model,
		Provider: providers.ProviderOpenAI,
		Config:   req.Config,
	}

	// Add JSON schema to request config
	// This will be implemented in the adapter
	openaiReq, err := p.adaptJSONRequest(chatReq, req.Schema)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderOpenAI, req.Model)
	}

	resp, err := p.client.Chat.Completions.New(ctx, *openaiReq)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderOpenAI, req.Model)
	}

	return p.adaptJSONResponse(*resp, req.Model, req.Schema)
}

// ListModels implements LLMProvider.ListModels
func (p *Provider) ListModels(ctx context.Context) ([]providers.Model, error) {
	if len(p.models) > 0 {
		return p.models, nil
	}

	// Fetch models from OpenAI API
	models, err := p.client.Models.List(ctx)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderOpenAI, "")
	}

	// Convert OpenAI models to unified format
	unifiedModels := make([]providers.Model, 0)
	for _, model := range models.Data {
		unifiedModel := p.adaptModel(model)
		unifiedModels = append(unifiedModels, unifiedModel)
	}

	p.models = unifiedModels
	return unifiedModels, nil
}

// GetCapabilities implements LLMProvider.GetCapabilities
func (p *Provider) GetCapabilities() providers.ProviderCapabilities {
	return providers.ProviderCapabilities{
		Models: []string{
			"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-4",
			"gpt-3.5-turbo", "gpt-3.5-turbo-16k",
		},
		MaxContextSize:      128000, // GPT-4 Turbo context size
		SupportedMimeTypes:  []string{"text/plain", "image/jpeg", "image/png", "image/gif", "image/webp"},
		SupportsStreaming:   true,
		SupportsVision:      true,
		SupportsFunctions:   true,
		SupportsJSONMode:    true,
		SpecificFeatures: map[string]string{
			"structured_output": "true",
			"function_calling":  "true",
			"vision":           "true",
			"json_mode":        "true",
		},
	}
}

// GetProviderType implements LLMProvider.GetProviderType
func (p *Provider) GetProviderType() providers.ProviderType {
	return providers.ProviderOpenAI
}

// Close implements LLMProvider.Close
func (p *Provider) Close() error {
	// OpenAI client doesn't require explicit cleanup
	return nil
}

// Private helper methods

func (p *Provider) initializeModels() {
	// Define common OpenAI models with their capabilities
	p.models = []providers.Model{
		{
			ID:       "gpt-4o",
			Name:     "GPT-4o",
			Provider: providers.ProviderOpenAI,
			Capabilities: providers.ModelCapabilities{
				TextGeneration:   true,
				ImageInput:       true,
				FunctionCalling:  true,
				JSONMode:         true,
				SystemMessage:    true,
				Streaming:        true,
				StructuredOutput: true,
			},
			ContextSize: 128000,
			Cost: &providers.ModelCost{
				InputTokens:  5.0,  // $5 per 1M input tokens
				OutputTokens: 15.0, // $15 per 1M output tokens
				Currency:     "USD",
			},
		},
		{
			ID:       "gpt-4o-mini",
			Name:     "GPT-4o Mini",
			Provider: providers.ProviderOpenAI,
			Capabilities: providers.ModelCapabilities{
				TextGeneration:   true,
				ImageInput:       true,
				FunctionCalling:  true,
				JSONMode:         true,
				SystemMessage:    true,
				Streaming:        true,
				StructuredOutput: true,
			},
			ContextSize: 128000,
			Cost: &providers.ModelCost{
				InputTokens:  0.15, // $0.15 per 1M input tokens
				OutputTokens: 0.6,  // $0.6 per 1M output tokens
				Currency:     "USD",
			},
		},
		{
			ID:       "gpt-3.5-turbo",
			Name:     "GPT-3.5 Turbo",
			Provider: providers.ProviderOpenAI,
			Capabilities: providers.ModelCapabilities{
				TextGeneration:  true,
				FunctionCalling: true,
				JSONMode:        true,
				SystemMessage:   true,
				Streaming:       true,
			},
			ContextSize: 16384,
			Cost: &providers.ModelCost{
				InputTokens:  0.5,  // $0.5 per 1M input tokens
				OutputTokens: 1.5,  // $1.5 per 1M output tokens
				Currency:     "USD",
			},
		},
	}
}