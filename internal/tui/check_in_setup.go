package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskform"
)

// This pass owns no navigation or persistent queue. The checkpoint restores
// presentation over the refreshed data; it never rolls back a saved schedule.
type checkInSetupPass struct {
	candidates                     []app.CheckInSetupCandidate
	index, saved, skipped, changed int
	checkpoint                     *outliner.OutlinerPresentationCheckpoint
	busy, exiting                  bool
	attempt                        uint64
	readError                      string
}

type checkInSetupSavedMsg struct {
	pass    *checkInSetupPass
	index   int
	attempt uint64
	value   operation.OperationCheckInValue
	result  app.TaskResult
	skipped bool
	err     error
}

func (m *AppModel) startCheckInSetup() tea.Cmd {
	if m.checkInSetup != nil || m.Overlays().Active() {
		m.feedback.err = fmt.Errorf("close the current editor before setting up check-ins")
		return nil
	}
	if m.domain.service == nil {
		m.feedback.err = fmt.Errorf("check-in setup service is unavailable")
		return nil
	}
	candidates, err := m.domain.service.CheckInSetupCandidates()
	if err != nil {
		m.feedback.err = fmt.Errorf("check-in setup could not cover the active workspace: %w", err)
		return nil
	}
	m.feedback.err = nil
	pass := &checkInSetupPass{candidates: candidates}
	if m.crossOutliner != nil {
		checkpoint := m.crossOutliner.CaptureOutlinerPresentationCheckpoint()
		pass.checkpoint = &checkpoint
	}
	m.retireInteraction()
	m.checkInSetup = pass
	return m.advanceCheckInSetup()
}

func (m *AppModel) advanceCheckInSetup() tea.Cmd {
	pass := m.checkInSetup
	for pass.index < len(pass.candidates) {
		candidate := pass.candidates[pass.index]
		fresh, eligible, err := m.domain.service.ResolveCheckInSetup(candidate.Locator)
		if err == nil && !eligible {
			pass.changed++
			pass.index++
			continue
		}
		pass.readError = ""
		if err == nil {
			candidate = fresh
		} else {
			pass.readError = fmt.Sprintf("Workspace read failed; coverage incomplete: %v", err)
		}
		subtitle := fmt.Sprintf("%s (%s) · #%d %s\n%d of %d · %d remaining after this\n%d saved · %d skipped · %d no longer eligible",
			candidate.ProjectName, candidate.ProjectSlug, candidate.ShortID, candidate.Title,
			pass.index+1, len(pass.candidates), len(pass.candidates)-pass.index-1,
			pass.saved, pass.skipped, pass.changed)
		editor := taskform.NewCheckInEditorModel(operation.OperationExecutionRequest{}, subtitle, models.CheckInSchedule{}, false)
		editor.ConfigureSetup()
		if pass.readError != "" {
			editor.Restore(operation.OperationCheckInValue{RepeatInput: "Off"}, pass.readError)
		}
		m.operations.CheckInEditor = editor
		return editor.InitCmd()
	}
	return m.finishCheckInSetup()
}

func (m *AppModel) handleCheckInSetupKey(msg tea.KeyPressMsg) tea.Cmd {
	pass := m.checkInSetup
	if pass.busy {
		if msg.String() == "esc" {
			pass.exiting = true
		}
		return nil
	}
	switch msg.String() {
	case "esc":
		return m.finishCheckInSetup()
	case "ctrl+n":
		pass.skipped++
		pass.index++
		return m.advanceCheckInSetup()
	}
	cmd, _, result := m.operations.CheckInEditor.HandleKey(msg)
	selected, ok := result.(operation.OperationValueSelectedMsg)
	if !ok {
		return cmd
	}
	value := selected.Value.(operation.OperationCheckInValue)
	if value.Clear {
		pass.skipped++
		pass.index++
		return m.advanceCheckInSetup()
	}
	pass.busy = true
	pass.attempt++
	attempt := pass.attempt
	index := pass.index
	locator := pass.candidates[index].Locator
	service := m.domain.service
	return func() tea.Msg {
		result, skipped, err := service.SaveCheckInSetup(locator, value.Schedule)
		return checkInSetupSavedMsg{pass: pass, index: index, attempt: attempt, value: value, result: result, skipped: skipped, err: err}
	}
}

func (m *AppModel) handleCheckInSetupSaved(msg checkInSetupSavedMsg) tea.Cmd {
	pass := m.checkInSetup
	// A duplicate or obsolete delivery can neither advance nor reopen a pass.
	if pass == nil || msg.pass != pass || msg.index != pass.index || msg.attempt != pass.attempt || !pass.busy {
		return nil
	}
	pass.busy = false
	if msg.err != nil {
		pass.readError = fmt.Sprintf("Save failed; task remains pending: %v", msg.err)
		m.operations.CheckInEditor.Restore(msg.value, pass.readError)
		if pass.exiting {
			return m.finishCheckInSetup()
		}
		return m.operations.CheckInEditor.InitCmd()
	}
	pass.readError = ""
	if msg.skipped {
		pass.changed++
	} else {
		pass.saved++
		if m.projectByID(msg.result.ProjectID) != nil && !m.reloadProjectByID(msg.result.ProjectID) {
			pass.readError = "Schedule saved, but the view could not reload its project."
		}
	}
	pass.index++
	if pass.exiting || pass.readError != "" {
		return m.finishCheckInSetup()
	}
	return m.advanceCheckInSetup()
}

func (m *AppModel) finishCheckInSetup() tea.Cmd {
	pass := m.checkInSetup
	m.operations.CheckInEditor = nil
	m.checkInSetup = nil
	if pass.checkpoint != nil && m.crossOutliner != nil {
		m.crossOutliner.RestoreOutlinerPresentationCheckpoint(*pass.checkpoint)
	}
	state := "finished"
	if pass.index < len(pass.candidates) {
		state = "stopped"
	}
	summary := fmt.Sprintf("Check-in setup %s: %d saved, %d skipped, %d no longer eligible, %d remaining (initial pass: %d)",
		state, pass.saved, pass.skipped, pass.changed, len(pass.candidates)-pass.index, len(pass.candidates))
	// The summary is ordinary shell feedback: it goes through the setters so it
	// takes its own sequence, severity, and reading time rather than inheriting
	// the previous message's, and its own history entry rather than rewriting
	// the one before it.
	if pass.readError != "" {
		m.setWarningStatus(summary + ". " + pass.readError)
	} else {
		m.setInfoStatus(summary)
	}
	return m.statusReadingTimeout()
}
