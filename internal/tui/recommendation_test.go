package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
)

// Session 2 contract: the cockpit recommendation domain model must expose
// the explicit reason set the redesign promises. The engine (session 3) and
// renderer (session 4) consume these values verbatim; if any required reason
// disappears or its label drifts, downstream copy silently breaks.

func TestRecommendationReasonsCoverContract(t *testing.T) {
	want := map[RecommendationReason]string{
		ReasonPlannedToday:     "planned today",
		ReasonOverdue:          "overdue",
		ReasonDueToday:         "due today",
		ReasonHighPriority:     "high priority",
		ReasonInboxNeedsTriage: "inbox needs triage",
		ReasonBlocked:          "blocked",
		ReasonStale:            "stale",
		ReasonCarriedOver:      "carried over",
		ReasonRecentlyActive:   "recently active",
	}
	for reason, label := range want {
		if got := reason.String(); got != label {
			t.Errorf("%v.String() = %q, want %q", reason, got, label)
		}
		if got := reason.Label(); got != label {
			t.Errorf("%v.Label() = %q, want %q", reason, got, label)
		}
		if !reason.IsValid() {
			t.Errorf("%v.IsValid() = false, want true", reason)
		}
	}

	all := AllRecommendationReasons()
	if len(all) != len(want) {
		t.Fatalf("AllRecommendationReasons() returned %d entries, want %d", len(all), len(want))
	}
	seen := make(map[RecommendationReason]bool, len(all))
	for _, r := range all {
		if seen[r] {
			t.Errorf("AllRecommendationReasons() repeats %v", r)
		}
		seen[r] = true
		if _, ok := want[r]; !ok {
			t.Errorf("AllRecommendationReasons() includes unknown reason %v", r)
		}
	}
	for r := range want {
		if !seen[r] {
			t.Errorf("AllRecommendationReasons() missing %v", r)
		}
	}
}

func TestRecommendationReasonIsValidRejectsUnknown(t *testing.T) {
	if RecommendationReason("").IsValid() {
		t.Error(`RecommendationReason("").IsValid() = true, want false`)
	}
	if RecommendationReason("nope").IsValid() {
		t.Error(`RecommendationReason("nope").IsValid() = true, want false`)
	}
	if RecommendationReason("Overdue").IsValid() {
		t.Error(`RecommendationReason("Overdue") capitalization variant must not be valid`)
	}
}

func TestRecommendationKindContract(t *testing.T) {
	want := map[RecommendationKind]string{
		RecommendationKindNone:          "",
		RecommendationKindWorkOnTask:    "work_on_task",
		RecommendationKindTriageInbox:   "triage_inbox",
		RecommendationKindReviewBlocked: "review_blocked",
		RecommendationKindPlanToday:     "plan_today",
		RecommendationKindAllDone:       "all_done",
	}
	for kind, str := range want {
		if got := kind.String(); got != str {
			t.Errorf("%v.String() = %q, want %q", kind, got, str)
		}
		if !kind.IsValid() {
			t.Errorf("%v.IsValid() = false, want true", kind)
		}
	}

	all := AllRecommendationKinds()
	// Enumerated set excludes the zero value because callers should never
	// surface RecommendationKindNone as an active recommendation.
	if len(all) != len(want)-1 {
		t.Fatalf("AllRecommendationKinds() returned %d entries, want %d", len(all), len(want)-1)
	}
	for _, k := range all {
		if k == RecommendationKindNone {
			t.Errorf("AllRecommendationKinds() must not include the zero value")
		}
		if _, ok := want[k]; !ok {
			t.Errorf("AllRecommendationKinds() includes unknown kind %v", k)
		}
	}
}

func TestRecommendationKindIsValidRejectsUnknown(t *testing.T) {
	if RecommendationKind("work-on-task").IsValid() {
		t.Error("hyphenated variant must not be valid (engine uses underscores)")
	}
	if RecommendationKind("triage").IsValid() {
		t.Error("partial label must not be valid")
	}
}

func TestRecommendedActionHasReason(t *testing.T) {
	action := &RecommendedAction{
		Kind:    RecommendationKindWorkOnTask,
		Reasons: []RecommendationReason{ReasonHighPriority, ReasonDueToday},
	}
	if !action.HasReason(ReasonHighPriority) {
		t.Error("HasReason(ReasonHighPriority) = false, want true")
	}
	if !action.HasReason(ReasonDueToday) {
		t.Error("HasReason(ReasonDueToday) = false, want true")
	}
	if action.HasReason(ReasonStale) {
		t.Error("HasReason(ReasonStale) = true, want false")
	}
}

