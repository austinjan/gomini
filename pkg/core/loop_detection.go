package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"gomini/pkg/gomini"
)

// Constants from TypeScript version
const (
	TOOL_CALL_LOOP_THRESHOLD = 5
	CONTENT_LOOP_THRESHOLD   = 10
	CONTENT_CHUNK_SIZE       = 50
	MAX_HISTORY_LENGTH       = 1000
	
	// LLM-based loop detection constants (future use)
	LLM_LOOP_CHECK_HISTORY_COUNT = 20
	LLM_CHECK_AFTER_TURNS        = 30
	DEFAULT_LLM_CHECK_INTERVAL   = 3
	MIN_LLM_CHECK_INTERVAL       = 5
	MAX_LLM_CHECK_INTERVAL       = 15
)

// LoopDetectionService provides loop detection for conversations
// Based on the TypeScript implementation in packages/core/src/services/loopDetectionService.ts
type LoopDetectionService struct {
	mu       sync.RWMutex
	config   *gomini.Config
	promptID string

	// Tool call tracking
	lastToolCallKey          string
	toolCallRepetitionCount  int

	// Content streaming tracking
	streamContentHistory     string
	contentStats            map[string][]int  // hash -> indices
	lastContentIndex        int
	loopDetected            bool
	inCodeBlock             bool

	// LLM loop tracking (future use)
	turnsInCurrentPrompt    int
	llmCheckInterval        int
	lastCheckTurn           int
}

// NewLoopDetectionService creates a new loop detection service
func NewLoopDetectionService(config *gomini.Config) *LoopDetectionService {
	return &LoopDetectionService{
		config:              config,
		contentStats:        make(map[string][]int),
		llmCheckInterval:    DEFAULT_LLM_CHECK_INTERVAL,
	}
}

// Reset clears all loop detection state for a new prompt
func (l *LoopDetectionService) Reset(promptID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	l.promptID = promptID
	l.resetToolCallCount()
	l.resetContentTracking(true)
	l.resetLLMCheckTracking()
	l.loopDetected = false
}

// AddAndCheck processes a stream event and checks for loop conditions
// Returns true if a loop is detected
func (l *LoopDetectionService) AddAndCheck(event gomini.StreamEvent) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.loopDetected {
		return true
	}

	switch event.Type {
	case gomini.EventToolCall:
		// Content chanting only happens in one single stream, reset if there
		// is a tool call in between
		l.resetContentTracking(false)
		if toolCallData, ok := event.Data.(gomini.ToolCallEvent); ok {
			l.loopDetected = l.checkToolCallLoop(toolCallData)
		}
	case gomini.EventContent:
		if contentData, ok := event.Data.(gomini.ContentEvent); ok {
			l.loopDetected = l.checkContentLoop(contentData.Text)
		}
	}
	
	return l.loopDetected
}

// TurnStarted signals the start of a new turn in the conversation
// Returns true if a loop is detected (future LLM-based detection)
func (l *LoopDetectionService) TurnStarted(ctx context.Context) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	l.turnsInCurrentPrompt++
	
	// TODO: Implement LLM-based loop detection when needed
	// This would involve calling an LLM to analyze conversation history
	// for cognitive loops, similar to the TypeScript implementation
	
	return false
}

// IsLoopDetected returns whether a loop has been detected
func (l *LoopDetectionService) IsLoopDetected() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.loopDetected
}

// getToolCallKey generates a hash key for a tool call
func (l *LoopDetectionService) getToolCallKey(toolCall gomini.ToolCallEvent) string {
	// Create a deterministic string representation
	argsBytes, _ := json.Marshal(toolCall.Arguments)
	keyString := fmt.Sprintf("%s:%s", toolCall.ToolName, string(argsBytes))
	
	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(keyString))
	return hex.EncodeToString(hash[:])
}

// checkToolCallLoop detects loops in tool calls
func (l *LoopDetectionService) checkToolCallLoop(toolCall gomini.ToolCallEvent) bool {
	key := l.getToolCallKey(toolCall)
	
	if l.lastToolCallKey == key {
		l.toolCallRepetitionCount++
	} else {
		l.lastToolCallKey = key
		l.toolCallRepetitionCount = 1
	}
	
	if l.toolCallRepetitionCount >= TOOL_CALL_LOOP_THRESHOLD {
		// Log loop detection (placeholder for future telemetry)
		if l.config.Debug {
			fmt.Printf("Tool call loop detected: %s repeated %d times\n", 
				toolCall.ToolName, l.toolCallRepetitionCount)
		}
		return true
	}
	
	return false
}

// checkContentLoop detects loops in content using sliding window analysis
func (l *LoopDetectionService) checkContentLoop(content string) bool {
	// Different content elements can often contain repetitive syntax that is not indicative of a loop.
	// To avoid false positives, we detect when we encounter different content types and
	// reset tracking to avoid analyzing content that spans across different element boundaries.
	numFences := strings.Count(content, "```")
	hasTable := regexp.MustCompile(`(^|\n)\s*(\|.*\||[|+-]{3,})`).MatchString(content)
	hasListItem := regexp.MustCompile(`(^|\n)\s*[*-+]\s`).MatchString(content) || 
				  regexp.MustCompile(`(^|\n)\s*\d+\.\s`).MatchString(content)
	hasHeading := regexp.MustCompile(`(^|\n)#+\s`).MatchString(content)
	hasBlockquote := regexp.MustCompile(`(^|\n)>\s`).MatchString(content)
	isDivider := regexp.MustCompile(`^[+\-_=*]+$`).MatchString(content)

	if numFences > 0 || hasTable || hasListItem || hasHeading || hasBlockquote || isDivider {
		// Reset tracking when different content elements are detected
		l.resetContentTracking(false)
	}

	wasInCodeBlock := l.inCodeBlock
	l.inCodeBlock = (numFences%2 == 0) == l.inCodeBlock // Toggle if odd number of fences
	if wasInCodeBlock || l.inCodeBlock || isDivider {
		return false
	}

	l.streamContentHistory += content
	l.truncateAndUpdate()
	return l.analyzeContentChunksForLoop()
}

