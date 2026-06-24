package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestComputeGlobalAlertLinesAdvertisesGlobalShortcut verifies the aggregate
// triage line still points to the global inbox processor via the ctrl+i hint
// so power users can act on the count without guessing the keybinding.
func TestComputeGlobalAlertLinesAdvertisesGlobalShortcut(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	project := testutil.NewTestProject("Triage Demo")
	inbox := testutil.NewTestTask("triage me", models.TaskTypeTask)
	inbox.Section = models.SectionInbox
	project.Tasks = []*models.Task{inbox}
	cfg.Projects = []*models.Project{project}

	m := &DashboardModel{config: cfg, projects: cfg.Projects}
	lines := m.computeGlobalAlertLines(nil)
	if len(lines) == 0 {
		t.Fatal("expected at least one alert line")
	}
	first := ansi.Strip(lines[0])
	if !strings.Contains(first, "1 to triage") {
		t.Fatalf("expected first alert line to surface triage count, got %q", first)
	}
	if !strings.Contains(first, "ctrl+i") {
		t.Fatalf("expected first alert line to point at global inbox shortcut, got %q", first)
	}
}

// TestComputeGlobalAlertLinesAdvertisesPerProjectTriage verifies the selected-
// project triage CTA is folded into the global triage line (session 9 alert
// hierarchy) so users see the O shortcut without a competing second row.
func TestComputeGlobalAlertLinesAdvertisesPerProjectTriage(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	project := testutil.NewTestProject("Alpha")
	for i := 0; i < 3; i++ {
		task := testutil.NewTestTask("triage me", models.TaskTypeTask)
		task.Section = models.SectionInbox
		project.Tasks = append(project.Tasks, task)
	}
	cfg.Projects = []*models.Project{project}

	m := &DashboardModel{config: cfg, projects: cfg.Projects}
	lines := m.computeGlobalAlertLines(project)
	if len(lines) == 0 {
		t.Fatalf("expected at least one alert line, got %v", lines)
	}

	first := ansi.Strip(lines[0])
	if !strings.Contains(first, "3 to triage") {
		t.Fatalf("expected global triage count in first line, got %q", first)
	}
	if !strings.Contains(first, "O for Alpha") {
		t.Fatalf("expected folded selected-project hint in first line, got %q", first)
	}
	if !strings.Contains(first, "(3)") {
		t.Fatalf("first line should include selected-project inbox count, got %q", first)
	}
}

// TestComputeGlobalAlertLinesSkipsPerProjectTriageWhenEmpty verifies the
// folded selected-project hint is omitted when the selected project has no
// inbox backlog, so the triage row doesn't carry a stale call to action.
func TestComputeGlobalAlertLinesSkipsPerProjectTriageWhenEmpty(t *testing.T) {
	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	project := testutil.NewTestProject("Clean")
	cfg.Projects = []*models.Project{project}

	m := &DashboardModel{config: cfg, projects: cfg.Projects}
	lines := m.computeGlobalAlertLines(project)
	for _, line := range lines {
		plain := ansi.Strip(line)
		if strings.Contains(plain, "O for ") {
			t.Fatalf("did not expect selected-project hint when inbox empty, got %q", plain)
		}
	}
}

// TestComputeGlobalAlertLinesOrdersTriageBeforeOverdue verifies the global
// triage line still appears before overdue / due-today / blocked lines so the
// "what should I do next?" mental model lands the user in triage first.
func TestComputeGlobalAlertLinesOrdersTriageBeforeOverdue(t *testing.T) {
	icons.SetUseEmoji(false)
	t.Cleanup(func() { icons.SetUseEmoji(false) })

	cfg := testutil.NewTestConfig()
	cfg.DataDir = t.TempDir()
	project := testutil.NewTestProject("Mixed")
	inbox := testutil.NewTestTask("triage me", models.TaskTypeTask)
	inbox.Section = models.SectionInbox

	overdue := testutil.NewTestTask("overdue thing", models.TaskTypeTask)
	overdue.Section = models.SectionNext
	overdueDue := time.Now().Add(-48 * time.Hour)
	overdue.DueDate = &overdueDue

	project.Tasks = []*models.Task{inbox, overdue}
	cfg.Projects = []*models.Project{project}

	m := &DashboardModel{config: cfg, projects: cfg.Projects}
	lines := m.computeGlobalAlertLines(nil)

	triageIdx, overdueIdx := -1, -1
	for i, line := range lines {
		plain := ansi.Strip(line)
		if triageIdx == -1 && strings.Contains(plain, "to triage") {
			triageIdx = i
		}
		if overdueIdx == -1 && strings.Contains(plain, "overdue") {
			overdueIdx = i
		}
	}
	if triageIdx == -1 {
		t.Fatalf("expected a triage line, got %v", lines)
	}
	if overdueIdx == -1 {
		t.Fatalf("expected an overdue line, got %v", lines)
	}
	if triageIdx >= overdueIdx {
		t.Fatalf("triage line (idx %d) should precede overdue line (idx %d): %v", triageIdx, overdueIdx, lines)
	}
}

