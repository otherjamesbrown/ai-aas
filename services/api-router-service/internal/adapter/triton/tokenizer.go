package triton

import (
	"fmt"
	"sync"

	"github.com/pkoukk/tiktoken-go"
)

// Tokenizer wraps tiktoken for token counting.
type Tokenizer struct {
	encoding *tiktoken.Tiktoken
	name     string
}

// tokenizersCache stores initialized tokenizers to avoid repeated initialization
var (
	tokenizersCache = make(map[string]*Tokenizer)
	tokenizersMu    sync.RWMutex
)

// SupportedEncodings lists all supported tokenizer encoding names
var SupportedEncodings = map[string]bool{
	"cl100k_base": true, // GPT-4, GPT-3.5-turbo, text-embedding-ada-002
	"o200k_base":  true, // GPT-4o, o1
	"llama3":      true, // Llama 3.x models (uses cl100k_base as approximation)
}

// NewTokenizer creates a new tokenizer for the given encoding.
// Supported encodings: cl100k_base, o200k_base, llama3
func NewTokenizer(encodingName string) (*Tokenizer, error) {
	if !SupportedEncodings[encodingName] {
		return nil, fmt.Errorf("unsupported tokenizer encoding: %s", encodingName)
	}

	// Check cache first
	tokenizersMu.RLock()
	if t, ok := tokenizersCache[encodingName]; ok {
		tokenizersMu.RUnlock()
		return t, nil
	}
	tokenizersMu.RUnlock()

	// Create new tokenizer
	tokenizersMu.Lock()
	defer tokenizersMu.Unlock()

	// Double-check after acquiring write lock
	if t, ok := tokenizersCache[encodingName]; ok {
		return t, nil
	}

	// Map encoding names to tiktoken encoding names
	tiktokenEncoding := encodingName
	if encodingName == "llama3" {
		// Llama 3 uses a BPE tokenizer similar to cl100k_base
		// This is an approximation - for exact counts, a llama-specific tokenizer would be needed
		tiktokenEncoding = "cl100k_base"
	}

	encoding, err := tiktoken.GetEncoding(tiktokenEncoding)
	if err != nil {
		return nil, fmt.Errorf("failed to get tiktoken encoding %s: %w", tiktokenEncoding, err)
	}

	t := &Tokenizer{
		encoding: encoding,
		name:     encodingName,
	}
	tokenizersCache[encodingName] = t
	return t, nil
}

// CountTokens counts the number of tokens in the given text.
func (t *Tokenizer) CountTokens(text string) int {
	if t.encoding == nil {
		return 0
	}
	tokens := t.encoding.Encode(text, nil, nil)
	return len(tokens)
}

// CountMessagesTokens counts tokens for a list of chat messages.
// This follows the OpenAI token counting convention for chat models.
func (t *Tokenizer) CountMessagesTokens(messages []ChatMessage) int {
	if t.encoding == nil || len(messages) == 0 {
		return 0
	}

	// Token counting for chat models includes message structure overhead
	// Reference: https://platform.openai.com/docs/guides/chat-completions/managing-tokens
	tokensPerMessage := 3 // every message follows <|start|>{role/name}\n{content}<|end|>\n
	tokensPerName := 1    // if there's a name, the role is omitted

	total := 0
	for _, msg := range messages {
		total += tokensPerMessage
		total += t.CountTokens(msg.Role)
		total += t.CountTokens(msg.Content)
		// Add name tokens if applicable (we don't track name in ChatMessage, so skip)
		_ = tokensPerName
	}

	// Every reply is primed with <|start|>assistant<|message|>
	total += 3

	return total
}

// EncodingName returns the name of the encoding used by this tokenizer.
func (t *Tokenizer) EncodingName() string {
	return t.name
}
