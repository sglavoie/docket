package tui

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
)

// TaskComposerPresetKind is the typed intent carried into the one task composer
// overlay. Presentation names from the retired capture/form family never decide
// behavior inside the composer.
type TaskComposerPresetKind uint8

const (
	ComposerGlobal TaskComposerPresetKind = iota
	ComposerProject
	ComposerChild
)

type TaskComposerMode uint8

const (
	ComposerCompact TaskComposerMode = iota
	ComposerExpanded
)

// TaskComposerSharedAttributes are defaults shared by every nonblank compact
// line. Explicit capture syntax on a line wins over these defaults.
type TaskComposerSharedAttributes struct {
	Priority    int
	Tags        []string
	Due         *time.Time
	ParentID    *string
	Destination app.CaptureDestination
}

// TaskComposerPreset is the complete typed launch contract for compact create,
// expanded create/edit, children, and links.
type TaskComposerPreset struct {
	Kind TaskComposerPresetKind
	// HeaderLabel overrides the kind's generic subtitle. Capture uses it to name
	// the destination project it resolved, because base capture and header-row
	// capture reach the same overlay with the same kind and would otherwise be
	// indistinguishable until the task landed somewhere unexpected.
	HeaderLabel        string
	Project            *models.Project
	AllProjects        []*models.Project
	Shared             TaskComposerSharedAttributes
	InitialText        string
	AllowProjectSwitch bool
	PreferMultiline    bool
}

func (p TaskComposerPreset) label() string {
	if p.HeaderLabel != "" {
		return p.HeaderLabel
	}
	switch p.Kind {
	case ComposerGlobal:
		return "Global capture"
	case ComposerProject:
		return "Project task"
	case ComposerChild:
		return "Child task"
	default:
		return "Task"
	}
}

func (p TaskComposerPreset) defaultDestination(
	parentID *string,
) app.CaptureDestination {
	if parentID != nil {
		return app.CaptureDestinationBacklog
	}
	if p.Shared.Destination != "" {
		return p.Shared.Destination
	}
	return app.CaptureDestinationInbox
}

// TaskComposerModel is the sole live task creation overlay. Compact mode owns
// one-or-many capture lines; expanded mode embeds the full form for one task.
type lineError struct {
	line    int
	message string
}

type TaskComposerModel struct {
	workspacePulseState
	quickCaptureInput
	preset             TaskComposerPreset
	runtime            *config.RuntimeState
	mode               TaskComposerMode
	expanded           *TaskFormModel
	capture            CaptureCoordinator
	taskForm           TaskFormCoordinator
	errors             []lineError
	width              int
	height             int
	submissionInFlight bool
	discardConfirmState
}

func NewTaskComposerModel(preset TaskComposerPreset, runtime *config.RuntimeState) *TaskComposerModel {
	m := &TaskComposerModel{
		preset:  preset,
		runtime: runtime,
		mode:    ComposerCompact,
		width:   80,
		height:  22,
	}

	m.initCompactInput()
	return m
}

func (m *TaskComposerModel) initCompactInput() {
	project := m.preset.Project
	placeholder := "Type a task title…"
	if m.preset.PreferMultiline {
		placeholder = "One task per line…"
	}
	m.quickCaptureInput = newQuickCaptureInput(project, m.preset.AllProjects, placeholder, !m.preset.AllowProjectSwitch)
	m.input.CharLimit = 4000
	m.input.SetHeight(3)
	if m.preset.InitialText != "" {
		m.input.SetValue(m.preset.InitialText)
		m.input.CursorEnd()
		m.updatePreview()
	}
	if project == nil {
		m.errors = []lineError{{line: 1, message: "composer preset requires a project"}}
	}
}

func (m *TaskComposerModel) SetCaptureCoordinator(c CaptureCoordinator) { m.capture = c }

func (m *TaskComposerModel) SetTaskFormCoordinator(c TaskFormCoordinator) {
	m.taskForm = c
	if m.expanded != nil {
		m.expanded.SetTaskFormCoordinator(c)
	}
}

func (m *TaskComposerModel) SetSize(width, height int) {
	m.width, m.height = width, height
	if m.expanded != nil {
		m.expanded.SetSize(width, height)
	}
	if m.mode == ComposerCompact {
		m.quickCaptureInput.width = max(width-2, 1)
		m.input.SetWidth(max(min(width-4, 90), 20))
		m.input.SetHeight(3)
		m.setSyntaxViewportWidth(max(width-2, 1))
	}
}

func (m *TaskComposerModel) setWorkspacePulse(pulse WorkspacePulse) {
	m.workspacePulseState.setWorkspacePulse(pulse)
	if m.expanded != nil {
		m.expanded.setWorkspacePulse(pulse)
	}
}

func (m *TaskComposerModel) Init() tea.Cmd {
	if m.expanded != nil {
		return m.expanded.Init()
	}
	return textarea.Blink
}

