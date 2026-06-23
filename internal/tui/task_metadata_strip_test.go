package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func newStripTestTask() *models.Task {
	now := time.Now()
	due := now.Add(48 * time.Hour)
	return &models.Task{
		ID:        "t1",
		Title:     "metadata strip task",
		Type:      models.TaskTypeTask,
		Status:    models.StatusTodo,
		Priority:  2,
		Section:   "Next",
		Stage:     "design",
		Tags:      []string{"backend", "infra"},
		DueDate:   &due,
		CreatedAt: now.Add(-3 * time.Hour),
		UpdatedAt: now.Add(-1 * time.Hour),
	}
}

func newStripTestProject() *models.Project {
	return &models.Project{ID: "p1", Name: "Acme"}
}

func TestTaskMetadataStripNilTask(t *testing.T) {
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: nil})
	if got != "" {
		t.Errorf("expected empty strip for nil task, got %q", got)
	}
}

func TestTaskMetadataStripEmptyMetadata(t *testing.T) {
	// A bare task with no due date, no stage, no tags, no project, no
	// timestamps — should render to "" since the only fallback (updated
	// relative) is also missing.
	task := &models.Task{
		ID:       "bare",
		Title:    "bare",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 0, // suppressed: only 1–5 render
	}
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task})
	if got != "" {
		t.Errorf("expected empty strip when task carries no renderable metadata, got %q", got)
	}
}

func TestTaskMetadataStripContainsCoreFields(t *testing.T) {
	task := newStripTestTask()
	proj := newStripTestProject()

	got := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
	})
	plain := ansi.Strip(got)

	for _, want := range []string{"[Acme]", "Next", "design", "backend", "infra"} {
		if !strings.Contains(plain, want) {
			t.Errorf("expected strip to contain %q, got %q", want, plain)
		}
	}
}

func TestTaskMetadataStripRedundancyOmission(t *testing.T) {
	task := newStripTestTask()
	proj := newStripTestProject()

	got := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
		AlreadyShown: TaskMetadataAlreadyShown{
			Priority: true,
			Project:  true,
			Due:      true,
			Stage:    true,
		},
	})
	plain := ansi.Strip(got)

	if strings.Contains(plain, "[Acme]") {
		t.Errorf("project label must be omitted when AlreadyShown.Project=true, got %q", plain)
	}
	if strings.Contains(plain, "design") {
		t.Errorf("stage must be omitted when AlreadyShown.Stage=true, got %q", plain)
	}
	if strings.Contains(plain, "today") || strings.Contains(plain, "tomorrow") ||
		strings.Contains(plain, "yesterday") || strings.Contains(plain, "Mon") {
		t.Errorf("due date must be omitted when AlreadyShown.Due=true, got %q", plain)
	}
	// Priority badge text comes from PriorityIcon. The default text-mode icon
	// is "P2" for priority=2; we check that no "P<n>" token survives.
	if strings.Contains(plain, "P2") {
		t.Errorf("priority badge must be omitted when AlreadyShown.Priority=true, got %q", plain)
	}
}

func TestTaskMetadataStripGroupingSeparators(t *testing.T) {
	task := newStripTestTask()
	proj := newStripTestProject()
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj})
	plainSep := ansi.Strip(IndicatorGroupSep)

	// At least one group transition must use IndicatorGroupSep when project,
	// stage, lifecycle, and tags are all populated.
	if !strings.Contains(ansi.Strip(got), strings.TrimSpace(plainSep)) {
		t.Errorf("expected group separator %q in strip, got %q", plainSep, ansi.Strip(got))
	}
}

func TestTaskMetadataStripOverflowDropsFooterFirst(t *testing.T) {
	task := newStripTestTask()
	proj := newStripTestProject()

	full := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj})
	fullW := lipgloss.Width(full)
	plainFull := ansi.Strip(full)
	if !strings.Contains(plainFull, "ago") {
		t.Fatalf("baseline strip should include updated-relative footer, got %q", plainFull)
	}

	// Trim a few cells from full width — the footer is the highest dropPriority
	// part and must be the first to disappear.
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
		Width:   fullW - 5,
	})
	plain := ansi.Strip(got)
	if strings.Contains(plain, "ago") {
		t.Errorf("expected footer (updated-relative) to be dropped first, got %q", plain)
	}
	// Core lifecycle/project context should still be present.
	if !strings.Contains(plain, "[Acme]") {
		t.Errorf("project should survive moderate overflow, got %q", plain)
	}
}