func TestRecommendedActionHasReasonNilReceiver(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil receiver panicked: %v", r)
		}
	}()
	var action *RecommendedAction
	if action.HasReason(ReasonOverdue) {
		t.Error("nil action.HasReason() = true, want false")
	}
}

func TestRecommendedActionIsEmpty(t *testing.T) {
	var zero RecommendedAction
	if !zero.IsEmpty() {
		t.Error("zero-value RecommendedAction.IsEmpty() = false, want true")
	}

	var nilAction *RecommendedAction
	if !nilAction.IsEmpty() {
		t.Error("nil RecommendedAction.IsEmpty() = false, want true")
	}

	withKind := &RecommendedAction{Kind: RecommendationKindPlanToday}
	if withKind.IsEmpty() {
		t.Error("RecommendedAction with kind should not be empty")
	}

	withReason := &RecommendedAction{Reasons: []RecommendationReason{ReasonStale}}
	if withReason.IsEmpty() {
		t.Error("RecommendedAction with a reason should not be empty")
	}
}

func TestRecommendedActionCarriesTaskAndProject(t *testing.T) {
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	generatedAt := time.Date(2026, 6, 24, 9, 30, 0, 0, time.UTC)

	action := RecommendedAction{
		Kind:        RecommendationKindWorkOnTask,
		Task:        task,
		Project:     project,
		Reasons:     []RecommendationReason{ReasonPlannedToday, ReasonHighPriority},
		GeneratedAt: generatedAt,
	}

	if action.Kind != RecommendationKindWorkOnTask {
		t.Errorf("Kind = %q, want %q", action.Kind, RecommendationKindWorkOnTask)
	}
	if action.Task != task {
		t.Errorf("Task pointer not preserved: got %p, want %p", action.Task, task)
	}
	if action.Project != project {
		t.Errorf("Project pointer not preserved: got %p, want %p", action.Project, project)
	}
	if !action.GeneratedAt.Equal(generatedAt) {
		t.Errorf("GeneratedAt = %v, want %v", action.GeneratedAt, generatedAt)
	}
	if len(action.Reasons) != 2 {
		t.Fatalf("Reasons length = %d, want 2", len(action.Reasons))
	}
}

func TestCockpitContractEnumsRejectUnknown(t *testing.T) {
	if !CockpitTargetTask.IsValid() || !CockpitTargetSurface.IsValid() {
		t.Fatal("known cockpit target kinds must be valid")
	}
	if CockpitTargetKind("taskish").IsValid() {
		t.Fatal("unknown cockpit target kind must be invalid")
	}
	if !CockpitCommandOpenTask.IsValid() || !CockpitCommandOpenPipelineHub.IsValid() {
		t.Fatal("known cockpit commands must be valid")
	}
	if CockpitCommand("open").IsValid() {
		t.Fatal("unknown cockpit command must be invalid")
	}
	if !CockpitProgressPlanned.IsValid() || !CockpitProgressComplete.IsValid() {
		t.Fatal("known cockpit progress states must be valid")
	}
	if CockpitProgressKind("done").IsValid() {
		t.Fatal("unknown cockpit progress state must be invalid")
	}
	if !CockpitEmptyStateFirstRun.IsValid() || !CockpitEmptyStateBacklogReady.IsValid() {
		t.Fatal("known cockpit empty states must be valid")
	}
	if CockpitEmptyState("empty").IsValid() {
		t.Fatal("unknown cockpit empty state must be invalid")
	}
}

func TestCockpitActionHintAndFooterOrdering(t *testing.T) {
	action := CockpitAction{Command: CockpitCommandPlanToday, Key: "p", Label: "plan"}
	if got := action.Hint(); got != "p plan" {
		t.Fatalf("Hint() = %q, want %q", got, "p plan")
	}
	if action.IsZero() {
		t.Fatal("populated action reported zero")
	}
	if !(CockpitAction{}).IsZero() {
		t.Fatal("zero action reported populated")
	}

	footer := CockpitFooterActions{
		Contextual: []CockpitAction{actionCompleteTask(), actionPrioritizeTask(), actionQuickCapture()},
		Escape:     cockpitEscapeActions(),
	}
	assertCockpitCommands(t, footer.All(), []CockpitCommand{
		CockpitCommandCompleteTask,
		CockpitCommandPrioritizeTask,
		CockpitCommandQuickCapture,
		CockpitCommandOpenPipelineHub,
		CockpitCommandOpenCommandPalette,
		CockpitCommandShowHelp,
	})
}

