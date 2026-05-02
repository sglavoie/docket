package tui

import (
	"io"
	"os/exec"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/sglavoie/tt/internal/models"
)

// AskViewModel implements the AI assistant view.
type AskViewModel struct {
	state    askState
	textarea textarea.Model
	viewport viewport.Model
	spinner  SpinnerModel
	project  *models.Project
	width    int
	height   int
	ready    bool
	response string
	commands []string
	cancelFn func()

	// CLI reference for selective injection
	cliReference string

	// Markdown renderer for prettifying responses
	mdRenderer *glamour.TermRenderer

	// Streaming state
	streamReader io.ReadCloser
	streamCmd    *exec.Cmd
}

// NewAskViewModel creates a new AI assistant view.
func NewAskViewModel(project *models.Project, cliReference string) *AskViewModel {
	a := textarea.New()
	a.Placeholder = "Ask about your tasks... (Ctrl+S to submit)"
	a.CharLimit = 500
	a.SetHeight(3)
	a.ShowLineNumbers = false
	a.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })
	a.Focus()
	applyThemedTextarea(&a)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(0),
	)

	return &AskViewModel{
		state:        askStateInput,
		textarea:     a,
		spinner:      NewSpinner(),
		project:      project,
		cliReference: cliReference,
		mdRenderer:   renderer,
	}
}

// Init implements tea.Model.
func (m *AskViewModel) Init() tea.Cmd {
	return textarea.Blink
}
