package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
)

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
		TitleWidth:        50,
		TitleHeight:       2,
		TitleCharLimit:    200,
		DescriptionHeight: 8,
	}
}

// NewTitleInput creates a textarea for task/project titles.
func (f *FormFieldFactory) NewTitleInput(placeholder string) textarea.Model {
	ti := textarea.New()
	ti.Placeholder = placeholder
	ti.CharLimit = f.TitleCharLimit
	ti.SetWidth(f.TitleWidth)
	ti.SetHeight(f.TitleHeight)
	ti.ShowLineNumbers = false
	return ti
}

// NewTitleInputFocused creates a focused title input.
func (f *FormFieldFactory) NewTitleInputFocused(placeholder string) textarea.Model {
	ti := f.NewTitleInput(placeholder)
	ti.Focus()
	return ti
}

// NewTitleInputWithValue creates a title input with an initial value.
func (f *FormFieldFactory) NewTitleInputWithValue(value string) textarea.Model {
	ti := f.NewTitleInput("")
	ti.SetValue(value)
	return ti
}

// NewDescriptionInput creates a textarea for descriptions.
func (f *FormFieldFactory) NewDescriptionInput(placeholder string) textarea.Model {
	ti := textarea.New()
	ti.Placeholder = placeholder
	ti.CharLimit = f.DefaultCharLimit
	ti.SetWidth(f.DefaultWidth)
	ti.SetHeight(f.DescriptionHeight)
	ti.ShowLineNumbers = false
	return ti
}

// NewDescriptionInputWithValue creates a description input with an initial value.
func (f *FormFieldFactory) NewDescriptionInputWithValue(value string) textarea.Model {
	ti := f.NewDescriptionInput("")
	ti.SetValue(value)
	return ti
}

// NewTextInput creates a single-line text input.
func (f *FormFieldFactory) NewTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = f.DefaultCharLimit
	ti.Width = f.DefaultWidth
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

// UpdateDimensions adjusts input widths based on available space.
// Call this when the view is resized.
func (f *FormFieldFactory) UpdateDimensions(availableWidth int) {
	// Apply responsive sizing with constraints
	maxWidth := min(availableWidth-20, 80)
	minWidth := 30

	f.DefaultWidth = max(min(maxWidth, f.DefaultWidth), minWidth)
	f.TitleWidth = max(min(maxWidth, f.TitleWidth), minWidth)
}

// ResizeTextarea adjusts a textarea to fit available width.
func (f *FormFieldFactory) ResizeTextarea(ta *textarea.Model, availableWidth int) {
	width := min(availableWidth-20, 80)
	width = max(width, 30)
	ta.SetWidth(width)
}

// ResizeTextInput adjusts a text input to fit available width.
func (f *FormFieldFactory) ResizeTextInput(ti *textinput.Model, availableWidth int) {
	width := min(availableWidth-20, 80)
	width = max(width, 30)
	ti.Width = width
}