func TestBuildCockpitRecommendationWorkOnTaskContract(t *testing.T) {
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	reasons := []RecommendationReason{ReasonPlannedToday, ReasonHighPriority}
	action := RecommendedAction{
		Kind:    RecommendationKindWorkOnTask,
		Task:    task,
		Project: project,
		Reasons: reasons,
	}

	got := BuildCockpitRecommendation(action, CockpitContext{
		HasProjects: true,
		OpenLoops:   OpenLoops{TodayDone: 1, TodayTotal: 2, HasTasks: true},
	})

	if got.Kind != RecommendationKindWorkOnTask {
		t.Fatalf("Kind = %v, want WorkOnTask", got.Kind)
	}
	if got.Target.Kind != CockpitTargetTask || got.Target.Task != task || got.Target.Project != project {
		t.Fatalf("Target = %+v, want task/project pointers preserved", got.Target)
	}
	if got.Target.IsEmpty() {
		t.Fatal("task target reported empty")
	}
	if got.Progress.Kind != CockpitProgressPlanned || got.Progress.Done != 1 || got.Progress.Total != 2 {
		t.Fatalf("Progress = %+v, want planned 1/2", got.Progress)
	}
	if got.EmptyState != CockpitEmptyStateNone {
		t.Fatalf("EmptyState = %v, want none", got.EmptyState)
	}
	if got.PrimaryAction.Command != CockpitCommandOpenTask {
		t.Fatalf("PrimaryAction = %+v, want open task", got.PrimaryAction)
	}
	if got.SecondaryAction.Command != CockpitCommandCompleteTask {
		t.Fatalf("SecondaryAction = %+v, want complete task", got.SecondaryAction)
	}
	assertCockpitCommands(t, got.Footer.Contextual, []CockpitCommand{
		CockpitCommandCompleteTask,
		CockpitCommandPrioritizeTask,
		CockpitCommandQuickCapture,
	})

	reasons[0] = ReasonDueToday
	if got.Rationale[0] != ReasonPlannedToday {
		t.Fatalf("Rationale aliases input slice; got %v after mutation", got.Rationale)
	}
}