// truncateAndUpdate manages content history size
func (l *LoopDetectionService) truncateAndUpdate() {
	if len(l.streamContentHistory) <= MAX_HISTORY_LENGTH {
		return
	}

	// Calculate how much content to remove from the beginning
	truncationAmount := len(l.streamContentHistory) - MAX_HISTORY_LENGTH
	l.streamContentHistory = l.streamContentHistory[truncationAmount:]
	l.lastContentIndex = max(0, l.lastContentIndex-truncationAmount)

	// Update all stored chunk indices to account for the truncation
	for hash, oldIndices := range l.contentStats {
		adjustedIndices := make([]int, 0, len(oldIndices))
		for _, index := range oldIndices {
			adjustedIndex := index - truncationAmount
			if adjustedIndex >= 0 {
				adjustedIndices = append(adjustedIndices, adjustedIndex)
			}
		}

		if len(adjustedIndices) > 0 {
			l.contentStats[hash] = adjustedIndices
		} else {
			delete(l.contentStats, hash)
		}
	}
}

// analyzeContentChunksForLoop analyzes content in fixed-size chunks
func (l *LoopDetectionService) analyzeContentChunksForLoop() bool {
	for l.hasMoreChunksToProcess() {
		// Extract current chunk of text
		endIndex := l.lastContentIndex + CONTENT_CHUNK_SIZE
		if endIndex > len(l.streamContentHistory) {
			endIndex = len(l.streamContentHistory)
		}
		
		currentChunk := l.streamContentHistory[l.lastContentIndex:endIndex]
		chunkHash := l.hashChunk(currentChunk)

		if l.isLoopDetectedForChunk(currentChunk, chunkHash) {
			if l.config.Debug {
				fmt.Printf("Content loop detected: chunk repeated %d+ times\n", CONTENT_LOOP_THRESHOLD)
			}
			return true
		}

		// Move to next position in the sliding window
		l.lastContentIndex++
	}

	return false
}

// hasMoreChunksToProcess checks if there are more chunks to analyze
func (l *LoopDetectionService) hasMoreChunksToProcess() bool {
	return l.lastContentIndex+CONTENT_CHUNK_SIZE <= len(l.streamContentHistory)
}

// hashChunk generates a hash for a content chunk
func (l *LoopDetectionService) hashChunk(chunk string) string {
	hash := sha256.Sum256([]byte(chunk))
	return hex.EncodeToString(hash[:])
}

// isLoopDetectedForChunk determines if a content chunk indicates a loop pattern
func (l *LoopDetectionService) isLoopDetectedForChunk(chunk, hash string) bool {
	existingIndices, exists := l.contentStats[hash]

	if !exists {
		l.contentStats[hash] = []int{l.lastContentIndex}
		return false
	}

	// Verify actual content matches to prevent hash collisions
	if !l.isActualContentMatch(chunk, existingIndices[0]) {
		return false
	}

	existingIndices = append(existingIndices, l.lastContentIndex)
	l.contentStats[hash] = existingIndices

	if len(existingIndices) < CONTENT_LOOP_THRESHOLD {
		return false
	}

	// Analyze the most recent occurrences to see if they're clustered closely together
	recentIndices := existingIndices[len(existingIndices)-CONTENT_LOOP_THRESHOLD:]
	totalDistance := recentIndices[len(recentIndices)-1] - recentIndices[0]
	averageDistance := float64(totalDistance) / float64(CONTENT_LOOP_THRESHOLD-1)
	maxAllowedDistance := float64(CONTENT_CHUNK_SIZE) * 1.5

	return averageDistance <= maxAllowedDistance
}

// isActualContentMatch verifies that two chunks with the same hash actually contain identical content
func (l *LoopDetectionService) isActualContentMatch(currentChunk string, originalIndex int) bool {
	if originalIndex+CONTENT_CHUNK_SIZE > len(l.streamContentHistory) {
		return false
	}
	
	originalChunk := l.streamContentHistory[originalIndex : originalIndex+CONTENT_CHUNK_SIZE]
	return originalChunk == currentChunk
}

// resetToolCallCount resets tool call tracking
func (l *LoopDetectionService) resetToolCallCount() {
	l.lastToolCallKey = ""
	l.toolCallRepetitionCount = 0
}

// resetContentTracking resets content loop tracking
func (l *LoopDetectionService) resetContentTracking(resetHistory bool) {
	if resetHistory {
		l.streamContentHistory = ""
	}
	l.contentStats = make(map[string][]int)
	l.lastContentIndex = 0
}

// resetLLMCheckTracking resets LLM-based loop tracking
func (l *LoopDetectionService) resetLLMCheckTracking() {
	l.turnsInCurrentPrompt = 0
	l.llmCheckInterval = DEFAULT_LLM_CHECK_INTERVAL
	l.lastCheckTurn = 0
}

// Helper function for max (Go 1.18+ would have this in stdlib)
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}