package tui

import (
	"image/color"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// ProjectFocusViewModel displays active-flagged tasks for a single project.
type ProjectFocusViewModel struct {
	BaseView
	InteractiveViewState
	FilterMixin

	project *models.Project
	cfg     *config.Config

	// Tasks
	tasks []*models.Task

	// Sub-tabs
	subTab           int
	subTabRowFocused bool
	subTabNames      []string
}

// ProjectFocusNavigationPrefixHints contains "g" prefix hints for project focus view.
var ProjectFocusNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
}

// SetSize satisfies the View interface.
func (m *ProjectFocusViewModel) SetSize(width, height int) {
	m.BaseView.SetSize(width, height)
	m.resetViewScrollOffset()
}

func (m *ProjectFocusViewModel) resetViewScrollOffset() {
	m.viewScrollOffset = 0
}

// SetAccentColor satisfies the AccentColorSetter interface.
func (m *ProjectFocusViewModel) SetAccentColor(c color.Color) { m.accentColor = c }

// AccentColor returns the current accent color.
func (m *ProjectFocusViewModel) AccentColor() color.Color {
	if m.accentColor != nil {
		return m.accentColor
	}
	return ColorPrimary
}

// IsFullHeight returns true — project focus renders its own full-height content.
func (m *ProjectFocusViewModel) IsFullHeight() bool { return true }

// HasTextInput returns false — no text input is active.
func (m *ProjectFocusViewModel) HasTextInput() bool { return false }

// HasPendingPrefix returns true while a multi-key prefix sequence is active.
func (m *ProjectFocusViewModel) HasPendingPrefix() bool {
	return m.InteractiveViewState.HasPendingPrefix()
}
