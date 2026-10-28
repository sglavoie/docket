package taskform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/external"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/status"
)

func TestCaptureInspectionPreservesBatchDraftAndCaret(t *testing.T) {
	parent := "parent"
	project := &models.Project{ID: "web", Slug: "web", Name: "Web", Tasks: []*models.Task{
		{ID: "root", ShortID: 1, Title: "Fix login", WorkflowState: models.StateBacklog},
		{ID: "child", ShortID: 2, ParentID: &parent, Title: "Fix login", WorkflowState: models.StateDoing,
			ResumeNote: "Start with the redirect", Description: strings.Repeat("Details with Unicode café. ", 80)},
		{ID: parent, ShortID: 7, Title: "Release"},
	}}
	m := NewTaskComposerModel(TaskComposerPreset{Project: project,
		Shared: TaskComposerSharedAttributes{ParentID: &parent}, InitialText: "Fix login\nAnother task"}, nil)
	m.Input.MoveToBegin()
	m.Input.SetCursorColumn(3)
	value, line, col := m.Input.Value(), m.Input.Line(), m.Input.Column()
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if m.matchingTaskPreview == nil {
		t.Fatal("matching task did not open")
	}
	if !strings.Contains(m.matchingTaskText, "web#2") || !strings.Contains(m.matchingTaskText, "Start with the redirect") {
		t.Fatalf("inspected wrong task: %s", m.matchingTaskText)
	}
	for _, width := range []int{60, 100} {
		m.SetSize(width, 18)
		if text := ansi.Strip(m.View().Content); !strings.Contains(text, "Matching task") {
			t.Fatalf("missing inspection: %s", text)
		}
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.matchingTaskIndex != 0 {
		t.Fatal("left wrapped at first match")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if !strings.Contains(m.matchingTaskText, "web#1") || !strings.Contains(ansi.Strip(m.View().Content), "Match 2 of 2") {
		t.Fatalf("did not browse to root: %s", m.View().Content)
	}
	if m.matchingTaskPreview.YOffset() != 0 {
		t.Fatal("new match did not start at top")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.matchingTaskIndex != 1 {
		t.Fatal("right wrapped at last match")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if !strings.Contains(m.matchingTaskText, "web#2") {
		t.Fatal("did not return to sibling")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.SubmissionInFlight {
		t.Fatal("inspection submitted the draft")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.matchingTaskPreview != nil || m.Input.Value() != value || m.Input.Line() != line || m.Input.Column() != col {
		t.Fatal("return lost draft or caret")
	}
	m.Input.SetValue("No match")
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if m.matchingTaskPreview != nil {
		t.Fatal("opened stale match")
	}
}

func TestHostCapturePreviewIncludesDeferralPriorityAndDuplicate(t *testing.T) {
	home := &models.Project{ID: "home", Name: "Home", Slug: "home"}
	web := &models.Project{ID: "web", Name: "Web", Slug: "web", Tasks: []*models.Task{
		{ID: "existing", ShortID: 42, Title: "Fix login", WorkflowState: models.StateWaiting},
	}}
	preset := TaskComposerPreset{Project: home, AllProjects: []*models.Project{home, web}, AllowProjectSwitch: true}
	text, errs := CapturePreviewText(preset, nil, "Fix login %web !1 @@2030-01-02")
	if len(errs) != 0 {
		t.Fatalf("preview errors: %+v", errs)
	}
	for _, want := range []string{"Project: Web", "Defer until:", "Jan 2 2030", "Priority: High", "Matching title: web#42 · Waiting"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	text, errs = CapturePreviewText(preset, nil, "Fix login %web !1\nOther !2")
	if len(errs) != 0 || !strings.Contains(text, "Priority: Mixed") || !strings.Contains(text, "Line 1: Matching title:") {
		t.Fatalf("batch preview: %s, %+v", text, errs)
	}
	text, errs = CapturePreviewText(preset, nil, "Fix login %missing !1")
	if len(errs) == 0 || text != "" {
		t.Fatalf("invalid preview: %s, %+v", text, errs)
	}
}

func TestCaptureInspectionUsesExistingTaskScheduleAndQualifiedPlan(t *testing.T) {
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, time.Local)
	deferred := now.Add(2 * time.Hour)
	checkIn := time.Date(2030, 1, 2, 0, 0, 0, 0, time.Local)
	task := &models.Task{ID: "shared-id", ShortID: 2, Title: "Fix login", WorkflowState: models.StateBacklog,
		DeferredUntil: &deferred, NextCheckIn: &checkIn, CheckInRepeatDays: 7}
	project := &models.Project{ID: "web", Slug: "web", Tasks: []*models.Task{task}}
	runtime := newTestConfig()
	m := NewTaskComposerModel(TaskComposerPreset{Project: project, InitialText: "Fix login $today"}, runtime)
	locator := config.ProjectTaskLocator{ProjectID: "web", TaskID: task.ID}
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: "other", TaskID: task.ID}})
	text := m.matchingTaskDocument(task, project, now)
	if strings.Contains(text, "Plan:") {
		t.Fatalf("used draft or other project's plan: %s", text)
	}
	for _, want := range []string{"Status: Backlog", "Defer until: 14:00", "Check-in: Wed Jan 2", "Repeat: every 7d"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %s", want, text)
		}
	}
	runtime.Planning().SetNext([]config.ProjectTaskLocator{locator})
	if text = m.matchingTaskDocument(task, project, now); !strings.Contains(text, "Plan: Next") {
		t.Fatalf("missing Next membership: %s", text)
	}
	runtime.Planning().SetNext(nil)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{locator})
	task.CheckInRepeatDays = 0
	text = m.matchingTaskDocument(task, project, deferred)
	if !strings.Contains(text, "Plan: Today") || !strings.Contains(text, "Repeat: off") || strings.Contains(text, "Defer until:") {
		t.Fatalf("wrong current schedule: %s", text)
	}
	task.NextCheckIn = nil
	text = m.matchingTaskDocument(task, project, deferred)
	if strings.Contains(text, "Check-in:") || strings.Contains(text, "Repeat:") {
		t.Fatalf("invented check-in: %s", text)
	}
}

func TestCaptureInspectionSnapshotsMatchesAndReopensFresh(t *testing.T) {
	task := &models.Task{ID: "one", ShortID: 1, Title: "Fix login", Description: "Original description", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "web", Slug: "web", Tasks: []*models.Task{task}}
	m := NewTaskComposerModel(TaskComposerPreset{Project: project, InitialText: "Fix login"}, nil)
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	task.Description = "Changed description"
	m.SetSize(60, 20)
	if !strings.Contains(m.matchingTaskText, "Original description") {
		t.Fatal("inspection lost its snapshot")
	}
	guidance, _ := m.ContextualGuidance()
	for _, hint := range guidance.Hints {
		if strings.Contains(hint.Key, "←/→") {
			t.Fatal("single match advertises browsing")
		}
	}
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if !strings.Contains(m.matchingTaskText, "Changed description") {
		t.Fatal("reopening did not refresh")
	}
}

func TestCaptureInspectionIncludesCommentsAttachmentsAndCopiesSelectedReference(t *testing.T) {
	project := &models.Project{ID: "web", Slug: "web", Tasks: []*models.Task{
		{ID: "one", ShortID: 1, Title: "Fix login", WorkflowState: models.StateBacklog,
			Comments:    []*models.Comment{{Text: "Already reproduced with café credentials", CreatedAt: time.Date(2030, 1, 2, 12, 30, 0, 0, time.Local)}, nil},
			Attachments: []models.Attachment{{Filename: "trace.txt"}, {Filename: "https://example.com/issue", IsURL: true}, {OriginalPath: "/tmp/login.go", IsCodeRef: true, LineNumber: 42}}},
		{ID: "two", ShortID: 2, Title: "Fix login", WorkflowState: models.StateWaiting},
	}}
	m := NewTaskComposerModel(TaskComposerPreset{Project: project, InitialText: "Fix login"}, nil)
	m.Input.SetCursorColumn(3)
	m.Update(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	for _, want := range []string{"Attachments: 3", "trace.txt", "https://example.com/issue", "login.go:42", "Comment 2030-01-02 12:30:", "Already reproduced with café credentials"} {
		if !strings.Contains(m.matchingTaskText, want) {
			t.Fatalf("missing %q: %s", want, m.matchingTaskText)
		}
	}
	originalCopy := external.CopyText
	t.Cleanup(func() { external.CopyText = originalCopy })
	var copied string
	external.CopyText = func(value string) error { copied = value; return nil }
	m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	// A command copies the reference frozen at the keypress, even after browsing.
	m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if cmd == nil {
		t.Fatal("missing clipboard command")
	}
	if _, ok := cmd().(status.InfoStatusMsg); !ok || copied != "web#2" {
		t.Fatalf("wrong copy: %q", copied)
	}
	external.CopyText = func(string) error { return errors.New("clipboard unavailable") }
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'y'})
	if _, ok := cmd().(status.ErrorMsg); !ok {
		t.Fatal("clipboard failure not reported")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.Input.Value() != "Fix login" || m.Input.Column() != 3 || m.SubmissionInFlight {
		t.Fatal("inspection copy changed draft")
	}
}

// The host preview is what is read before a copied file leaves its folder, so
// it names every attachment, a file by the path its bytes come from, and
// refuses a missing file or a folder by name instead of leaving it to Enter.
func TestHostCapturePreviewNamesAttachmentsAndRefusesBadFiles(t *testing.T) {
	home := &models.Project{ID: "home", Name: "Home", Slug: "home"}
	preset := TaskComposerPreset{Project: home, AllProjects: []*models.Project{home}, AllowProjectSwitch: true}
	dir := t.TempDir()
	file := filepath.Join(dir, "report.pdf")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	text, errs := CapturePreviewText(preset, nil, "Read &"+file+" &message://%3Cabc@example.com%3E")
	if len(errs) != 0 {
		t.Fatalf("preview errors: %+v", errs)
	}
	if !strings.Contains(text, "Attachments: "+file+" · message://%3Cabc@example.com%3E") {
		t.Fatalf("preview = %q, want an attachments line naming the file path and the link", text)
	}

	missing := filepath.Join(dir, "absent.pdf")
	for _, tc := range []struct{ value, want string }{
		{missing, "file not found: " + missing},
		{dir, "not a file: " + dir},
	} {
		text, errs := CapturePreviewText(preset, nil, "Read &"+tc.value)
		if text != "" || len(errs) != 1 || !strings.Contains(errs[0].Message, tc.want) {
			t.Errorf("preview of %s = %q, %+v; want a refusal naming %q", tc.value, text, errs, tc.want)
		}
	}
}
