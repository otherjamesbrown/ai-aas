package hello

import "testing"

func TestGreeting(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"basic name", "world", "Hello, World!"},
		{"trim spaces", "  jane  ", "Hello, Jane!"},
		{"empty name defaults", "", "Hello, there!"},
		{"single letter uppercases", "q", "Hello, Q!"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := Greeting(tc.input); got != tc.expected {
				t.Fatalf("Greeting(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}
