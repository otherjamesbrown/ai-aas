package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Prompter handles interactive prompts.
type Prompter struct {
	reader    io.Reader
	bufReader *bufio.Reader
	writer    io.Writer
	stdin     int // File descriptor for terminal operations
}

// Option configures the Prompter.
type Option func(*Prompter)

// WithReader sets a custom reader for input.
func WithReader(r io.Reader) Option {
	return func(p *Prompter) {
		p.reader = r
	}
}

// WithWriter sets a custom writer for output.
func WithWriter(w io.Writer) Option {
	return func(p *Prompter) {
		p.writer = w
	}
}

// New creates a new Prompter with the given options.
func New(opts ...Option) *Prompter {
	p := &Prompter{
		reader: os.Stdin,
		writer: os.Stdout,
		stdin:  int(os.Stdin.Fd()),
	}
	for _, opt := range opts {
		opt(p)
	}
	p.bufReader = bufio.NewReader(p.reader)
	return p
}

// Default is the default prompter using stdin/stdout.
var Default = New()

// Confirm prompts for yes/no confirmation.
// Returns true if the user responds with 'y' or 'yes'.
func (p *Prompter) Confirm(message string, defaultYes bool) (bool, error) {
	suffix := " [y/N]: "
	if defaultYes {
		suffix = " [Y/n]: "
	}

	fmt.Fprint(p.writer, message+suffix)

	input, err := p.bufReader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes, nil
	}

	return input == "y" || input == "yes", nil
}

// Confirm prompts for yes/no confirmation using the default prompter.
func Confirm(message string, defaultYes bool) (bool, error) {
	return Default.Confirm(message, defaultYes)
}

// ConfirmWithWord prompts the user to type a specific word to confirm.
// This is used for destructive operations where extra caution is needed.
func (p *Prompter) ConfirmWithWord(message, confirmWord string) (bool, error) {
	fmt.Fprintf(p.writer, "%s\n", message)
	fmt.Fprintf(p.writer, "Type '%s' to confirm: ", confirmWord)

	input, err := p.bufReader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	return strings.TrimSpace(input) == confirmWord, nil
}

// ConfirmWithWord prompts the user to type a specific word using the default prompter.
func ConfirmWithWord(message, confirmWord string) (bool, error) {
	return Default.ConfirmWithWord(message, confirmWord)
}

// Input prompts for text input with an optional default value.
func (p *Prompter) Input(message string, defaultValue string) (string, error) {
	if defaultValue != "" {
		fmt.Fprintf(p.writer, "%s [%s]: ", message, defaultValue)
	} else {
		fmt.Fprintf(p.writer, "%s: ", message)
	}

	input, err := p.bufReader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue, nil
	}

	return input, nil
}

// Input prompts for text input using the default prompter.
func Input(message string, defaultValue string) (string, error) {
	return Default.Input(message, defaultValue)
}

// InputRequired prompts for required text input (cannot be empty).
func (p *Prompter) InputRequired(message string) (string, error) {
	for {
		result, err := p.Input(message, "")
		if err != nil {
			return "", err
		}
		if result != "" {
			return result, nil
		}
		fmt.Fprintln(p.writer, "This field is required. Please enter a value.")
	}
}

// InputRequired prompts for required text input using the default prompter.
func InputRequired(message string) (string, error) {
	return Default.InputRequired(message)
}

// Password prompts for password input (hidden).
func (p *Prompter) Password(message string) (string, error) {
	fmt.Fprintf(p.writer, "%s: ", message)

	// Try to read password with hidden input
	password, err := term.ReadPassword(p.stdin)
	fmt.Fprintln(p.writer) // Add newline after hidden input

	if err != nil {
		// Fallback to regular input if terminal is not available
		input, readErr := p.bufReader.ReadString('\n')
		if readErr != nil {
			return "", fmt.Errorf("failed to read password: %w", readErr)
		}
		return strings.TrimSpace(input), nil
	}

	return string(password), nil
}

// Password prompts for password input using the default prompter.
func Password(message string) (string, error) {
	return Default.Password(message)
}

// SelectOption represents an option in a selection prompt.
type SelectOption struct {
	Label       string
	Value       string
	Description string
}

// Select prompts the user to select from a list of options.
func (p *Prompter) Select(message string, options []SelectOption) (SelectOption, error) {
	if len(options) == 0 {
		return SelectOption{}, fmt.Errorf("no options provided")
	}

	fmt.Fprintln(p.writer, message)
	for i, opt := range options {
		if opt.Description != "" {
			fmt.Fprintf(p.writer, "  %d) %s - %s\n", i+1, opt.Label, opt.Description)
		} else {
			fmt.Fprintf(p.writer, "  %d) %s\n", i+1, opt.Label)
		}
	}
	fmt.Fprint(p.writer, "Enter selection (1-", len(options), "): ")

	input, err := p.bufReader.ReadString('\n')
	if err != nil {
		return SelectOption{}, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	var selection int
	_, parseErr := fmt.Sscanf(input, "%d", &selection)
	if parseErr != nil || selection < 1 || selection > len(options) {
		return SelectOption{}, fmt.Errorf("invalid selection: %s", input)
	}

	return options[selection-1], nil
}

// Select prompts the user to select from options using the default prompter.
func Select(message string, options []SelectOption) (SelectOption, error) {
	return Default.Select(message, options)
}

// MultiSelect prompts the user to select multiple options.
func (p *Prompter) MultiSelect(message string, options []SelectOption) ([]SelectOption, error) {
	if len(options) == 0 {
		return nil, fmt.Errorf("no options provided")
	}

	fmt.Fprintln(p.writer, message)
	for i, opt := range options {
		if opt.Description != "" {
			fmt.Fprintf(p.writer, "  %d) %s - %s\n", i+1, opt.Label, opt.Description)
		} else {
			fmt.Fprintf(p.writer, "  %d) %s\n", i+1, opt.Label)
		}
	}
	fmt.Fprint(p.writer, "Enter selections (comma-separated, e.g., 1,3,4): ")

	input, err := p.bufReader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return []SelectOption{}, nil
	}

	parts := strings.Split(input, ",")
	var selected []SelectOption
	seen := make(map[int]bool)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		var selection int
		_, parseErr := fmt.Sscanf(part, "%d", &selection)
		if parseErr != nil || selection < 1 || selection > len(options) {
			return nil, fmt.Errorf("invalid selection: %s", part)
		}
		if !seen[selection] {
			seen[selection] = true
			selected = append(selected, options[selection-1])
		}
	}

	return selected, nil
}

// MultiSelect prompts the user to select multiple options using the default prompter.
func MultiSelect(message string, options []SelectOption) ([]SelectOption, error) {
	return Default.MultiSelect(message, options)
}
