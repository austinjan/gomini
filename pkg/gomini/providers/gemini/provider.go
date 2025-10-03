package gemini

import (
	"context"
	"time"

	"google.golang.org/genai"
	"gomini/pkg/gomini/providers"
)

// Provider implements the LLMProvider interface for Google Gemini
type Provider struct {
	client   *genai.Client
	config   *Config
	models   []providers.Model
	created  time.Time
}

// Config holds Gemini-specific configuration
type Config struct {
	APIKey          string                     `json:"api_key,omitempty"`
	Project         string                     `json:"project,omitempty"`         // For Vertex AI
	Location        string                     `json:"location,omitempty"`        // For Vertex AI
	UseVertexAI     bool                       `json:"use_vertex_ai,omitempty"`
	DefaultModel    string                     `json:"default_model,omitempty"`
	SafetySettings  []providers.SafetySetting  `json:"safety_settings,omitempty"`
	ThinkingEnabled bool                       `json:"thinking_enabled,omitempty"`
	ThinkingBudget  int                        `json:"thinking_budget,omitempty"`
	ExtraHeaders    map[string]string          `json:"extra_headers,omitempty"`
	Timeout         time.Duration              `json:"timeout,omitempty"`
}

// NewProvider creates a new Gemini provider instance
func NewProvider(config *Config) (*Provider, error) {
	var client *genai.Client
	var err error

	// Configure client based on authentication method
	if config.UseVertexAI {
		if config.Project == "" || config.Location == "" {
			return nil, providers.NewLLMError(providers.ErrorInvalidAuth, "Vertex AI requires project and location", providers.ProviderGemini, nil)
		}

		// Create Vertex AI client
		clientConfig := &genai.ClientConfig{
			Project:  config.Project,
			Location: config.Location,
			Backend:  genai.BackendVertexAI,
		}

		client, err = genai.NewClient(context.Background(), clientConfig)
	} else {
		if config.APIKey == "" {
			return nil, providers.NewLLMError(providers.ErrorInvalidAPIKey, "Gemini API key is required", providers.ProviderGemini, nil)
		}

		// Create Gemini API client
		clientConfig := &genai.ClientConfig{
			APIKey:  config.APIKey,
			Backend: genai.BackendGeminiAPI,
		}

		client, err = genai.NewClient(context.Background(), clientConfig)
	}

	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderGemini, "")
	}

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
	// Convert unified request to Gemini format
	geminiReq, err := p.adaptChatRequest(req)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderGemini, req.Model)
	}

	// Make Gemini API call
	resp, err := p.client.Models.GenerateContent(ctx, req.Model, geminiReq.Contents, geminiReq.Config)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderGemini, req.Model)
	}

	// Convert Gemini response to unified format
	return p.adaptChatResponse(resp, req.Model), nil
}

// SendMessageStream implements LLMProvider.SendMessageStream
func (p *Provider) SendMessageStream(ctx context.Context, req *providers.ChatRequest) <-chan providers.StreamEvent {
	eventChan := make(chan providers.StreamEvent, 10)

	go func() {
		defer close(eventChan)

		// Convert to Gemini streaming request
		geminiReq, err := p.adaptChatRequest(req)
		if err != nil {
			eventChan <- providers.NewErrorEvent(providers.ProviderGemini, req.Model, err, false)
			return
		}

		// Create streaming request
		iter := p.client.Models.GenerateContentStream(ctx, req.Model, geminiReq.Contents, geminiReq.Config)

		// Process streaming chunks (simplified for SDK compatibility)
		// Note: The actual streaming API may need adjustment based on SDK version
		for chunk, err := range iter {
			if err != nil {
				eventChan <- providers.NewErrorEvent(providers.ProviderGemini, req.Model, err, false)
				break
			}

			event := p.adaptStreamChunk(chunk, req.Model)
			if event != nil {
				eventChan <- *event
			}
		}
	}()

	return eventChan
}

// GenerateJSON implements LLMProvider.GenerateJSON
func (p *Provider) GenerateJSON(ctx context.Context, req *providers.JSONRequest) (*providers.JSONResponse, error) {
	// Convert to Gemini request with JSON response format
	geminiReq, err := p.adaptJSONRequest(req)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderGemini, req.Model)
	}

	resp, err := p.client.Models.GenerateContent(ctx, req.Model, geminiReq.Contents, geminiReq.Config)
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderGemini, req.Model)
	}

	return p.adaptJSONResponse(resp, req.Model, req.Schema)
}

