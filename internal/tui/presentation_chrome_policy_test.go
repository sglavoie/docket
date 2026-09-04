package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/help"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/route"
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

func TestFullscreenWorkspacePulseTitleRendersZeroAndNonzeroStates(t *testing.T) {
	tests := []struct {
		name  string
		pulse pulse.WorkspacePulse
		want  string
	}{
		{name: "zero", pulse: pulse.WorkspacePulse{}, want: "workspace: today 0 · done 0"},
		{name: "nonzero", pulse: pulse.WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}, want: "workspace: inbox 3 · today 2 · overdue 1 · done 4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := ansi.Strip(chrome.RenderFullscreenTitleWithPulse("Notes", "All Notes", tt.pulse, 100))
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
	outliner := scopeTestOutliner(project)
	outliner.workspacePulse = pulse.WorkspacePulse{Inbox: 3, Today: 9, Overdue: 1}
	outliner.SetCompletionActivity(app.CompletionActivity{
		DailyCompletions: map[time.Time]int{time.Date(2026, time.August, 19, 0, 0, 0, 0, time.UTC): 1},
	})

	for _, mode := range chrome.OutlinerModeCatalog() {
		outliner.primaryMode = mode.ID
		plain := ansi.Strip(outliner.buildOutlinerTitleLine(160))
		if mode.ID == chrome.ModeToday {
			if !strings.Contains(plain, "today 9 · median 0/day (30d)") {
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

	outliner.primaryMode = chrome.ModeToday
	outliner.SetCompletionActivity(app.CompletionActivity{})
	plain := ansi.Strip(outliner.buildOutlinerTitleLine(160))
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
	model.crossOutliner.primaryMode = chrome.ModeToday
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
			m := NewProjectCreatorModel(nil, nil, nil)
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "project editor", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := NewProjectEditorModel(project, nil, nil, nil, false)
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "notes", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := &NotesViewModel{scope: route.AllNotesScope()}
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "task composer", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "task form", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := NewTaskFormModelCreate(project, nil, nil)
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "task tool", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := NewTaskCommentsModel(task, project)
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
		{name: "text editor", view: func(workspacePulse pulse.WorkspacePulse) string {
			m := chrome.NewTextEditorModel("description", "body")
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
			m := NewProjectHubModel(config.EmptyRuntimeState(""))
			m.SetSize(140, 30)
			m.SetWorkspacePulse(workspacePulse)
			return m.View().Content
		}},
	}

	// The two expectations are spelled out rather than read back from the
	// formatter internal/tui/pulse owns, which is what this file already does
	// for the same two pulses above and below. The formatter's own component
	// order and ladder are asserted inside that package; what this test adds is
	// that every eligible family paints the full form exactly once.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, pulse := range []struct {
				value pulse.WorkspacePulse
				want  string
			}{
				{value: pulse.WorkspacePulse{}, want: "workspace: today 0 · done 0"},
				{value: pulse.WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}, want: "workspace: inbox 3 · today 2 · overdue 1 · done 4"},
			} {
				plain := ansi.Strip(tt.view(pulse.value))
				want := pulse.want
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
