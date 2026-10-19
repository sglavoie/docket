package taskform

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
)

func recoveryComposerFactory(t *testing.T) func() *TaskComposerModel {
	t.Helper()
	domains := config.NewDomains()
	domains.Catalog.WorkspaceID = uuid.NewString()
	runtime := config.NewRuntimeState(t.TempDir(), domains)
	project := &models.Project{ID: "project", Name: "Project", Slug: "project"}
	return func() *TaskComposerModel {
		m := NewTaskComposerModel(TaskComposerPreset{Project: project, AllProjects: []*models.Project{project}}, runtime)
		m.EnableDraftRecovery()
		if m.recovery == nil {
			t.Fatalf("no recovery store: %+v", m.Errors)
		}
		return m
	}
}

func TestCaptureCloseRestoresVerbatimDraftAndCaret(t *testing.T) {
	fresh := recoveryComposerFactory(t)
	m := fresh()
	text := "First task\nFix café link @not-a-date"
	m.Input.SetValue(text)
	m.Input.SetCursorColumn(4)
	line, column := m.Input.Line(), m.Input.Column()
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatalf("close failed: %+v", m.Errors)
	}
	if _, ok := cmd().(status.BackMsg); !ok || m.ConfirmDiscard {
		t.Fatal("close asked to discard")
	}
	restored := fresh()
	if restored.Input.Value() != text || restored.Input.Line() != line || restored.Input.Column() != column {
		t.Fatalf("draft/caret changed: %q %d/%d", restored.Input.Value(), restored.Input.Line(), restored.Input.Column())
	}
	restored.Update(syntheticCtrlKey('x'))
	if !restored.ConfirmDiscard {
		t.Fatal("discard did not ask")
	}
	restored.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if fresh().Input.Value() != text {
		t.Fatal("canceled discard lost draft")
	}
	restored.Update(syntheticCtrlKey('x'))
	_, cmd = restored.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil || fresh().HasDraft() {
		t.Fatal("confirmed discard retained draft")
	}
}

func TestCaptureAutosaveRestoresOpenDraftAfterInterruptedSession(t *testing.T) {
	fresh := recoveryComposerFactory(t)
	m := fresh()
	_, scheduled := m.Update(tea.PasteMsg{Content: "Draft left open @tomorrow"})
	if scheduled == nil || m.recoveryAutosaveSequence == 0 {
		t.Fatal("editing did not schedule a recovery copy")
	}
	stale := composerAutosaveMsg{owner: m, sequence: m.recoveryAutosaveSequence}
	m.Update(tea.KeyPressMsg{Code: '!', Text: "!"})
	if m.recoveryAutosaveSequence == stale.sequence {
		t.Fatal("later typing did not supersede the old autosave")
	}
	m.Update(stale)
	if fresh().HasDraft() {
		t.Fatal("a stale timer saved an unfinished version")
	}
	m.Update(composerAutosaveMsg{owner: m, sequence: m.recoveryAutosaveSequence})
	restored := fresh()
	if restored.Input.Value() != m.Input.Value() || restored.Input.Column() != m.Input.Column() {
		t.Fatalf("interrupted draft/caret = %q/%d, want %q/%d", restored.Input.Value(), restored.Input.Column(), m.Input.Value(), m.Input.Column())
	}
}

func TestExpandedCaptureAutosaveRestoresUnsavedFieldEdit(t *testing.T) {
	fresh := recoveryComposerFactory(t)
	m := fresh()
	m.ExpandRequest(app.CaptureRequest{ProjectID: "project", Title: "Before", Destination: app.CaptureDestinationInbox})
	_, scheduled := m.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	if scheduled == nil || m.recoveryAutosaveSequence == 0 {
		t.Fatal("expanded edit did not schedule recovery")
	}
	m.Update(composerAutosaveMsg{owner: m, sequence: m.recoveryAutosaveSequence})
	restored := fresh()
	if restored.Mode != ComposerExpanded || restored.Expanded.titleInput.Value() != m.Expanded.titleInput.Value() {
		t.Fatalf("expanded edit was lost: %q", restored.Expanded.titleInput.Value())
	}
}