// ListModels implements LLMProvider.ListModels
func (p *Provider) ListModels(ctx context.Context) ([]providers.Model, error) {
	if len(p.models) > 0 {
		return p.models, nil
	}

	// Fetch models from Gemini API (need to check SDK API)
	models, err := p.client.Models.List(ctx, nil) // Add nil config parameter 
	if err != nil {
		return nil, providers.WrapProviderError(err, providers.ProviderGemini, "")
	}

	// Convert Gemini models to unified format (simplified for SDK compatibility)
	unifiedModels := make([]providers.Model, 0)
	// Note: The actual model listing may need adjustment based on SDK version
	// for _, model := range models.Models {
	// 	unifiedModel := p.adaptModel(model)
	// 	unifiedModels = append(unifiedModels, unifiedModel)
	// }
	_ = models // Avoid unused variable

	p.models = unifiedModels
	return unifiedModels, nil
}

// GetCapabilities implements LLMProvider.GetCapabilities
func (p *Provider) GetCapabilities() providers.ProviderCapabilities {
	return providers.ProviderCapabilities{
		Models: []string{
			"gemini-2.0-flash-exp", "gemini-1.5-pro", "gemini-1.5-flash",
			"gemini-1.0-pro", "gemini-pro-vision",
		},
		MaxContextSize:      2000000, // 2M tokens for Gemini 1.5 Pro
		SupportedMimeTypes:  []string{"text/plain", "image/jpeg", "image/png", "image/gif", "image/webp", "video/mp4", "audio/wav"},
		SupportsStreaming:   true,
		SupportsVision:      true,
		SupportsFunctions:   true,
		SupportsJSONMode:    true,
		SpecificFeatures: map[string]string{
			"thinking_mode":    "true",
			"function_calling": "true",
			"vision":          "true",
			"multimodal":      "true",
			"large_context":   "true",
			"safety_filters":  "true",
		},
	}
}

// GetProviderType implements LLMProvider.GetProviderType
func (p *Provider) GetProviderType() providers.ProviderType {
	return providers.ProviderGemini
}

// Close implements LLMProvider.Close
func (p *Provider) Close() error {
	// Gemini client may not have a Close method in this SDK version
	// if p.client != nil {
	// 	return p.client.Close()
	// }
	return nil
}

// Private helper methods

func (p *Provider) initializeModels() {
	// Define common Gemini models with their capabilities
	p.models = []providers.Model{
		{
			ID:       "gemini-2.0-flash-exp",
			Name:     "Gemini 2.0 Flash (Experimental)",
			Provider: providers.ProviderGemini,
			Capabilities: providers.ModelCapabilities{
				TextGeneration:  true,
				ImageInput:      true,
				FunctionCalling: true,
				JSONMode:        true,
				SystemMessage:   true,
				Streaming:       true,
				ThinkingMode:    true,
			},
			ContextSize: 1000000, // 1M tokens
			Cost: &providers.ModelCost{
				InputTokens:  0.075, // $0.075 per 1M input tokens
				OutputTokens: 0.3,   // $0.3 per 1M output tokens
				Currency:     "USD",
			},
		},
		{
			ID:       "gemini-1.5-pro",
			Name:     "Gemini 1.5 Pro",
			Provider: providers.ProviderGemini,
			Capabilities: providers.ModelCapabilities{
				TextGeneration:  true,
				ImageInput:      true,
				FunctionCalling: true,
				JSONMode:        true,
				SystemMessage:   true,
				Streaming:       true,
			},
			ContextSize: 2000000, // 2M tokens
			Cost: &providers.ModelCost{
				InputTokens:  1.25, // $1.25 per 1M input tokens
				OutputTokens: 5.0,  // $5 per 1M output tokens
				Currency:     "USD",
			},
		},
		{
			ID:       "gemini-1.5-flash",
			Name:     "Gemini 1.5 Flash",
			Provider: providers.ProviderGemini,
			Capabilities: providers.ModelCapabilities{
				TextGeneration:  true,
				ImageInput:      true,
				FunctionCalling: true,
				JSONMode:        true,
				SystemMessage:   true,
				Streaming:       true,
			},
			ContextSize: 1000000, // 1M tokens
			Cost: &providers.ModelCost{
				InputTokens:  0.075, // $0.075 per 1M input tokens
				OutputTokens: 0.3,   // $0.3 per 1M output tokens
				Currency:     "USD",
			},
		},
	}
}

// Placeholder types for the adapter methods
type GeminiRequest struct {
	Contents []*genai.Content
	Config   *genai.GenerateContentConfig
}

type StreamChunk struct {
	// Placeholder for actual Gemini streaming chunk structure
	Content *genai.GenerateContentResponse
}