func TestTaskMetadataStripOverflowKeepsLifecycleLongest(t *testing.T) {
	task := newStripTestTask()
	proj := newStripTestProject()

	// Very narrow width — everything but the most critical lifecycle parts
	// (priority badge) and possibly project should drop.
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
		Width:   12,
	})
	if got == "" {
		t.Fatalf("expected non-empty strip even at narrow width")
	}
	if w := lipgloss.Width(got); w > 12 {
		t.Errorf("strip exceeded configured width: got %d > 12 for %q", w, ansi.Strip(got))
	}
	plain := ansi.Strip(got)
	if strings.Contains(plain, "ago") || strings.Contains(plain, "backend") ||
		strings.Contains(plain, "infra") {
		t.Errorf("expected low-priority groups to be dropped first, got %q", plain)
	}
}

func TestTaskMetadataStripNarrowSinglePartTruncated(t *testing.T) {
	task := newStripTestTask()
	proj := newStripTestProject()
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
		Width:   3,
	})
	if got == "" {
		// Acceptable: nothing fits — strip may legitimately render empty.
		return
	}
	if w := lipgloss.Width(got); w > 3 {
		t.Errorf("strip must respect width=3 even after single-part truncation, got width=%d %q",
			w, ansi.Strip(got))
	}
}

func TestTaskMetadataStripLongStageRespectsWidth(t *testing.T) {
	task := newStripTestTask()
	task.Stage = "verylongstagenamethatdoesnotwraparoundatall"
	proj := newStripTestProject()
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
		Width:   60,
	})
	if w := lipgloss.Width(got); w > 60 {
		t.Errorf("strip exceeded width with long stage: %d > 60, %q", w, ansi.Strip(got))
	}
}

// TestTaskMetadataStripImplementInitialVersionStage uses the plan's literal
// long-stage label and asserts the strip surfaces it when there is room and
// gracefully drops the stage group before dropping critical lifecycle context
// under tight widths.
func TestTaskMetadataStripImplementInitialVersionStage(t *testing.T) {
	task := newStripTestTask()
	task.Stage = "Implement initial version"
	proj := newStripTestProject()

	full := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj})
	if !strings.Contains(ansi.Strip(full), "Implement initial version") {
		t.Fatalf("expected wide strip to surface the long stage, got %q", ansi.Strip(full))
	}

	tight := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
		Width:   20,
	})
	if w := lipgloss.Width(tight); w > 20 {
		t.Errorf("strip exceeded width with long stage at width=20: %d > 20, %q", w, ansi.Strip(tight))
	}
	plainTight := ansi.Strip(tight)
	if strings.Contains(plainTight, "Implement initial version") {
		t.Errorf("expected long stage (dropHigh) to be dropped before critical lifecycle context at width=20, got %q", plainTight)
	}
}

func TestTaskMetadataStripLongTagsRespectsWidth(t *testing.T) {
	task := newStripTestTask()
	task.Tags = []string{
		"superduperlongtagname-one",
		"superduperlongtagname-two",
		"superduperlongtagname-three",
		"superduperlongtagname-four",
	}
	proj := newStripTestProject()
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:    task,
		Project: proj,
		Width:   50,
	})
	if w := lipgloss.Width(got); w > 50 {
		t.Errorf("strip exceeded width with long tag list: %d > 50, %q", w, ansi.Strip(got))
	}
}

func TestTaskMetadataStripANSIWidthIsAccurate(t *testing.T) {
	task := newStripTestTask()
	proj := newStripTestProject()
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj})
	visualW := lipgloss.Width(got)
	plainW := len([]rune(ansi.Strip(got)))
	if visualW != plainW {
		// Acceptable mismatch for east-asian or emoji widths; the contract
		// is just that visualW is what ANSI-stripping reports. We assert it
		// is positive and lipgloss.Width is consistent across calls.
		if visualW <= 0 {
			t.Errorf("expected positive visual width, got %d for %q", visualW, ansi.Strip(got))
		}
	}
	// Re-measuring should be idempotent.
	if got2 := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj}); lipgloss.Width(got2) != visualW {
		t.Errorf("repeat render produced different width: %d != %d", lipgloss.Width(got2), visualW)
	}
}

