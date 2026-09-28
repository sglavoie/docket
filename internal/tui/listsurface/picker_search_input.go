package listsurface

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// PickerSearchInput provides cursor editing and a horizontal viewport for
// compact pickers. The host still owns the query, filtering, and Enter/Esc
// cascade; synchronizing an unchanged query preserves the editing position.
// Its zero value is ready to use.
type PickerSearchInput struct {
	input textinput.Model
	ready bool
}

func (s *PickerSearchInput) sync(raw string) {
	if !s.ready {
		s.input = textinput.New()
		s.input.Prompt = "Search: "
		// These overlays route keys only, so clipboard commands cannot deliver
		// their result back to this input. Keep the existing paste behavior.
		s.input.KeyMap.Paste.SetEnabled(false)
		s.ready = true
	}
	if s.input.Value() != raw {
		s.input.SetValue(raw)
		s.input.CursorEnd()
	}
}

// HandleKey edits the query. Hosts intercept list navigation and commit/cancel
// keys before calling this method.
func (s *PickerSearchInput) HandleKey(raw string, msg tea.KeyPressMsg) string {
	s.sync(raw)
	s.style()
	s.input.Focus()
	s.input, _ = s.input.Update(msg)
	return s.input.Value()
}

func (s *PickerSearchInput) style() {
	theme.ApplyThemedTextInput(&s.input)
	styles := s.input.Styles()
	// A static virtual caret needs no timer routing through the overlay host.
	styles.Cursor.Blink = false
	s.input.SetStyles(styles)
}

// View keeps the active caret in the available columns, including after a
// resize. Accepted queries remain a compact, non-editable summary.
func (s *PickerSearchInput) View(raw string, active bool, width int) string {
	s.sync(raw)
	if !active {
		return theme.MutedStyle.Render(textfit.TruncateToVisualWidth("Search: "+raw, width))
	}
	s.style()
	s.input.Focus()
	// The input's cursor cell is drawn in addition to its scroll window.
	s.input.SetWidth(max(width-lipgloss.Width(s.input.Prompt)-1, 1))
	s.input.SetCursor(s.input.Position())
	return textfit.TruncateToVisualWidth(s.input.View(), width)
}
