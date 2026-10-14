package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/notes"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

type staticCompletionActivityReader struct {
	activity app.CompletionActivity
	err      error
	calls    int
}

func (r *staticCompletionActivityReader) CompletionActivity() (app.CompletionActivity, error) {
	r.calls++
	return r.activity, r.err
}

// TestFullscreenWorkspacePulseTitleRendersZeroAndNonzeroStates holds the pulse
// to the zero test: a component with nothing to count is omitted rather than
// reported, and a pulse with nothing at all to say leaves no bare `workspace:`
// prefix behind. An empty want spells that absence.
func TestFullscreenWorkspacePulseTitleRendersZeroAndNonzeroStates(t *testing.T) {
	tests := []struct {
		name  string
		pulse pulse.WorkspacePulse
		want  string
	}{
		{name: "zero", pulse: pulse.WorkspacePulse{}},
		{name: "partial", pulse: pulse.WorkspacePulse{Inbox: 3, Overdue: 1}, want: "workspace: inbox 3 · overdue 1"},
		{name: "nonzero", pulse: pulse.WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}, want: "workspace: inbox 3 · today 2 · overdue 1 · done 4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := ansi.Strip(chrome.RenderFullscreenTitleWithPulse("Notes", "All Notes", tt.pulse, 100))
			if tt.want == "" {
				if strings.Contains(rendered, "workspace") {
					t.Fatalf("a pulse with nothing to count still named itself:\n%s", rendered)
				}
				return
			}
			if !strings.Contains(rendered, tt.want) {
				t.Fatalf("title missing pulse %q:\n%s", tt.want, rendered)
			}
			if strings.Count(rendered, "workspace:") != 1 {
				t.Fatalf("workspace pulse count = %d, want one:\n%s", strings.Count(rendered, "workspace:"), rendered)
			}
		})
	}
}

func TestTodayPaceReadoutIsTodayOnlyAndRequiresHistory(t *testing.T) {
	project := scopeTestProject("alpha", "Alpha", "launch")
	outlinerView := scopeTestOutliner(project)
	outlinerView.SetWorkspacePulse(pulse.WorkspacePulse{Inbox: 3, Today: 9, Overdue: 1})
	outlinerView.SetCompletionActivity(app.CompletionActivity{
		DailyCompletions: map[time.Time]int{time.Date(2026, time.August, 19, 0, 0, 0, 0, time.UTC): 1},
	})

	for _, mode := range chrome.OutlinerModeCatalog() {
		outlinerView.PrimaryMode = mode.ID
		plain := ansi.Strip(outlinerView.BuildOutlinerTitleLine(160))
		if mode.ID == chrome.ModeToday {
			if !strings.Contains(plain, "9 open · median 0 done/day (30d)") {
				t.Fatalf("Today title missing pace readout: %q", plain)
			}
			if !strings.Contains(plain, "workspace:") {
				t.Fatalf("Today pace replaced the separate pulse region: %q", plain)
			}
			continue
		}
		if strings.Contains(plain, "median") || strings.Contains(plain, "(30d)") {
			t.Fatalf("%s title rendered Today-only pace: %q", mode.Label, plain)
		}
	}

	outlinerView.PrimaryMode = chrome.ModeToday
	outlinerView.SetCompletionActivity(app.CompletionActivity{})
	plain := ansi.Strip(outlinerView.BuildOutlinerTitleLine(160))
	if strings.Contains(plain, "median") || strings.Contains(plain, "(30d)") {
		t.Fatalf("history-free workspace rendered a zero pace: %q", plain)
	}
}

func TestAppViewUsesCachedCompletionActivityWithoutRenderTimeRead(t *testing.T) {
	model := typedTestApp(t)
	reader := &staticCompletionActivityReader{activity: app.CompletionActivity{
		DailyCompletions: map[time.Time]int{time.Date(2026, time.August, 19, 0, 0, 0, 0, time.UTC): 1},
	}}
	model.features.Activity = coordinator.NewActivityCoordinator(reader)
	model.crossOutliner.PrimaryMode = chrome.ModeToday
	model.refreshWorkspacePulseAt(time.Now())
	model.crossOutliner.SetCompletionActivity(app.CompletionActivity{})

	_ = model.View()

	if reader.calls != 1 {
		t.Fatalf("completion activity queries = %d, want one cached refresh and no render-time read", reader.calls)
	}
	if !model.crossOutliner.CompletionActivity.HasHistory() {
		t.Fatal("active Outliner did not receive refreshed completion activity")
	}
}

