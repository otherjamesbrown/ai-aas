package prompt

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirm(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultYes bool
		want       bool
	}{
		{"yes lowercase", "y\n", false, true},
		{"yes full", "yes\n", false, true},
		{"no lowercase", "n\n", false, false},
		{"no full", "no\n", false, false},
		{"empty with default no", "\n", false, false},
		{"empty with default yes", "\n", true, true},
		{"random input", "maybe\n", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(
				WithReader(strings.NewReader(tt.input)),
				WithWriter(&bytes.Buffer{}),
			)
			got, err := p.Confirm("Test?", tt.defaultYes)
			if err != nil {
				t.Fatalf("Confirm() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Confirm() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfirmWithWord(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		confirmWord string
		want        bool
	}{
		{"exact match", "DELETE\n", "DELETE", true},
		{"wrong word", "delete\n", "DELETE", false},
		{"empty", "\n", "DELETE", false},
		{"partial match", "DEL\n", "DELETE", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			p := New(
				WithReader(strings.NewReader(tt.input)),
				WithWriter(&out),
			)
			got, err := p.ConfirmWithWord("Are you sure?", tt.confirmWord)
			if err != nil {
				t.Fatalf("ConfirmWithWord() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("ConfirmWithWord() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInput(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		defaultValue string
		want         string
	}{
		{"with value", "hello\n", "", "hello"},
		{"empty with default", "\n", "default", "default"},
		{"override default", "override\n", "default", "override"},
		{"with whitespace", "  trimmed  \n", "", "trimmed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(
				WithReader(strings.NewReader(tt.input)),
				WithWriter(&bytes.Buffer{}),
			)
			got, err := p.Input("Enter value", tt.defaultValue)
			if err != nil {
				t.Fatalf("Input() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Input() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInputRequired(t *testing.T) {
	// Test that it eventually gets a value after empty inputs
	input := "\n\nfinally\n"
	var out bytes.Buffer
	p := New(
		WithReader(strings.NewReader(input)),
		WithWriter(&out),
	)

	got, err := p.InputRequired("Enter value")
	if err != nil {
		t.Fatalf("InputRequired() error = %v", err)
	}
	if got != "finally" {
		t.Errorf("InputRequired() = %v, want 'finally'", got)
	}

	// Check that required message was printed
	if !strings.Contains(out.String(), "required") {
		t.Error("expected 'required' message in output")
	}
}

func TestSelect(t *testing.T) {
	options := []SelectOption{
		{Label: "Option A", Value: "a"},
		{Label: "Option B", Value: "b", Description: "The B option"},
		{Label: "Option C", Value: "c"},
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"select first", "1\n", "a", false},
		{"select second", "2\n", "b", false},
		{"select third", "3\n", "c", false},
		{"invalid zero", "0\n", "", true},
		{"invalid high", "4\n", "", true},
		{"invalid text", "abc\n", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(
				WithReader(strings.NewReader(tt.input)),
				WithWriter(&bytes.Buffer{}),
			)
			got, err := p.Select("Choose:", options)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Select() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got.Value != tt.want {
				t.Errorf("Select() = %v, want %v", got.Value, tt.want)
			}
		})
	}
}

func TestSelect_NoOptions(t *testing.T) {
	p := New(
		WithReader(strings.NewReader("1\n")),
		WithWriter(&bytes.Buffer{}),
	)
	_, err := p.Select("Choose:", []SelectOption{})
	if err == nil {
		t.Error("expected error for empty options")
	}
}

func TestMultiSelect(t *testing.T) {
	options := []SelectOption{
		{Label: "Option A", Value: "a"},
		{Label: "Option B", Value: "b"},
		{Label: "Option C", Value: "c"},
	}

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"select one", "1\n", []string{"a"}, false},
		{"select multiple", "1,3\n", []string{"a", "c"}, false},
		{"select all", "1,2,3\n", []string{"a", "b", "c"}, false},
		{"empty selection", "\n", []string{}, false},
		{"with spaces", "1, 2, 3\n", []string{"a", "b", "c"}, false},
		{"duplicate ignored", "1,1,2\n", []string{"a", "b"}, false},
		{"invalid option", "1,4\n", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(
				WithReader(strings.NewReader(tt.input)),
				WithWriter(&bytes.Buffer{}),
			)
			got, err := p.MultiSelect("Choose:", options)
			if (err != nil) != tt.wantErr {
				t.Fatalf("MultiSelect() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("MultiSelect() returned %d items, want %d", len(got), len(tt.want))
				}
				for i, v := range got {
					if v.Value != tt.want[i] {
						t.Errorf("MultiSelect()[%d] = %v, want %v", i, v.Value, tt.want[i])
					}
				}
			}
		})
	}
}

func TestOutputFormat(t *testing.T) {
	var out bytes.Buffer
	p := New(
		WithReader(strings.NewReader("y\n")),
		WithWriter(&out),
	)

	_, _ = p.Confirm("Test message", false)

	if !strings.Contains(out.String(), "Test message") {
		t.Error("expected message in output")
	}
	if !strings.Contains(out.String(), "[y/N]") {
		t.Error("expected [y/N] in output for defaultYes=false")
	}
}
