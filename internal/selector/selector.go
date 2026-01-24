// Package selector provides shared types for interactive selection UI.
// This package decouples CLI commands from TUI implementation details.
package selector

// Item represents an item that can be selected in an interactive selector.
type Item struct {
	ID      string
	Data    any
	Columns []string
	Headers []string // Column header names (only first item's headers are used)
}
