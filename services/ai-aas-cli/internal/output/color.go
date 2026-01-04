// Package output provides styled CLI output utilities.
package output

import (
	"fmt"
	"os"

	"github.com/fatih/color"
)

// Color instances for consistent styling throughout the CLI
var (
	// Status colors
	Success = color.New(color.FgGreen, color.Bold)
	Warning = color.New(color.FgYellow)
	Error   = color.New(color.FgRed, color.Bold)
	Info    = color.New(color.FgCyan)

	// Element colors
	Label     = color.New(color.FgWhite, color.Bold)
	Value     = color.New(color.FgWhite)
	Muted     = color.New(color.FgHiBlack)
	Highlight = color.New(color.FgMagenta, color.Bold)
	Bold      = color.New(color.Bold)

	// Status indicators
	CheckMark = Success.Sprint("✓")
	CrossMark = Error.Sprint("✗")
	Bullet    = Info.Sprint("•")
	Arrow     = Muted.Sprint("→")
)

// Symbols for status display
const (
	SymbolCheck = "✓"
	SymbolCross = "✗"
	SymbolDot   = "•"
	SymbolArrow = "→"
)

// Printf prints formatted colored output
func Printf(c *color.Color, format string, a ...interface{}) {
	c.Printf(format, a...)
}

// Println prints colored output with newline
func Println(c *color.Color, a ...interface{}) {
	c.Println(a...)
}

// SuccessMsg prints a success message with checkmark
func SuccessMsg(format string, a ...interface{}) {
	fmt.Printf("%s %s\n", CheckMark, fmt.Sprintf(format, a...))
}

// ErrorMsg prints an error message with cross mark
func ErrorMsg(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s %s\n", CrossMark, fmt.Sprintf(format, a...))
}

// WarningMsg prints a warning message
func WarningMsg(format string, a ...interface{}) {
	Warning.Printf("! %s\n", fmt.Sprintf(format, a...))
}

// InfoMsg prints an info message
func InfoMsg(format string, a ...interface{}) {
	fmt.Printf("%s %s\n", Bullet, fmt.Sprintf(format, a...))
}

// Header prints a styled header/section title
func Header(title string) {
	Label.Println(title)
	fmt.Println(Muted.Sprint("─────────────────────────────────────────"))
}

// KeyValue prints a key-value pair with styling
func KeyValue(key, value string) {
	fmt.Printf("  %s: %s\n", Label.Sprint(key), value)
}

// KeyValueStatus prints a key-value pair with a status indicator
func KeyValueStatus(key, value string, configured bool) {
	status := CrossMark
	if configured {
		status = CheckMark
	}
	fmt.Printf("  %s %s: %s\n", status, Label.Sprint(key), value)
}

// StatusBadge returns a colored status badge
func StatusBadge(status string) string {
	switch status {
	case "ready", "complete", "success", "healthy", "active":
		return Success.Sprint(status)
	case "pending", "waiting", "registered":
		return Warning.Sprint(status)
	case "failed", "error", "unhealthy":
		return Error.Sprint(status)
	case "cached", "downloading", "uploading", "deploying":
		return Info.Sprint(status)
	case "disabled":
		return Muted.Sprint(status)
	default:
		return status
	}
}

// ActionHint prints a hint for the next action
func ActionHint(action string) string {
	return Muted.Sprintf("→ %s", action)
}

// Table styling helpers
type TableStyle struct {
	HeaderColor *color.Color
	RowColors   []*color.Color
}

// DefaultTableStyle returns the default table styling
func DefaultTableStyle() *TableStyle {
	return &TableStyle{
		HeaderColor: Label,
		RowColors:   []*color.Color{Value, Muted},
	}
}

// Box prints content in a styled box
func Box(title, content string) {
	fmt.Println()
	Label.Printf("┌─ %s ", title)
	fmt.Println(Muted.Sprint("────────────────────────────────"))
	fmt.Println(content)
	fmt.Println(Muted.Sprint("└────────────────────────────────────────"))
}