func (m *TaskComposerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case TaskCreatedMsg, status.ErrorMsg:
		m.submissionInFlight = false
	}

	if m.mode == ComposerExpanded {
		if m.expanded == nil {
			return m, nil
		}
		updated, cmd := m.expanded.Update(msg)
		if form, ok := updated.(*TaskFormModel); ok {
			m.expanded = form
		}
		return m, cmd
	}

	if m.confirmDiscard {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			switch key.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return status.BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
			}
		}
		return m, nil
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		name := key.String()
		if handled, cmd := m.handleComposerAutocompleteKey(name); handled {
			return m, cmd
		}
		if m.helpExpanded && !m.isAutocompleteActive() {
			switch name {
			case "ctrl+k":
				m.syntaxScrollLine(-1)
				return m, nil
			case "ctrl+j":
				m.syntaxScrollLine(1)
				return m, nil
			case "pgup":
				m.syntaxScrollPage(-1)
				return m, nil
			case "pgdown":
				m.syntaxScrollPage(1)
				return m, nil
			}
		}
		if name == "?" && m.ac.Mode == acModeNone {
			m.toggleHelp()
			return m, nil
		}
		switch name {
		case "esc":
			if m.helpExpanded {
				m.closeHelp()
				return m, nil
			}
			if strings.TrimSpace(m.input.Value()) != "" {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return status.BackMsg{} }
		case "shift+enter":
			m.input.InsertString("\n")
			m.updatePreview()
			return m, nil
		case "enter", "ctrl+s":
			if m.ac.Mode == acModeNone {
				return m, m.createCompact(false)
			}
		case "ctrl+e":
			if m.ac.Mode == acModeNone {
				return m, m.createCompact(true)
			}
		case "ctrl+f":
			return m, m.expandSingle()
		case "ctrl+p":
			m.cycleProject()
			return m, nil
		}
	}

	return m, m.updateInput(msg)
}

func (m *TaskComposerModel) handleComposerAutocompleteKey(name string) (bool, tea.Cmd) {
	if name == "enter" && m.autocompleteLiteralComplete() {
		return true, m.createCompact(false)
	}
	if m.handleAutocompleteKey(name) {
		return true, nil
	}
	return false, nil
}

func (m *TaskComposerModel) createCompact(openContent bool) tea.Cmd {
	if m.submissionInFlight {
		return nil
	}
	reqs, ok := m.parseCompactLines()
	if !ok {
		return nil
	}
	var cmd tea.Cmd
	if len(reqs) == 1 {
		cmd = m.capture.CaptureCreate(reqs[0], openContent)
	} else {
		cmd = m.capture.CaptureBulk(reqs, true)
	}
	if cmd != nil {
		m.submissionInFlight = true
	}
	return cmd
}

func (m *TaskComposerModel) parseCompactLines() ([]app.CaptureRequest, bool) {
	m.errors = nil
	if m.preset.Project == nil {
		m.errors = []lineError{{line: 1, message: "project is unavailable"}}
		return nil, false
	}

	lines := strings.Split(m.input.Value(), "\n")
	reqs := make([]app.CaptureRequest, 0, len(lines))
	for idx, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parsed, err := parseQuickCapture(line, m.preset.Project, m.preset.AllProjects, !m.preset.AllowProjectSwitch)
		if err != nil {
			m.errors = append(m.errors, lineError{line: idx + 1, message: err.Error()})
			continue
		}
		if err := models.ValidateTaskTitle(parsed.title); err != nil {
			m.errors = append(m.errors, lineError{line: idx + 1, message: err.Error()})
			continue
		}

		target := m.preset.Project
		if parsed.targetProjectID != nil {
			target = m.projectByID(*parsed.targetProjectID)
		}
		if target == nil {
			m.errors = append(m.errors, lineError{line: idx + 1, message: "target project is unavailable"})
			continue
		}

		priority := parsed.priority
		if m.preset.Shared.Priority != 0 && !parsed.prioritySet {
			priority = m.preset.Shared.Priority
		}
		parentID := parsed.parentID
		if parentID == nil {
			parentID = cloneStringPtr(m.preset.Shared.ParentID)
		}
		due := parsed.dueDate
		if due == nil {
			due = cloneTime(m.preset.Shared.Due)
		}
		destination := parsed.destination
		if !parsed.destinationSet {
			destination = m.preset.defaultDestination(parentID)
		}
		if dueDateIsToday(due) {
			destination = app.CaptureDestinationToday
		}
		destination = composerDeferredDestination(parsed, destination)
		req := app.CaptureRequest{
			SourceLine:   idx + 1,
			ProjectID:    target.ID,
			Title:        parsed.title,
			Priority:     priority,
			Tags:         mergeComposerTags(m.preset.Shared.Tags, parsed.tags),
			Context:      parsed.context,
			Due:          due,
			DeferSet:     parsed.deferSet,
			Defer:        parsed.deferUntil,
			ParentID:     parentID,
			ChildIDs:     parsed.childIDs(),
			Destination:  destination,
			Prepend:      true,
			Pinned:       parsed.pinned,
			Review:       parsed.review,
			Attachments:  parsed.attachments,
			Comments:     parsed.comments,
			Dependencies: parsed.dependencyLocators(),
		}
		if parentID != nil {
			if err := validateComposerParent(target, *parentID); err != nil {
				m.errors = append(m.errors, lineError{line: idx + 1, message: err.Error()})
				continue
			}
		}
		reqs = append(reqs, req)
	}

	if len(reqs) == 0 && len(m.errors) == 0 {
		m.errors = []lineError{{line: 1, message: "no tasks to create"}}
	}
	if len(m.errors) > 0 {
		return nil, false
	}
	return reqs, true
}
