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

// NewItem creates a selector item with its column and header slices copied.
func NewItem(id string, data any, columns, headers []string) Item {
	return Item{
		ID:      id,
		Data:    data,
		Columns: append([]string(nil), columns...),
		Headers: append([]string(nil), headers...),
	}
}

// Clone returns a copy of the item with independent column and header slices.
func (i Item) Clone() Item {
	return NewItem(i.ID, i.Data, i.Columns, i.Headers)
}
