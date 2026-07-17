package tui

import (
	"fmt"
	"sync/atomic"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"

	"github.com/sglavoie/tt/internal/aiharness"
	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

const (
	askFocusQuery = iota
	askFocusHarness
	askFocusConfigDir
	askFocusCount
)

var askOperationIDs atomic.Uint64

// askWorkspaceLoader materializes task data for an active-project snapshot
// captured on the UI event path. It deliberately returns project models rather
// than formatted context so each request can apply its selected disclosure mode
// after the snapshot is safely available.
type askWorkspaceLoader func([]*models.Project) ([]*models.Project, error)

// nextAskOperationID returns a process-wide identity for asynchronous Ask
// work. Zero remains the unscheduled sentinel on each model.
func nextAskOperationID() uint64 {
	for {
		current := askOperationIDs.Load()
		if current == ^uint64(0) {
			panic("Ask operation ID space exhausted")
		}
		if askOperationIDs.CompareAndSwap(current, current+1) {
			return current + 1
		}
	}
}

// AskViewModel implements the AI assistant view.
type AskViewModel struct {
	state       askState
	textarea    textarea.Model
	configInput textinput.Model
	viewport    viewport.Model
	spinner     SpinnerModel
	// project is retained exclusively for the focused, single-project
	// compatibility constructors below. Production Ask views use
	// workspaceLoader and must not inherit the dashboard's potentially stale
	// current project.
	project         *models.Project
	workspaceLoader askWorkspaceLoader
	width           int
	height          int
	ready           bool
	response        string
	raw             string
	prepared        *app.PreparedActionPlan
	cancelFn        func()
	pendingQuery    string
	inputError      string
	acceptedSafe    bool
	contextMode     external.TaskContextMode
	harness         aiharness.Name
	configDir       string
	focusIndex      int
	configDrafts    map[aiharness.Name]string
	runtime         *config.RuntimeState
	service         *app.Service
	runner          external.Runner
	available       func(aiharness.Name) bool

	// Request-failure reuse keeps the exact query and context mode that crossed
	// the provider boundary. Reuse only restores these values to the input form;
	// it never starts a provider request on its own.
	requestQuery       string
	requestContextMode external.TaskContextMode
	failureQuery       string
	failureContextMode external.TaskContextMode
	reuseAvailable     bool

	// CLI reference for selective injection
	cliReference string

	// Markdown renderer for prettifying responses
	mdRenderer *glamour.TermRenderer

	// Streaming state
	stream           external.Stream
	requestID        uint64
	preferenceSaveID uint64
	consentSaveID    uint64
	applyID          uint64
}

// NewAskViewModel creates a new AI assistant view.
func NewAskViewModel(project *models.Project, cliReference string) *AskViewModel {
	return NewAskViewModelWithDeps(project, cliReference, nil, nil)
}

// NewAskViewModelWithDeps creates the transactional, consent-aware assistant
// view over the shell's runtime snapshot and application service.
func NewAskViewModelWithDeps(project *models.Project, cliReference string, runtime *config.RuntimeState, service *app.Service) *AskViewModel {
	return NewAskViewModelWithRunner(project, cliReference, runtime, service, external.DefaultRunner)
}

// NewAskViewModelWithRunner creates an assistant view with an injected AI
// process boundary. Production callers normally use NewAskViewModelWithDeps;
// tests use this constructor to drive streaming without an installed provider.
func NewAskViewModelWithRunner(project *models.Project, cliReference string, runtime *config.RuntimeState, service *app.Service, runner external.Runner) *AskViewModel {
	return newAskViewModel(project, nil, cliReference, runtime, service, runner)
}

// NewAskViewModelForWorkspace creates the production Ask view. It intentionally
// does not accept the shell's current project: every consented contextual query
// loads a fresh snapshot of all active projects after submission instead.
func NewAskViewModelForWorkspace(cliReference string, runtime *config.RuntimeState, service *app.Service, persister Persister) *AskViewModel {
	return NewAskViewModelForWorkspaceWithRunner(cliReference, runtime, service, persister, external.DefaultRunner)
}

// NewAskViewModelForWorkspaceWithRunner exposes the production workspace
// factory with an injected process boundary for tests.
func NewAskViewModelForWorkspaceWithRunner(cliReference string, runtime *config.RuntimeState, service *app.Service, persister Persister, runner external.Runner) *AskViewModel {
	return newAskViewModel(nil, newActiveAskWorkspaceLoader(persister), cliReference, runtime, service, runner)
}

func newAskViewModel(project *models.Project, workspaceLoader askWorkspaceLoader, cliReference string, runtime *config.RuntimeState, service *app.Service, runner external.Runner) *AskViewModel {
	a := textarea.New()
	a.Placeholder = "Ask about your tasks... (Ctrl+S to submit)"
	a.CharLimit = 500
	a.SetHeight(3)
	a.ShowLineNumbers = false
	a.SetPromptFunc(0, func(info textarea.PromptInfo) string { return "" })
	a.Focus()
	applyThemedTextarea(&a)

	dirInput := textinput.New()
	dirInput.Placeholder = "empty clears the machine override"
	dirInput.CharLimit = 1024
	dirInput.Blur()
	applyThemedTextInput(&dirInput)

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(0),
	)

	harness := aiharness.Default
	configDrafts := make(map[aiharness.Name]string, len(aiharness.Names()))
	if runtime != nil {
		machine := runtime.Machine()
		harness = machine.GetAIHarness()
		for _, name := range aiharness.Names() {
			configDrafts[name] = machine.GetAIConfigDir(name)
		}
	} else {
		for _, name := range aiharness.Names() {
			configDrafts[name] = ""
		}
	}
	configDir := configDrafts[harness]
	dirInput.SetValue(configDir)

	return &AskViewModel{
		state:           askStateInput,
		textarea:        a,
		configInput:     dirInput,
		spinner:         NewSpinner(),
		project:         project,
		workspaceLoader: workspaceLoader,
		cliReference:    cliReference,
		mdRenderer:      renderer,
		contextMode:     external.TaskContextFull,
		harness:         harness,
		configDir:       configDir,
		focusIndex:      askFocusQuery,
		configDrafts:    configDrafts,
		runtime:         runtime,
		service:         service,
		runner:          runner,
		available:       external.HarnessAvailable,
	}
}

// newActiveAskWorkspaceLoader captures the persister dependency now and reads
// task files only when its returned function runs as a Tea command. The active
// project list is provided by submitQuery, which snapshots runtime state on the
// UI event path. A failed active project rejects the entire contextual request
// so the provider never receives a partial or misleading workspace snapshot.
func newActiveAskWorkspaceLoader(persister Persister) askWorkspaceLoader {
	return func(projects []*models.Project) ([]*models.Project, error) {
		if persister == nil {
			return nil, fmt.Errorf("project loader is unavailable")
		}

		for _, project := range projects {
			if project == nil {
				continue
			}
			if err := persister.Load(project); err != nil {
				return nil, fmt.Errorf("load active project %q: %w", project.Name, err)
			}
		}
		return projects, nil
	}
}

// Init implements tea.Model.
func (m *AskViewModel) Init() tea.Cmd {
	return textarea.Blink
}