func TestPresentationChromeEligibleFamiliesRenderWorkspacePulse(t *testing.T) {
	task, err := models.NewGenericTask("task-1", 1, "Scoped task")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	project := &models.Project{ID: "project-1", Name: "Project One", Slug: "project-one"}

	tests := []struct {
		name string
		view func(pulse.WorkspacePulse) string
	}{
		{name: "project creator", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := projecthub.NewProjectCreatorModel(nil, nil, nil)
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "project editor", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := projecthub.NewProjectEditorModel(project, nil, nil, nil, false)
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "notes", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := &notes.NotesViewModel{Scope: route.AllNotesScope()}
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "task form", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := taskform.NewTaskFormModelCreate(project, nil, nil)
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "task tool", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := taskdetail.NewCommentsPanel(taskdetail.NewTaskCommentOwner(task, project, coordinator.SubviewCoordinator{}), "")
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "text editor", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := chrome.NewTextEditorModel("description", "", "body")
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "help", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := help.NewHelpViewModel(nil, "Outliner")
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "project hub", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := projecthub.NewProjectHubModel(config.EmptyRuntimeState(""))
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
	}

	// The expectations are spelled out rather than read back from the formatter
	// internal/tui/pulse owns, which is what this file already does for the same
	// pulses above and below. The formatter's own component order, zero test, and
	// ladder are asserted inside that package; what this test adds is that every
	// eligible family paints the full form exactly once — and paints nothing at
	// all where the workspace has nothing to report.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, pulse := range []struct {
				value pulse.WorkspacePulse
				want  string
			}{
				{value: pulse.WorkspacePulse{}},
				{value: pulse.WorkspacePulse{Inbox: 3, Overdue: 1}, want: "workspace: inbox 3 · overdue 1"},
				{value: pulse.WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}, want: "workspace: inbox 3 · today 2 · overdue 1 · done 4"},
			} {
				plain := ansi.Strip(tt.view(pulse.value))
				want := pulse.want
				if want == "" {
					if strings.Contains(plain, "workspace") {
						t.Fatalf("a pulse with nothing to count still named itself:\n%s", plain)
					}
					continue
				}
				if !strings.Contains(plain, want) {
					t.Fatalf("view missing workspace pulse %q:\n%s", want, plain)
				}
				if strings.Count(plain, "workspace:") != 1 {
					t.Fatalf("workspace pulse count = %d, want one:\n%s", strings.Count(plain, "workspace:"), plain)
				}
			}
		})
	}
}

// TestPresentationChromeHostedPanelLeavesThePulseToItsHost is the other side of
// that contract. The compact composer is a bounded panel with no title row, and
// the presentation it is drawn over keeps painting its own, so the composition
// still carries the pulse exactly once. A panel that painted one too would
// double it; one that replaced its host would lose it.
func TestPresentationChromeHostedPanelLeavesThePulseToItsHost(t *testing.T) {
	model := typedTestApp(t)
	model.handleWindowSize(tea.WindowSizeMsg{Width: 100, Height: 30})

	handled, _ := model.handleViewMessages(model.captureComposerMessage(model.currentProject, model.taskWorkspaceProjects()))
	if !handled || model.currentView() != presentation.ViewTaskComposer {
		t.Fatalf("composer did not open: handled=%v view=%v", handled, model.currentView())
	}
	// The count arrives after the panel opened, and only the shell holds it: the
	// host is not the active view, so it shows this pulse only if View hands it
	// one.
	model.workspacePulse = pulse.WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}
	model.syncHostedPanelHost()

	plain := ansi.Strip(model.View().Content)
	if want := "workspace: inbox 3 · today 2 · overdue 1 · done 4"; !strings.Contains(plain, want) {
		t.Fatalf("hosted composition missing workspace pulse %q:\n%s", want, plain)
	}
	if got := strings.Count(plain, "workspace:"); got != 1 {
		t.Fatalf("workspace pulse count = %d, want one:\n%s", got, plain)
	}
}

func TestPresentationChromeTransientPopupPreservesHostPulse(t *testing.T) {
	const width, height = 100, 8
	hostTitle := chrome.RenderFullscreenTitleWithPulse("Notes", "All Notes", pulse.WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}, width)
	host := hostTitle + strings.Repeat("\n", height-1)
	tallPopup := strings.Repeat("transient action\n", height*2) + "last action"
	rendered := ansi.Strip(overlayPopupBelowPersistentTitle(host, tallPopup, width, height))
	if !strings.Contains(rendered, "workspace: inbox 3 · today 2 · overdue 1 · done 4") {
		t.Fatalf("transient popup obscured the host pulse:\n%s", rendered)
	}
	if strings.Count(rendered, "workspace:") != 1 {
		t.Fatalf("transient popup duplicated the host pulse:\n%s", rendered)
	}
}