func TestExpandedCaptureRecoveryKeepsRawFieldsAndChildren(t *testing.T) {
	fresh := recoveryComposerFactory(t)
	m := fresh()
	m.ExpandRequest(app.CaptureRequest{ProjectID: "project", Title: "Keep it", Destination: app.CaptureDestinationToday, Description: "Unfinished prose", Tags: []string{"review"}, Attachments: []string{"https://example.test/"}, Comments: []string{"Ask Maya"}})
	m.Expanded.dueTextInput.SetValue("not a date yet")
	m.Expanded.contextInput.SetValue("unfinished context ")
	m.Expanded.draft.subtasks = []formSubtaskDraft{{title: "Child draft"}}
	m.Expanded.draft.checklist = []models.ChecklistItem{{Text: "Read notes", Done: true}}
	m.Expanded.activeTab, m.Expanded.focusIndex = formTabSchedule, 0
	before := m.recoverySnapshot(false)
	if m.closeKeepingDraft() == nil {
		t.Fatal(m.Expanded.err)
	}
	after := fresh().recoverySnapshot(false)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("expanded recovery changed fields:\n%+v\n%+v", before.Expanded, after.Expanded)
	}
}

func TestCaptureRecoveryRefusesConcurrentReplacementAndCorruptData(t *testing.T) {
	fresh := recoveryComposerFactory(t)
	first, second := fresh(), fresh()
	first.Input.SetValue("First terminal")
	second.Input.SetValue("Second terminal")
	if first.closeKeepingDraft() == nil {
		t.Fatal(first.Errors)
	}
	if second.closeKeepingDraft() != nil || second.Input.Value() != "Second terminal" {
		t.Fatal("concurrent draft was overwritten")
	}
	if fresh().Input.Value() != "First terminal" {
		t.Fatal("first draft lost")
	}
	if err := first.recovery.Replace([]byte("broken json")); err != nil {
		t.Fatal(err)
	}
	corrupt := fresh()
	corrupt.Input.SetValue("Keep new text too")
	if corrupt.closeKeepingDraft() != nil {
		t.Fatal("corrupt original was overwritten")
	}
	data, err := corrupt.recovery.Read()
	if err != nil || string(data) != "broken json" {
		t.Fatalf("corrupt original lost: %s %v", data, err)
	}
	if corrupt.discardRecovery() == nil || fresh().HasDraft() {
		t.Fatal("explicit discard could not clear an unreadable draft")
	}
}

func TestCaptureRecoveryDistinguishesCommitFailureAndInterruptedSave(t *testing.T) {
	fresh := recoveryComposerFactory(t)
	m := fresh()
	m.Input.SetValue("Keep this task")
	calls := 0
	cmd := m.guardRecoverySubmit(func() tea.Msg { calls++; return status.ErrorMsg{Err: errors.New("disk full")} })
	if cmd == nil {
		t.Fatal(m.Errors)
	}
	interrupted := fresh()
	if !interrupted.recoveryPending {
		t.Fatal("submission was not marked before dispatch")
	}
	interrupted.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	interrupted.Update(composerAutosaveMsg{owner: interrupted, sequence: interrupted.recoveryAutosaveSequence})
	if !fresh().recoveryPending {
		t.Fatal("autosave erased an interrupted submission marker")
	}
	if interrupted.guardRecoverySubmit(func() tea.Msg { calls++; return nil }) != nil || calls != 0 {
		t.Fatal("ambiguous submission could be repeated")
	}
	cmd()
	failed := fresh()
	if failed.recoveryPending || failed.Input.Value() != "Keep this task" {
		t.Fatal("failure lost recoverable draft")
	}
	cmd = failed.guardRecoverySubmit(func() tea.Msg { return coordinator.TaskCreatedMsg{} })
	if cmd == nil {
		t.Fatal(failed.Errors)
	}
	cmd()
	if fresh().HasDraft() {
		t.Fatal("committed draft can be resubmitted after reopening")
	}
}

func TestCaptureRecoveryKeepsUnknownFormatUntouched(t *testing.T) {
	fresh := recoveryComposerFactory(t)
	m := fresh()
	data, _ := json.Marshal(composerRecovery{Version: 99, ProjectID: "project", Text: "Future draft"})
	if err := m.recovery.Replace(data); err != nil {
		t.Fatal(err)
	}
	restored := fresh()
	restored.Input.SetValue("New draft")
	if restored.closeKeepingDraft() != nil || !strings.Contains(restored.Errors[0].Message, "unsupported") {
		t.Fatal("unknown format was accepted")
	}
}
