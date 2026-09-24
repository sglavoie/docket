package chrome

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/theme"
)

// SpinnerModel wraps the bubbles spinner for loading states.
type SpinnerModel struct {
	spinner spinner.Model
}

// NewSpinner creates a new spinner with the app's loading style.
func NewSpinner() SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.WarningIconStyle
	return SpinnerModel{
		spinner: s,
	}
}

// Init starts the spinner animation.
func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles spinner tick messages.
func (m SpinnerModel) Update(msg tea.Msg) (SpinnerModel, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View renders the spinner.
func (m SpinnerModel) View() string {
	return m.spinner.View()
}

// Tick returns the spinner's tick command for continuous animation.
func (m SpinnerModel) Tick() tea.Cmd {
	return m.spinner.Tick
}
