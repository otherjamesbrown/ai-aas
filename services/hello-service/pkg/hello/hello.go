package hello

import (
	"fmt"
	"strings"
)

// Greeting returns a friendly greeting for the provided name.
// Names are trimmed and capitalized. Empty names default to "there".
func Greeting(name string) string {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return "Hello, there!"
	}

	return fmt.Sprintf("Hello, %s!", capitalize(clean))
}

func capitalize(word string) string {
	if word == "" {
		return word
	}
	if len(word) == 1 {
		return strings.ToUpper(word)
	}
	return strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
}