func TestBuildCockpitRecommendationCharacterizesCurrentSurfaceStates(t *testing.T) {
	project, err := models.NewProject(uuid.New().String(), "P", "")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, "Blocked")
	if err != nil {
		t.Fatal(err)
	}
	blocked.WorkflowState = models.StateWaiting

	cases := []struct {
		name          string
		action        RecommendedAction
		context       CockpitContext
		wantTarget    CockpitTargetKind
		wantProgress  CockpitProgressKind
		wantEmpty     CockpitEmptyState
		wantPrimary   CockpitCommand
		wantSecondary CockpitCommand
		wantFooter    []CockpitCommand
	}{
		{
			name: "review blocked",
			action: RecommendedAction{
				Kind:    RecommendationKindReviewBlocked,
				Task:    blocked,
				Project: project,
				Reasons: []RecommendationReason{ReasonBlocked},
			},
			context:       CockpitContext{HasProjects: true, OpenLoops: OpenLoops{HasTasks: true}},
			wantTarget:    CockpitTargetTask,
			wantProgress:  CockpitProgressUnplanned,
			wantEmpty:     CockpitEmptyStateNone,
			wantPrimary:   CockpitCommandUnblockTask,
			wantSecondary: CockpitCommandOpenTask,
			wantFooter: []CockpitCommand{
				CockpitCommandCompleteTask,
				CockpitCommandPrioritizeTask,
				CockpitCommandQuickCapture,
			},
		},
		{
			name:          "inbox only",
			action:        RecommendedAction{Kind: RecommendationKindTriageInbox, Reasons: []RecommendationReason{ReasonInboxNeedsTriage}},
			context:       CockpitContext{HasProjects: true, OpenLoops: OpenLoops{Inbox: 3, HasTasks: true}},
			wantTarget:    CockpitTargetSurface,
			wantProgress:  CockpitProgressUnplanned,
			wantEmpty:     CockpitEmptyStateInboxOnly,
			wantPrimary:   CockpitCommandTriageInbox,
			wantSecondary: CockpitCommandQuickCapture,
			wantFooter: []CockpitCommand{
				CockpitCommandQuickCapture,
				CockpitCommandSwitchTab,
				CockpitCommandCloseView,
			},
		},
		{
			name:          "plan today",
			action:        RecommendedAction{Kind: RecommendationKindPlanToday},
			context:       CockpitContext{HasProjects: true, OpenLoops: OpenLoops{HasTasks: true}},
			wantTarget:    CockpitTargetSurface,
			wantProgress:  CockpitProgressUnplanned,
			wantEmpty:     CockpitEmptyStateBacklogReady,
			wantPrimary:   CockpitCommandPlanToday,
			wantSecondary: CockpitCommandQuickCapture,
			wantFooter: []CockpitCommand{
				CockpitCommandQuickCapture,
				CockpitCommandSwitchTab,
				CockpitCommandCloseView,
			},
		},
		{
			name:          "first run",
			action:        RecommendedAction{Kind: RecommendationKindAllDone},
			context:       CockpitContext{},
			wantTarget:    CockpitTargetSurface,
			wantProgress:  CockpitProgressUnplanned,
			wantEmpty:     CockpitEmptyStateFirstRun,
			wantPrimary:   CockpitCommandNewProject,
			wantSecondary: CockpitCommandQuickCapture,
			wantFooter: []CockpitCommand{
				CockpitCommandQuickCapture,
				CockpitCommandSwitchTab,
				CockpitCommandCloseView,
			},
		},
		{
			name:          "today cleared",
			action:        RecommendedAction{Kind: RecommendationKindAllDone},
			context:       CockpitContext{HasProjects: true, OpenLoops: OpenLoops{TodayDone: 2, TodayTotal: 2, HasTasks: true}},
			wantTarget:    CockpitTargetSurface,
			wantProgress:  CockpitProgressComplete,
			wantEmpty:     CockpitEmptyStateTodayCleared,
			wantPrimary:   CockpitCommandPlanMore,
			wantSecondary: CockpitCommandQuickCapture,
			wantFooter: []CockpitCommand{
				CockpitCommandPlanMore,
				CockpitCommandQuickCapture,
				CockpitCommandCloseView,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildCockpitRecommendation(tc.action, tc.context)
			if got.Target.Kind != tc.wantTarget {
				t.Fatalf("Target.Kind = %v, want %v", got.Target.Kind, tc.wantTarget)
			}
			if got.Progress.Kind != tc.wantProgress {
				t.Fatalf("Progress.Kind = %v, want %v", got.Progress.Kind, tc.wantProgress)
			}
			if got.EmptyState != tc.wantEmpty {
				t.Fatalf("EmptyState = %v, want %v", got.EmptyState, tc.wantEmpty)
			}
			if got.PrimaryAction.Command != tc.wantPrimary {
				t.Fatalf("PrimaryAction.Command = %v, want %v", got.PrimaryAction.Command, tc.wantPrimary)
			}
			if got.SecondaryAction.Command != tc.wantSecondary {
				t.Fatalf("SecondaryAction.Command = %v, want %v", got.SecondaryAction.Command, tc.wantSecondary)
			}
			assertCockpitCommands(t, got.Footer.Contextual, tc.wantFooter)
			assertCockpitCommands(t, got.Footer.Escape, []CockpitCommand{
				CockpitCommandOpenPipelineHub,
				CockpitCommandOpenCommandPalette,
				CockpitCommandShowHelp,
			})
		})
	}
}

func assertCockpitCommands(t *testing.T, got []CockpitAction, want []CockpitCommand) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("command count = %d, want %d (actions: %+v)", len(got), len(want), got)
	}
	for i, action := range got {
		if action.Command != want[i] {
			t.Fatalf("command[%d] = %v, want %v (actions: %+v)", i, action.Command, want[i], got)
		}
		if !action.Command.IsValid() {
			t.Fatalf("command[%d] = %v is invalid", i, action.Command)
		}
	}
}
