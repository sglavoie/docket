package tui

import (
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
)

// formFactory is a shared factory instance for resizing form fields.
var formFactory = DefaultFormFieldFactory()

// FormFieldFactory creates consistently configured input fields.
// Use this to ensure uniform appearance and behavior across forms.
type FormFieldFactory struct {
	// Default dimensions
	DefaultWidth     int
	DefaultHeight    int
	DefaultCharLimit int

	// Title field settings
	TitleWidth     int
	TitleHeight    int
	TitleCharLimit int

	// Description field settings
	DescriptionHeight int
}

// DefaultFormFieldFactory returns a factory with standard settings.
func DefaultFormFieldFactory() *FormFieldFactory {
	return &FormFieldFactory{
		DefaultWidth:      50,
		DefaultHeight:     1,
		DefaultCharLimit:  500,
		TitleWidth:        68, // fits within 80-char card (80 - 4 border/padding - 6 content margin - 2 field margin)
		TitleHeight:       3,
		TitleCharLimit:    200,
		DescriptionHeight: 8,
	}
}

// NewTitleInput creates a textarea for task/project titles.
// Uses full available width to ensure natural word wrapping at container edge.
func (f *FormFieldFactory) NewTitleInput(placeholder string) textarea.Model {
	ti := textarea.New()
	ti.Placeholder = placeholder
	ti.CharLimit = f.TitleCharLimit
	ti.SetWidth(f.TitleWidth)
	ti.SetHeight(f.TitleHeight)
	ti.ShowLineNumbers = false
	return ti
}

// NewTitleInputWithValue creates a title input with an initial value.
func (f *FormFieldFactory) NewTitleInputWithValue(value string) textarea.Model {
	ti := f.NewTitleInput("")
	ti.SetValue(value)
	return ti
}

// NewTextInput creates a single-line text input.
func (f *FormFieldFactory) NewTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = f.DefaultCharLimit
	ti.SetWidth(f.DefaultWidth)
	applyThemedTextInput(&ti)
	return ti
}

// NewTextInputFocused creates a focused single-line text input.
func (f *FormFieldFactory) NewTextInputFocused(placeholder string) textinput.Model {
	ti := f.NewTextInput(placeholder)
	ti.Focus()
	return ti
}

// NewTextInputWithValue creates a text input with an initial value.
func (f *FormFieldFactory) NewTextInputWithValue(value, placeholder string) textinput.Model {
	ti := f.NewTextInput(placeholder)
	ti.SetValue(value)
	return ti
}

// ResizeTextarea adjusts a textarea to fit available width.
// The calculation accounts for card structure:
// - Card width: min(availableWidth - 4, 80)
// - lipgloss Width() value: cardWidth - 2 (border is outside Width)
// - Usable text area: Width - 4 (padding is inside Width)
// - Subtract 1 more for cursor to prevent overflow at line edge
func (f *FormFieldFactory) ResizeTextarea(ta *textarea.Model, availableWidth int) {
	cardWidth := min(availableWidth-4, 80)
	lipglossWidth := cardWidth - 2
	textWidth := lipglossWidth - 4 - 1
	width := max(textWidth, 30)
	ta.SetWidth(width)
}

// ResizeTextInput adjusts a text input to fit available width.
// Uses same calculation as ResizeTextarea for consistency.
func (f *FormFieldFactory) ResizeTextInput(ti *textinput.Model, availableWidth int) {
	cardWidth := min(availableWidth-4, 80)
	lipglossWidth := cardWidth - 2
	textWidth := lipglossWidth - 4
	width := max(textWidth, 30)
	ti.SetWidth(width)
}

// ResizeTextInputWithReserve adjusts a single-line input that shares a row with
// fixed labels or indentation. Bubbles textinput renders prompt/cursor chrome
// outside the configured width, so reserve a small extra margin before clamping.
func (f *FormFieldFactory) ResizeTextInputWithReserve(ti *textinput.Model, availableWidth, reservedColumns int) {
	const textInputChromeWidth = 3

	cardWidth := min(availableWidth-4, 80)
	lipglossWidth := cardWidth - 2
	textWidth := lipglossWidth - 4
	budget := max(availableWidth-reservedColumns-textInputChromeWidth, 1)
	width := min(textWidth, budget)
	ti.SetWidth(max(width, 1))
}
