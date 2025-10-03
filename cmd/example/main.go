package main

import (
	"context"
	"fmt"
	"log"

	"gomini/pkg/core"
	"gomini/pkg/gomini"
)

func main() {
	// Create client from environment variables
	client, err := core.NewClientFromEnv()
	if err != nil {
		log.Fatal("Failed to create client:", err)
	}
	defer client.Close()

	// List available providers and models
	fmt.Printf("Enabled providers: %v\n", client.GetEnabledProviders())

	models, err := client.ListModels(context.Background())
	if err != nil {
		log.Printf("Failed to list models: %v\n", err)
	} else {
		fmt.Printf("Available models: %d\n", len(models))
		for _, model := range models {
			fmt.Printf("  - %s (%s) - %d context size\n", model.Name, model.Provider, model.ContextSize)
		}
	}

	// Example 1: Send a simple message
	fmt.Println("\n=== Example 1: Simple Message ===")
	response, err := client.SendMessage(context.Background(), &gomini.ChatRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Hello, how are you?"),
		},
		Model: "gpt-4o-mini", // Will auto-route to OpenAI
	})
	if err != nil {
		log.Printf("Failed to send message: %v\n", err)
	} else {
		fmt.Printf("Response from %s: %v\n", response.Provider, response.Choices[0])
	}

	// Example 2: Stream a message from current provider
	fmt.Println("\n=== Example 2: Streaming Response ===")
	streamChan := client.SendMessageStream(context.Background(), &gomini.ChatRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Write a short haiku about Go programming"),
		},
		Model: "gpt-4o-mini", // Use available provider
	}, "example-prompt-1")

	fmt.Print("Streaming response: ")
	for event := range streamChan {
		switch event.Type {
		case gomini.EventContent:
			if contentData, ok := event.Data.(gomini.ContentEvent); ok {
				fmt.Print(contentData.Text)
			}
		case gomini.EventError:
			fmt.Printf("\nError: %v\n", event.Error)
		case gomini.EventFinished:
			fmt.Print("\n")
		}
	}

	// Example 3: JSON Generation
	fmt.Println("\n=== Example 3: JSON Generation ===")
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"language": map[string]interface{}{
				"type":        "string",
				"description": "The programming language",
			},
			"features": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Key features of the language",
			},
		},
		"required": []string{"language", "features"},
	}

	jsonResp, err := client.GenerateJSON(context.Background(), &gomini.JSONRequest{
		Messages: []gomini.Message{
			gomini.NewUserMessage("Describe Go programming language in the specified format"),
		},
		Model:  "gpt-4o-mini",
		Schema: schema,
	})
	if err != nil {
		log.Printf("Failed to generate JSON: %v\n", err)
	} else {
		fmt.Printf("JSON response from %s:\n", jsonResp.Provider)
		if jsonResp.Data != nil {
			for key, value := range jsonResp.Data {
				fmt.Printf("  %s: %v\n", key, value)
			}
		} else {
			fmt.Println("  No data returned")
		}
	}

	// Example 4: Current Provider Capabilities
	fmt.Println("\n=== Example 4: Current Provider Capabilities ===")
	currentProvider := client.GetCurrentProvider()
	currentProviderType := client.GetCurrentProviderType()
	
	if currentProvider != nil {
		capabilities := currentProvider.GetCapabilities()
		fmt.Printf("%s capabilities:\n", currentProviderType)
		fmt.Printf("  - Vision: %v\n", capabilities.SupportsVision)
		fmt.Printf("  - Functions: %v\n", capabilities.SupportsFunctions)
		fmt.Printf("  - JSON Mode: %v\n", capabilities.SupportsJSONMode)
		fmt.Printf("  - Max Context: %d tokens\n", capabilities.MaxContextSize)
		if len(capabilities.SpecificFeatures) > 0 {
			fmt.Printf("  - Special Features: %v\n", capabilities.SpecificFeatures)
		}
		fmt.Println()
	}
	
	// Show all available providers
	fmt.Println("Available providers in config:")
	for _, providerType := range client.GetAvailableProviders() {
		if providerType == currentProviderType {
			fmt.Printf("  - %s (current)\n", providerType)
		} else {
			fmt.Printf("  - %s\n", providerType)
		}
	}

	fmt.Println("Done! 🎉")
}