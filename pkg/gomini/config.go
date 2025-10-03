package gomini

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	
	"gomini/pkg/gomini/providers"
)

// Config holds configuration for the unified LLM client
type Config struct {
	// Provider configurations
	Providers map[providers.ProviderType]*ProviderConfig `json:"providers"`
	
	// Global settings
	DefaultProvider providers.ProviderType `json:"default_provider,omitempty"`
	EnableFallback  bool         `json:"enable_fallback"`
	FallbackChain   []providers.ProviderType `json:"fallback_chain,omitempty"`
	
	// Routing settings
	Router *RouterConfig `json:"router,omitempty"`
	
	// Global request defaults
	DefaultConfig RequestConfig `json:"default_config,omitempty"`
	
	// Timeouts and limits
	RequestTimeout  time.Duration `json:"request_timeout,omitempty"`
	MaxRetries      int           `json:"max_retries,omitempty"`
	RetryDelay      time.Duration `json:"retry_delay,omitempty"`
	
	// Debug and logging
	Debug       bool   `json:"debug,omitempty"`
	LogLevel    string `json:"log_level,omitempty"`
	LogRequests bool   `json:"log_requests,omitempty"`
	
	// Session management and loop detection
	MaxSessionTurns       int  `json:"max_session_turns,omitempty"`
	SkipNextSpeakerCheck  bool `json:"skip_next_speaker_check,omitempty"`
	LoopDetectionEnabled  bool `json:"loop_detection_enabled,omitempty"`
}

// ProviderConfig holds configuration for a specific provider
type ProviderConfig struct {
	Enabled bool `json:"enabled"`
	
	// Authentication
	APIKey    string `json:"api_key,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
	Project   string `json:"project,omitempty"`   // Gemini/Vertex AI
	Location  string `json:"location,omitempty"`  // Gemini/Vertex AI
	UseVertex bool   `json:"use_vertex,omitempty"` // Use Vertex AI instead of Gemini API
	
	// Request settings
	DefaultModel string                 `json:"default_model,omitempty"`
	Models       []string               `json:"models,omitempty"` // Allowed models
	ExtraHeaders map[string]string      `json:"extra_headers,omitempty"`
	ExtraQuery   map[string]string      `json:"extra_query,omitempty"`
	ExtraBody    map[string]interface{} `json:"extra_body,omitempty"`
	
	// Rate limiting
	RateLimit *providers.RateLimit `json:"rate_limit,omitempty"`
	
	// Provider-specific settings
	OpenAI *OpenAIConfig `json:"openai,omitempty"`
	Gemini *GeminiConfig `json:"gemini,omitempty"`
}

// OpenAIConfig holds OpenAI-specific configuration
type OpenAIConfig struct {
	Organization   string `json:"organization,omitempty"`
	BaseURL        string `json:"base_url,omitempty"`
	DefaultModel   string `json:"default_model,omitempty"`
	MaxTokens      int    `json:"max_tokens,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
	TopP           float64 `json:"top_p,omitempty"`
	Stop           []string `json:"stop,omitempty"`
}

// GeminiConfig holds Gemini-specific configuration  
type GeminiConfig struct {
	DefaultModel     string          `json:"default_model,omitempty"`
	MaxOutputTokens  int             `json:"max_output_tokens,omitempty"`
	Temperature      float64         `json:"temperature,omitempty"`
	TopP             float64         `json:"top_p,omitempty"`
	TopK             int             `json:"top_k,omitempty"`
	SafetySettings   []SafetySetting `json:"safety_settings,omitempty"`
	ThinkingEnabled  bool            `json:"thinking_enabled,omitempty"`
	ThinkingBudget   int             `json:"thinking_budget,omitempty"`
}