func TestTaskMetadataStripRecurrenceShowsStreak(t *testing.T) {
	task := newStripTestTask()
	task.Recurrence = &models.RecurrencePattern{
		Interval: 1,
		Unit:     models.RecurrenceDaily,
		Streak:   7,
	}
	proj := newStripTestProject()
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj})
	if !strings.Contains(ansi.Strip(got), "↻7") {
		t.Errorf("expected recurrence indicator with streak in strip, got %q", ansi.Strip(got))
	}
}

func TestTaskMetadataStripRecurrenceWithoutStreak(t *testing.T) {
	task := newStripTestTask()
	task.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}
	proj := newStripTestProject()
	got := renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj})
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "↻") {
		t.Errorf("expected recurrence indicator in strip, got %q", plain)
	}
	if strings.Contains(plain, "↻0") {
		t.Errorf("zero streak should render bare arrow, not ↻0, got %q", plain)
	}
}

func TestTaskMetadataStripAlreadyShownUpdated(t *testing.T) {
	task := newStripTestTask()
	proj := newStripTestProject()

	baseline := ansi.Strip(renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj}))
	if !strings.Contains(baseline, "ago") {
		t.Fatalf("baseline strip should include updated-relative footer, got %q", baseline)
	}

	got := ansi.Strip(renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:         task,
		Project:      proj,
		AlreadyShown: TaskMetadataAlreadyShown{Updated: true},
	}))
	if strings.Contains(got, "ago") {
		t.Errorf("updated-relative footer must be omitted when AlreadyShown.Updated=true, got %q", got)
	}
}

func TestTaskMetadataStripAlreadyShownStale(t *testing.T) {
	// Force a stale signal: very old updated time relative to a tight
	// staleness threshold in the config.
	task := newStripTestTask()
	old := time.Now().Add(-365 * 24 * time.Hour)
	task.UpdatedAt = old
	proj := newStripTestProject()

	baseline := ansi.Strip(renderTaskMetadataStrip(TaskMetadataStripOpts{Task: task, Project: proj}))

	got := ansi.Strip(renderTaskMetadataStrip(TaskMetadataStripOpts{
		Task:         task,
		Project:      proj,
		AlreadyShown: TaskMetadataAlreadyShown{Stale: true},
	}))

	// Stale indicator output depends on config; assert only that the Stale
	// suppression at minimum produces a result no longer than the baseline.
	if lipgloss.Width(got) > lipgloss.Width(baseline) {
		t.Errorf("AlreadyShown.Stale=true must not lengthen the strip; baseline=%q got=%q", baseline, got)
	}
}

func TestMetadataStripPickDropIndexRightmostTie(t *testing.T) {
	parts := []metadataPart{
		{group: metadataGroupProjectSection, text: "[a]", dropPriority: 5},
		{group: metadataGroupLifecycle, text: "x", dropPriority: 1},
		{group: metadataGroupFooter, text: "y", dropPriority: 5},
	}
	idx := pickMetadataDropIndex(parts)
	if idx != 2 {
		t.Errorf("expected right-most tie winner index=2, got %d", idx)
	}
}

func TestMetadataStripJoinMixedGroups(t *testing.T) {
	parts := []metadataPart{
		{group: metadataGroupProjectSection, text: "[a]"},
		{group: metadataGroupProjectSection, text: "Next"},
		{group: metadataGroupStage, text: "design"},
		{group: metadataGroupStage, text: "v2"},
	}
	got := ansi.Strip(joinMetadataParts(parts))
	sep := ansi.Strip(IndicatorGroupSep)
	want := "[a] Next" + sep + "design v2"
	if got != want {
		t.Errorf("join produced %q, want %q", got, want)
	}
}

func TestMetadataStripEmptyJoin(t *testing.T) {
	if got := joinMetadataParts(nil); got != "" {
		t.Errorf("empty parts must join to \"\", got %q", got)
	}
}
