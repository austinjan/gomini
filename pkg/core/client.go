package core

import (
	"context"
	"fmt"
	"time"

	"gomini/pkg/gomini"
	"gomini/pkg/gomini/providers"
	"gomini/pkg/gomini/providers/gemini"
	"gomini/pkg/gomini/providers/openai"
)

// Constants from TypeScript version
const (
	MAX_TURNS = 100 // Maximum turns to prevent infinite loops
)

// Client is the main unified LLM client  
type Client struct {
	config          *gomini.Config
	currentProvider providers.LLMProvider
	providerType    providers.ProviderType
	created         time.Time
	
	// Session management and loop detection
	sessionTurnCount int
	lastPromptID     string
	loopDetector     *LoopDetectionService
}

// NewClient creates a new unified LLM client
func NewClient(config *gomini.Config) (*Client, error) {
	if config == nil {
		config = gomini.NewConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	client := &Client{
		config:       config,
		created:      time.Now(),
		loopDetector: NewLoopDetectionService(config),
	}

	// Initialize with default provider
	defaultProvider := config.DefaultProvider
	if defaultProvider == "" {
		// Pick the first enabled provider
		enabledProviders := config.GetEnabledProviders()
		if len(enabledProviders) == 0 {
			return nil, fmt.Errorf("no providers enabled")
		}
		defaultProvider = enabledProviders[0]
	}

	if err := client.initializeProvider(defaultProvider); err != nil {
		return nil, fmt.Errorf("failed to initialize default provider: %w", err)
	}

	return client, nil
}

// NewClientFromEnv creates a new client from environment variables
func NewClientFromEnv() (*Client, error) {
	config := gomini.NewConfig()
	if err := config.LoadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}
	return NewClient(config)
}

// initializeProvider sets up a specific provider
func (c *Client) initializeProvider(providerType providers.ProviderType) error {
	providerConfig, err := c.config.GetProviderConfig(providerType)
	if err != nil {
		return fmt.Errorf("provider %s not found in config: %w", providerType, err)
	}

	if !providerConfig.Enabled {
		return fmt.Errorf("provider %s is not enabled", providerType)
	}

	var provider providers.LLMProvider

	switch providerType {
	case providers.ProviderGemini:
		geminiConfig := c.convertToGeminiConfig(providerConfig)
		provider, err = gemini.NewProvider(geminiConfig)
	case providers.ProviderOpenAI:
		openaiConfig := c.convertToOpenAIConfig(providerConfig)
		provider, err = openai.NewProvider(openaiConfig)
	default:
		return fmt.Errorf("unsupported provider type: %s", providerType)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize %s provider: %w", providerType, err)
	}

	// Close existing provider if any
	if c.currentProvider != nil {
		c.currentProvider.Close()
	}

	c.currentProvider = provider
	c.providerType = providerType
	return nil
}

// SwitchProvider changes the active provider
func (c *Client) SwitchProvider(providerType providers.ProviderType) error {
	if c.providerType == providerType {
		return nil // Already using this provider
	}

	return c.initializeProvider(providerType)
}

// GetCurrentProvider returns the currently active provider
func (c *Client) GetCurrentProvider() providers.LLMProvider {
	return c.currentProvider
}

// GetCurrentProviderType returns the type of current provider
func (c *Client) GetCurrentProviderType() providers.ProviderType {
	return c.providerType
}

// GetAvailableProviders returns list of available (enabled) providers
func (c *Client) GetAvailableProviders() []providers.ProviderType {
	return c.config.GetEnabledProviders()
}

// SendMessage sends a message and returns a response
func (c *Client) SendMessage(ctx context.Context, request *gomini.ChatRequest) (*gomini.ChatResponse, error) {
	// If request specifies a different provider, switch to it
	if request.Provider != "" && providers.ProviderType(request.Provider) != c.providerType {
		if err := c.SwitchProvider(providers.ProviderType(request.Provider)); err != nil {
			return nil, fmt.Errorf("failed to switch to provider %s: %w", request.Provider, err)
		}
	}

	// Use current provider
	return c.currentProvider.SendMessage(ctx, request)
}