// RouterConfig defines how to route requests between providers
type RouterConfig struct {
	Strategy           RouterStrategy    `json:"strategy"`
	CostOptimized      bool             `json:"cost_optimized,omitempty"`
	LoadBalance        bool             `json:"load_balance,omitempty"`
	ModelPreferences   map[string]providers.ProviderType `json:"model_preferences,omitempty"` // model -> preferred provider
	CapabilityRouting  bool             `json:"capability_routing,omitempty"`
	FallbackOnError    bool             `json:"fallback_on_error,omitempty"`
	MaxFallbackAttempts int             `json:"max_fallback_attempts,omitempty"`
}

// RouterStrategy defines routing strategies
type RouterStrategy string

const (
	StrategyRoundRobin    RouterStrategy = "round_robin"
	StrategyLeastLoaded   RouterStrategy = "least_loaded"
	StrategyLowestCost    RouterStrategy = "lowest_cost"
	StrategyBestCapability RouterStrategy = "best_capability"
	StrategyManual        RouterStrategy = "manual"
)

// NewConfig creates a new configuration with defaults
func NewConfig() *Config {
	return &Config{
		Providers:      make(map[providers.ProviderType]*ProviderConfig),
		EnableFallback: true,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
		Router: &RouterConfig{
			Strategy:            StrategyManual,
			FallbackOnError:     true,
			MaxFallbackAttempts: 2,
		},
		// Session management defaults
		MaxSessionTurns:       100,  // Match TypeScript MAX_TURNS
		SkipNextSpeakerCheck:  false, // Enable automatic continuation by default
		LoopDetectionEnabled:  true,  // Enable loop detection by default
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() error {
	// OpenAI configuration
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		if c.Providers[ProviderOpenAI] == nil {
			c.Providers[ProviderOpenAI] = &ProviderConfig{}
		}
		c.Providers[ProviderOpenAI].Enabled = true
		c.Providers[ProviderOpenAI].APIKey = apiKey
		
		if org := os.Getenv("OPENAI_ORGANIZATION"); org != "" {
			if c.Providers[ProviderOpenAI].OpenAI == nil {
				c.Providers[ProviderOpenAI].OpenAI = &OpenAIConfig{}
			}
			c.Providers[ProviderOpenAI].OpenAI.Organization = org
		}
		
		if baseURL := os.Getenv("OPENAI_BASE_URL"); baseURL != "" {
			if c.Providers[ProviderOpenAI].OpenAI == nil {
				c.Providers[ProviderOpenAI].OpenAI = &OpenAIConfig{}
			}
			c.Providers[ProviderOpenAI].OpenAI.BaseURL = baseURL
		}
	}
	
	// Gemini configuration
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		if c.Providers[ProviderGemini] == nil {
			c.Providers[ProviderGemini] = &ProviderConfig{}
		}
		c.Providers[ProviderGemini].Enabled = true
		c.Providers[ProviderGemini].APIKey = apiKey
	}
	
	// Alternative Gemini API key
	if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" {
		if c.Providers[ProviderGemini] == nil {
			c.Providers[ProviderGemini] = &ProviderConfig{}
		}
		c.Providers[ProviderGemini].Enabled = true
		c.Providers[ProviderGemini].APIKey = apiKey
	}
	
	// Vertex AI configuration
	if useVertex := os.Getenv("GOOGLE_GENAI_USE_VERTEXAI"); useVertex != "" {
		if c.Providers[ProviderGemini] == nil {
			c.Providers[ProviderGemini] = &ProviderConfig{}
		}
		c.Providers[ProviderGemini].UseVertex = true
		c.Providers[ProviderGemini].Enabled = true
		
		if project := os.Getenv("GOOGLE_CLOUD_PROJECT"); project != "" {
			c.Providers[ProviderGemini].Project = project
		}
		
		if location := os.Getenv("GOOGLE_CLOUD_LOCATION"); location != "" {
			c.Providers[ProviderGemini].Location = location
		}
	}
	
	// Default provider
	if provider := os.Getenv("GOMINI_DEFAULT_PROVIDER"); provider != "" {
		c.DefaultProvider = providers.ProviderType(provider)
	}
	
	// Router strategy
	if strategy := os.Getenv("GOMINI_ROUTER_STRATEGY"); strategy != "" {
		if c.Router == nil {
			c.Router = &RouterConfig{}
		}
		c.Router.Strategy = RouterStrategy(strategy)
	}
	
	// Cost optimization
	if costOpt := os.Getenv("GOMINI_COST_OPTIMIZED"); costOpt != "" {
		if c.Router == nil {
			c.Router = &RouterConfig{}
		}
		c.Router.CostOptimized = strings.ToLower(costOpt) == "true"
	}
	
	// Debug mode
	if debug := os.Getenv("GOMINI_DEBUG"); debug != "" {
		c.Debug = strings.ToLower(debug) == "true"
	}
	
	// Request timeout
	if timeout := os.Getenv("GOMINI_REQUEST_TIMEOUT"); timeout != "" {
		if duration, err := time.ParseDuration(timeout); err == nil {
			c.RequestTimeout = duration
		}
	}
	
	// Max retries
	if retries := os.Getenv("GOMINI_MAX_RETRIES"); retries != "" {
		if maxRetries, err := strconv.Atoi(retries); err == nil {
			c.MaxRetries = maxRetries
		}
	}
	
	// Session management settings
	if maxTurns := os.Getenv("GOMINI_MAX_SESSION_TURNS"); maxTurns != "" {
		if turns, err := strconv.Atoi(maxTurns); err == nil {
			c.MaxSessionTurns = turns
		}
	}
	
	if skipCheck := os.Getenv("GOMINI_SKIP_NEXT_SPEAKER_CHECK"); skipCheck != "" {
		c.SkipNextSpeakerCheck = strings.ToLower(skipCheck) == "true"
	}
	
	if loopDetection := os.Getenv("GOMINI_LOOP_DETECTION_ENABLED"); loopDetection != "" {
		c.LoopDetectionEnabled = strings.ToLower(loopDetection) == "true"
	}
	
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("no providers configured")
	}
	
	enabledProviders := 0
	for providerType, config := range c.Providers {
		if !config.Enabled {
			continue
		}
		enabledProviders++
		
		// Validate provider-specific config
		switch providerType {
		case ProviderOpenAI:
			if config.APIKey == "" {
				return fmt.Errorf("OpenAI API key is required")
			}
		case ProviderGemini:
			if !config.UseVertex && config.APIKey == "" {
				return fmt.Errorf("Gemini API key is required (unless using Vertex AI)")
			}
			if config.UseVertex && (config.Project == "" || config.Location == "") {
				return fmt.Errorf("Vertex AI requires both project and location")
			}
		}
	}
	
	if enabledProviders == 0 {
		return fmt.Errorf("no enabled providers found")
	}
	
	// Set default provider if not specified
	if c.DefaultProvider == "" {
		for providerType, config := range c.Providers {
			if config.Enabled {
				c.DefaultProvider = providerType
				break
			}
		}
	}
	
	// Validate default provider is enabled
	if defaultConfig, exists := c.Providers[c.DefaultProvider]; !exists || !defaultConfig.Enabled {
		return fmt.Errorf("default provider %s is not enabled", c.DefaultProvider)
	}
	
	return nil
}

// GetProviderConfig returns the configuration for a specific provider
func (c *Config) GetProviderConfig(provider providers.ProviderType) (*ProviderConfig, error) {
	config, exists := c.Providers[provider]
	if !exists {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}
	
	if !config.Enabled {
		return nil, fmt.Errorf("provider %s is disabled", provider)
	}
	
	return config, nil
}

// GetEnabledProviders returns a list of enabled providers
func (c *Config) GetEnabledProviders() []providers.ProviderType {
	var enabled []providers.ProviderType
	for providerType, config := range c.Providers {
		if config.Enabled {
			enabled = append(enabled, providerType)
		}
	}
	return enabled
}

// HasProvider checks if a provider is configured and enabled
func (c *Config) HasProvider(provider providers.ProviderType) bool {
	config, exists := c.Providers[provider]
	return exists && config.Enabled
}