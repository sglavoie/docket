package taskform

import (
	"strings"
	"time"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/pulse"
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
type TaskComposerModel struct {
	pulse.WorkspacePulseState
	grammar.QuickCaptureInput
	Preset             TaskComposerPreset
	runtime            *config.RuntimeState
	Mode               TaskComposerMode
	Expanded           *TaskFormModel
	capture            coordinator.CaptureCoordinator
	taskForm           coordinator.TaskFormCoordinator
	Errors             []grammar.LineError
	width              int
	height             int
	SubmissionInFlight bool
	chrome.DiscardConfirmState
}

func NewTaskComposerModel(preset TaskComposerPreset, runtime *config.RuntimeState) *TaskComposerModel {
	m := &TaskComposerModel{
		Preset:  preset,
		runtime: runtime,
		Mode:    ComposerCompact,
		width:   80,
		height:  22,
	}

	m.initCompactInput()
	return m
}

func (m *TaskComposerModel) initCompactInput() {
	project := m.Preset.Project
	placeholder := "Type a task title…"
	if m.Preset.PreferMultiline {
		placeholder = "One task per line…"
	}
	m.QuickCaptureInput = grammar.NewQuickCaptureInput(project, m.Preset.AllProjects, placeholder, !m.Preset.AllowProjectSwitch)
	m.Input.CharLimit = 4000
	m.Input.SetHeight(3)
	if m.Preset.InitialText != "" {
		m.Input.SetValue(m.Preset.InitialText)
		m.Input.CursorEnd()
		m.UpdatePreview()
	}
	if project == nil {
		m.Errors = []grammar.LineError{{Line: 1, Message: "composer preset requires a project"}}
	}
}

func (m *TaskComposerModel) SetCaptureCoordinator(c coordinator.CaptureCoordinator) { m.capture = c }

func (m *TaskComposerModel) SetTaskFormCoordinator(c coordinator.TaskFormCoordinator) {
	m.taskForm = c
	if m.Expanded != nil {
		m.Expanded.SetTaskFormCoordinator(c)
	}
}

func (m *TaskComposerModel) SetSize(width, height int) {
	m.width, m.height = width, height
	if m.Expanded != nil {
		m.Expanded.SetSize(width, height)
	}
	if m.Mode == ComposerCompact {
		m.Width = max(width-2, 1)
		m.Input.SetWidth(max(min(width-4, 90), 20))
		m.Input.SetHeight(3)
		m.SetSyntaxViewportWidth(max(width-2, 1))
	}
}

func (m *TaskComposerModel) SetWorkspacePulse(workspacePulse pulse.WorkspacePulse) {
	m.WorkspacePulseState.SetWorkspacePulse(workspacePulse)
	if m.Expanded != nil {
		m.Expanded.SetWorkspacePulse(workspacePulse)
	}
}

func (m *TaskComposerModel) Init() tea.Cmd {
	if m.Expanded != nil {
		return m.Expanded.Init()
	}
	return textarea.Blink
}

func (m *TaskComposerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case coordinator.TaskCreatedMsg, status.ErrorMsg:
		m.SubmissionInFlight = false
	}

	if m.Mode == ComposerExpanded {
		if m.Expanded == nil {
			return m, nil
		}
		updated, cmd := m.Expanded.Update(msg)
		if form, ok := updated.(*TaskFormModel); ok {
			m.Expanded = form
		}
		return m, cmd
	}

	if m.ConfirmDiscard {
		if key, ok := msg.(tea.KeyPressMsg); ok {
			switch key.String() {
			case "y", "Y":
				m.ConfirmDiscard = false
				return m, func() tea.Msg { return status.BackMsg{} }
			case "n", "N", "esc":
				m.ConfirmDiscard = false
			}
		}
		return m, nil
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		name := key.String()
		if handled, cmd := m.handleComposerAutocompleteKey(name); handled {
			return m, cmd
		}
		if m.HelpExpanded && !m.IsAutocompleteActive() {
			switch name {
			case "ctrl+k":
				m.SyntaxScrollLine(-1)
				return m, nil
			case "ctrl+j":
				m.SyntaxScrollLine(1)
				return m, nil
			case "pgup":
				m.SyntaxScrollPage(-1)
				return m, nil
			case "pgdown":
				m.SyntaxScrollPage(1)
				return m, nil
			}
		}
		if name == "?" && m.AC.Mode == grammar.ACModeNone {
			m.ToggleHelp()
			return m, nil
		}
		switch name {
		case "esc":
			if m.HelpExpanded {
				m.CloseHelp()
				return m, nil
			}
			if strings.TrimSpace(m.Input.Value()) != "" {
				m.ConfirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg { return status.BackMsg{} }
		case "shift+enter":
			m.Input.InsertString("\n")
			m.UpdatePreview()
			return m, nil
		case "enter", "ctrl+s":
			if m.AC.Mode == grammar.ACModeNone {
				return m, m.CreateCompact(false)
			}
		case "ctrl+e":
			if m.AC.Mode == grammar.ACModeNone {
				return m, m.CreateCompact(true)
			}
		case "ctrl+f":
			return m, m.ExpandSingle()
		case "ctrl+p":
			m.cycleProject()
			return m, nil
		}
	}

	return m, m.UpdateInput(msg)
}

func (m *TaskComposerModel) handleComposerAutocompleteKey(name string) (bool, tea.Cmd) {
	if name == "enter" && m.AutocompleteLiteralComplete() {
		return true, m.CreateCompact(false)
	}
	if m.HandleAutocompleteKey(name) {
		return true, nil
	}
	return false, nil
}

func (m *TaskComposerModel) CreateCompact(openContent bool) tea.Cmd {
	if m.SubmissionInFlight {
		return nil
	}
	reqs, ok := m.ParseCompactLines()
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
		m.SubmissionInFlight = true
	}
	return cmd
}

