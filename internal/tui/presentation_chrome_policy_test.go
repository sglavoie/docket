package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/presentation"
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

func TestPresentationChromePolicyClassifiesEveryPresentation(t *testing.T) {
	transient := map[presentation.PresentationID]bool{
		presentation.ViewPriorityPicker: true,
		presentation.ViewParentSelector: true,
		presentation.ViewTagSelector:    true,
		presentation.ViewOutlineErrors:  true,
	}

	for _, presentation := range allPresentations() {
		policy, ok := chromePolicyForPresentation(presentation)
		if !ok {
			t.Fatalf("presentation %s has no chrome policy", presentation)
		}
		wantPulse := !transient[presentation]
		if got := policy.workspacePulseEligible(); got != wantPulse {
			t.Errorf("%s pulse eligibility = %t, want %t", presentation, got, wantPulse)
		}
		switch policy.guidance {
		case presentationGuidanceShell, presentationGuidanceSurface, presentationGuidanceOverlay:
		default:
			t.Errorf("%s has no shell, surface, or overlay guidance owner", presentation)
		}
		if transient[presentation] && policy.guidance != presentationGuidanceOverlay {
			t.Errorf("%s guidance owner = %v, want overlay", presentation, policy.guidance)
		}
	}

	if _, ok := chromePolicyForPresentation(presentation.ViewUnknown); ok {
		t.Fatal("ViewUnknown must not receive an implicit chrome policy")
	}
}

func TestPresentationGuidanceReachabilityNamesShellAndSurfaceOwners(t *testing.T) {
	wantShell := map[presentation.PresentationID]bool{
		presentation.ViewCrossProjectOutliner: true,
		presentation.ViewTaskComposer:         true,
	}
	for _, presentation := range allPresentations() {
		policy, ok := chromePolicyForPresentation(presentation)
		if !ok || policy.kind != presentationChromeFullscreen {
			continue
		}
		want := presentationGuidanceSurface
		if wantShell[presentation] {
			want = presentationGuidanceShell
		}
		if policy.guidance != want {
			t.Errorf("%s guidance owner = %v, want %v", presentation, policy.guidance, want)
		}
	}
}

func TestFullscreenWorkspacePulseTitleRendersZeroAndNonzeroStates(t *testing.T) {
	tests := []struct {
		name  string
		pulse WorkspacePulse
		want  string
	}{
		{name: "zero", pulse: WorkspacePulse{}, want: "workspace: today 0 · done 0"},
		{name: "nonzero", pulse: WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}, want: "workspace: inbox 3 · today 2 · overdue 1 · done 4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := ansi.Strip(renderFullscreenTitleWithPulse("Notes", "All Notes", tt.pulse, 100))
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
	outliner.workspacePulse = WorkspacePulse{Inbox: 3, Today: 9, Overdue: 1}
	outliner.setCompletionActivity(app.CompletionActivity{
		DailyCompletions: map[time.Time]int{time.Date(2026, time.August, 19, 0, 0, 0, 0, time.UTC): 1},
	})

	for _, mode := range OutlinerModeCatalog() {
		outliner.primaryMode = mode.ID
		plain := ansi.Strip(outliner.buildOutlinerTitleLine(160))
		if mode.ID == ModeToday {
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

	outliner.primaryMode = ModeToday
	outliner.setCompletionActivity(app.CompletionActivity{})
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
	model.features.Activity = ActivityCoordinator{reader: reader}
	model.crossOutliner.primaryMode = ModeToday
	model.refreshWorkspacePulseAt(time.Now())
	model.crossOutliner.setCompletionActivity(app.CompletionActivity{})

	_ = model.View()

	if reader.calls != 1 {
		t.Fatalf("completion activity queries = %d, want one cached refresh and no render-time read", reader.calls)
	}
	if !model.crossOutliner.completionActivity.HasHistory() {
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
		view func(WorkspacePulse) string
	}{
		{name: "project creator", view: func(pulse WorkspacePulse) string {
			m := NewProjectCreatorModel(nil, nil, nil)
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
		{name: "project editor", view: func(pulse WorkspacePulse) string {
			m := NewProjectEditorModel(project, nil, nil, nil, false)
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
		{name: "notes", view: func(pulse WorkspacePulse) string {
			m := &NotesViewModel{scope: AllNotesScope()}
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
		{name: "task composer", view: func(pulse WorkspacePulse) string {
			m := NewTaskComposerModel(TaskComposerPreset{Kind: ComposerProject, Project: project}, nil)
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
		{name: "task form", view: func(pulse WorkspacePulse) string {
			m := NewTaskFormModelCreate(project, nil, nil)
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
		{name: "task tool", view: func(pulse WorkspacePulse) string {
			m := NewTaskCommentsModel(task, project)
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
		{name: "text editor", view: func(pulse WorkspacePulse) string {
			m := NewTextEditorModel("description", "body")
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
		{name: "help", view: func(pulse WorkspacePulse) string {
			m := NewHelpViewModel(nil, "Outliner")
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
		{name: "project hub", view: func(pulse WorkspacePulse) string {
			m := NewProjectHubModel(config.EmptyRuntimeState(""))
			m.SetSize(140, 30)
			m.setWorkspacePulse(pulse)
			return m.View().Content
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, pulse := range []WorkspacePulse{{}, {Inbox: 3, Today: 2, Overdue: 1, Done: 4}} {
				plain := ansi.Strip(tt.view(pulse))
				want := ansi.Strip(formatWorkspacePulseComponents(pulse, workspacePulseFull))
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
	hostTitle := renderFullscreenTitleWithPulse("Notes", "All Notes", WorkspacePulse{Inbox: 3, Today: 2, Overdue: 1, Done: 4}, width)
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
