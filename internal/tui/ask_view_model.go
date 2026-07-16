package tui

import (
	"io"
	"os/exec"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// AskViewModel implements the AI assistant view.
type AskViewModel struct {
	state        askState
	textarea     textarea.Model
	viewport     viewport.Model
	spinner      SpinnerModel
	project      *models.Project
	width        int
	height       int
	ready        bool
	response     string
	raw          string
	prepared     *app.PreparedActionPlan
	cancelFn     func()
	pendingQuery string
	acceptedSafe bool
	contextMode  external.TaskContextMode
	runtime      *config.RuntimeState
	service      *app.Service

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
	return NewAskViewModelWithDeps(project, cliReference, nil, nil)
}

// NewAskViewModelWithDeps creates the transactional, consent-aware assistant
// view over the shell's runtime snapshot and application service.
func NewAskViewModelWithDeps(project *models.Project, cliReference string, runtime *config.RuntimeState, service *app.Service) *AskViewModel {
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
		contextMode:  external.TaskContextFull,
		runtime:      runtime,
		service:      service,
	}
}

// Init implements tea.Model.
func (m *AskViewModel) Init() tea.Cmd {
	return textarea.Blink
}