func (m *TaskComposerModel) ParseCompactLines() ([]app.CaptureRequest, bool) {
	m.Errors = nil
	if m.Preset.Project == nil {
		m.Errors = []grammar.LineError{{Line: 1, Message: "project is unavailable"}}
		return nil, false
	}

	lines := strings.Split(m.Input.Value(), "\n")
	reqs := make([]app.CaptureRequest, 0, len(lines))
	for idx, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parsed, err := grammar.ParseQuickCapture(line, m.Preset.Project, m.Preset.AllProjects, !m.Preset.AllowProjectSwitch)
		if err != nil {
			m.Errors = append(m.Errors, grammar.LineError{Line: idx + 1, Message: err.Error()})
			continue
		}
		if err := models.ValidateTaskTitle(parsed.Title); err != nil {
			m.Errors = append(m.Errors, grammar.LineError{Line: idx + 1, Message: err.Error()})
			continue
		}

		target := m.Preset.Project
		if parsed.TargetProjectID != nil {
			target = m.projectByID(*parsed.TargetProjectID)
		}
		if target == nil {
			m.Errors = append(m.Errors, grammar.LineError{Line: idx + 1, Message: "target project is unavailable"})
			continue
		}

		priority := parsed.Priority
		if m.Preset.Shared.Priority != 0 && !parsed.PrioritySet {
			priority = m.Preset.Shared.Priority
		}
		parentID := parsed.ParentID
		if parentID == nil {
			parentID = cloneStringPtr(m.Preset.Shared.ParentID)
		}
		due := parsed.DueDate
		if due == nil {
			due = cloneTime(m.Preset.Shared.Due)
		}
		destination := parsed.Destination
		if !parsed.DestinationSet {
			destination = m.Preset.defaultDestination(parentID)
		}
		if coordinator.DueDateIsToday(due) {
			destination = app.CaptureDestinationToday
		}
		destination = grammar.ComposerDeferredDestination(parsed, destination)
		req := app.CaptureRequest{
			SourceLine:   idx + 1,
			ProjectID:    target.ID,
			Title:        parsed.Title,
			Priority:     priority,
			Tags:         mergeComposerTags(m.Preset.Shared.Tags, parsed.Tags),
			Context:      parsed.Context,
			Due:          due,
			DeferSet:     parsed.DeferSet,
			Defer:        parsed.DeferUntil,
			ParentID:     parentID,
			ChildIDs:     parsed.ChildIDs(),
			Destination:  destination,
			Prepend:      true,
			Pinned:       parsed.Pinned,
			Review:       parsed.Review,
			Attachments:  parsed.Attachments,
			Comments:     parsed.Comments,
			Dependencies: parsed.DependencyLocators(),
		}
		if parentID != nil {
			if err := validateComposerParent(target, *parentID); err != nil {
				m.Errors = append(m.Errors, grammar.LineError{Line: idx + 1, Message: err.Error()})
				continue
			}
		}
		reqs = append(reqs, req)
	}

	if len(reqs) == 0 && len(m.Errors) == 0 {
		m.Errors = []grammar.LineError{{Line: 1, Message: "no tasks to create"}}
	}
	if len(m.Errors) > 0 {
		return nil, false
	}
	return reqs, true
}
