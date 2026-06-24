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