// TestCloseInboxProcessorMessageOffersNextActions verifies the local close-
// message copy: when at least one task was triaged, the status bar must
// include the triaged count, the skipped count, and both next-action hints
// (;w Today and ;q Backlog). Pipeline Hub establishes this vocabulary.
func TestCloseInboxProcessorMessageOffersNextActions(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewInboxProcessor)
	app.views.Register(ViewInboxProcessor, &InboxProcessorModel{})

	msg := CloseInboxProcessorMsg{ProcessedCount: 3, SkippedCount: 1}
	handled, cmd := app.handleInboxMessages(msg)
	if !handled || cmd == nil {
		t.Fatal("expected handled close msg with a status-clear cmd")
	}
	got := app.statusMessage
	if !strings.Contains(got, "3 triaged") {
		t.Errorf("expected triaged count in status, got %q", got)
	}
	if !strings.Contains(got, "1 skipped") {
		t.Errorf("expected skipped count in status, got %q", got)
	}
	if !strings.Contains(got, ";w Today") {
		t.Errorf("expected ;w Today next-action in status, got %q", got)
	}
	if !strings.Contains(got, ";q Backlog") {
		t.Errorf("expected ;q Backlog next-action in status, got %q", got)
	}
}

// TestCloseInboxProcessorSkippedOnlyOffersBacklog verifies the skipped-only
// close path mentions ;q Backlog so the user has a way to revisit the
// deferred items.
func TestCloseInboxProcessorSkippedOnlyOffersBacklog(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewInboxProcessor)
	app.views.Register(ViewInboxProcessor, &InboxProcessorModel{})

	msg := CloseInboxProcessorMsg{ProcessedCount: 0, SkippedCount: 2}
	handled, cmd := app.handleInboxMessages(msg)
	if !handled || cmd == nil {
		t.Fatal("expected handled close msg with a status-clear cmd")
	}
	got := app.statusMessage
	if !strings.Contains(got, "2 skipped") {
		t.Errorf("expected skipped count in status, got %q", got)
	}
	if !strings.Contains(got, ";q Backlog") {
		t.Errorf("expected ;q Backlog next-action in status, got %q", got)
	}
}

// TestCloseGlobalInboxProcessorMessageOffersNextActions verifies the global
// close-message copy mirrors the local processor: triaged + skipped counts and
// both ;w Today and ;q Backlog hints.
func TestCloseGlobalInboxProcessorMessageOffersNextActions(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewGlobalInboxProcessor)

	msg := CloseGlobalInboxProcessorMsg{ProcessedCount: 5, SkippedCount: 2}
	handled, cmd := app.handleInboxMessages(msg)
	if !handled || cmd == nil {
		t.Fatal("expected handled close msg with a status-clear cmd")
	}
	got := app.statusMessage
	if !strings.Contains(got, "5 triaged") {
		t.Errorf("expected triaged count in status, got %q", got)
	}
	if !strings.Contains(got, "2 skipped") {
		t.Errorf("expected skipped count in status, got %q", got)
	}
	if !strings.Contains(got, ";w Today") {
		t.Errorf("expected ;w Today next-action in status, got %q", got)
	}
	if !strings.Contains(got, ";q Backlog") {
		t.Errorf("expected ;q Backlog next-action in status, got %q", got)
	}
}

// TestCloseGlobalInboxProcessorSkippedOnlyOffersBacklog verifies the skipped-
// only global close path also surfaces ;q Backlog.
func TestCloseGlobalInboxProcessorSkippedOnlyOffersBacklog(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewGlobalInboxProcessor)

	msg := CloseGlobalInboxProcessorMsg{ProcessedCount: 0, SkippedCount: 3}
	handled, cmd := app.handleInboxMessages(msg)
	if !handled || cmd == nil {
		t.Fatal("expected handled close msg with a status-clear cmd")
	}
	got := app.statusMessage
	if !strings.Contains(got, "3 skipped") {
		t.Errorf("expected skipped count in status, got %q", got)
	}
	if !strings.Contains(got, ";q Backlog") {
		t.Errorf("expected ;q Backlog next-action in status, got %q", got)
	}
}