// SendMessageStream sends a message and returns a stream of events with loop detection and session management
func (c *Client) SendMessageStream(ctx context.Context, request *gomini.ChatRequest, promptID string) <-chan gomini.StreamEvent {
	resultChan := make(chan gomini.StreamEvent, 10)
	
	go func() {
		defer close(resultChan)
		
		// Session management and loop detection setup
		if c.lastPromptID != promptID {
			c.loopDetector.Reset(promptID)
			c.lastPromptID = promptID
			c.sessionTurnCount = 0 // Reset session turn count for new prompt
		}
		
		c.sessionTurnCount++
		
		// Check session turn limits
		if c.config.MaxSessionTurns > 0 && c.sessionTurnCount > c.config.MaxSessionTurns {
			event := gomini.NewMaxSessionTurnsEvent(c.providerType, request.Model, 
				c.sessionTurnCount, c.config.MaxSessionTurns, promptID)
			resultChan <- event
			return
		}
		
		// Check for loop at turn start
		if c.config.LoopDetectionEnabled {
			if loopDetected := c.loopDetector.TurnStarted(ctx); loopDetected {
				event := gomini.NewLoopDetectedEvent(c.providerType, request.Model, 
					gomini.LoopTypeLLMDetected, promptID, "LLM detected conversation loop", 
					c.sessionTurnCount, 0)
				resultChan <- event
				return
			}
		}
		
		// Provider switching
		if request.Provider != "" && providers.ProviderType(request.Provider) != c.providerType {
			if err := c.SwitchProvider(providers.ProviderType(request.Provider)); err != nil {
				resultChan <- gomini.NewErrorEvent(c.providerType, request.Model, 
					fmt.Errorf("failed to switch provider: %w", err), false)
				return
			}
		}

		// Stream from current provider with loop detection
		providerChan := c.currentProvider.SendMessageStream(ctx, request)
		for event := range providerChan {
			// Convert provider StreamEvent to gomini StreamEvent
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
			
			// Check for loops in this event if loop detection is enabled
			if c.config.LoopDetectionEnabled && c.loopDetector.AddAndCheck(gominiEvent) {
				// Emit loop detected event
				loopType := gomini.LoopTypeToolCall
				description := "Tool call loop detected"
				if gominiEvent.Type == gomini.EventContent {
					loopType = gomini.LoopTypeContent
					description = "Content repetition loop detected"
				}
				
				loopEvent := gomini.NewLoopDetectedEvent(c.providerType, request.Model, 
					loopType, promptID, description, c.sessionTurnCount, 0)
				resultChan <- loopEvent
				return
			}
			
			// Forward the event
			resultChan <- gominiEvent
			
			// Check for errors
			if gominiEvent.Type == gomini.EventError {
				return
			}
		}
	}()
	
	return resultChan
}

// GenerateJSON generates structured JSON responses
func (c *Client) GenerateJSON(ctx context.Context, request *gomini.JSONRequest) (*gomini.JSONResponse, error) {
	// If request specifies a different provider, switch to it
	if request.Provider != "" && providers.ProviderType(request.Provider) != c.providerType {
		if err := c.SwitchProvider(providers.ProviderType(request.Provider)); err != nil {
			return nil, fmt.Errorf("failed to switch to provider %s: %w", request.Provider, err)
		}
	}

	// Use current provider
	return c.currentProvider.GenerateJSON(ctx, request)
}

// ListModels lists all available models from current provider
func (c *Client) ListModels(ctx context.Context) ([]gomini.Model, error) {
	return c.currentProvider.ListModels(ctx)
}

// GetEnabledProviders returns a list of enabled provider types (alias for GetAvailableProviders)
func (c *Client) GetEnabledProviders() []providers.ProviderType {
	return c.GetAvailableProviders()
}

// GetProvider returns the current provider if it matches the requested type
func (c *Client) GetProvider(providerType providers.ProviderType) (providers.LLMProvider, error) {
	if c.providerType == providerType {
		return c.currentProvider, nil
	}
	return nil, fmt.Errorf("provider %s is not currently active (current: %s)", providerType, c.providerType)
}

// convertToGeminiConfig converts gomini.ProviderConfig to gemini.Config
func (c *Client) convertToGeminiConfig(pc *gomini.ProviderConfig) *gemini.Config {
	config := &gemini.Config{
		APIKey:       pc.APIKey,
		Project:      pc.Project,
		Location:     pc.Location,
		UseVertexAI:  pc.UseVertex,
		DefaultModel: pc.DefaultModel,
		ExtraHeaders: pc.ExtraHeaders,
	}
	
	// Use Gemini-specific config if available
	if pc.Gemini != nil {
		config.SafetySettings = pc.Gemini.SafetySettings
		config.ThinkingEnabled = pc.Gemini.ThinkingEnabled
		config.ThinkingBudget = pc.Gemini.ThinkingBudget
		if pc.Gemini.DefaultModel != "" {
			config.DefaultModel = pc.Gemini.DefaultModel
		}
	}
	
	return config
}

// convertToOpenAIConfig converts gomini.ProviderConfig to openai.Config
func (c *Client) convertToOpenAIConfig(pc *gomini.ProviderConfig) *openai.Config {
	config := &openai.Config{
		APIKey:       pc.APIKey,
		BaseURL:      pc.Endpoint,
		Project:      pc.Project,
		DefaultModel: pc.DefaultModel,
		ExtraHeaders: pc.ExtraHeaders,
	}
	
	// Use OpenAI-specific config if available
	if pc.OpenAI != nil {
		config.Organization = pc.OpenAI.Organization
		if pc.OpenAI.BaseURL != "" {
			config.BaseURL = pc.OpenAI.BaseURL
		}
		if pc.OpenAI.DefaultModel != "" {
			config.DefaultModel = pc.OpenAI.DefaultModel
		}
	}
	
	return config
}

// convertEventData converts provider event data to gomini event data
func (c *Client) convertEventData(eventType providers.EventType, data interface{}) interface{} {
	switch eventType {
	case providers.EventContent:
		if providerContentEvent, ok := data.(providers.ContentEvent); ok {
			return gomini.ContentEvent{
				Text:     providerContentEvent.Text,
				Delta:    providerContentEvent.Delta,
				Complete: providerContentEvent.Complete,
			}
		}
	case providers.EventThought:
		if providerThoughtEvent, ok := data.(providers.ThoughtEvent); ok {
			return gomini.ThoughtEvent{
				Subject:     providerThoughtEvent.Subject,
				Description: providerThoughtEvent.Description,
				Text:        providerThoughtEvent.Text,
			}
		}
	}
	// For other event types or if conversion fails, return data as-is
	return data
}

// Close closes the client and cleans up resources
func (c *Client) Close() error {
	if c.currentProvider != nil {
		return c.currentProvider.Close()
	}
	return nil
}