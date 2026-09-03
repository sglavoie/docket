package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/query"
)

// This file covers one indicator at a time: what each signal spells, which
// states may wear it, and the two orders the set is held to — the order they
// render in and the order they are dropped in. What a whole row does with them
// under width pressure is in outliner_render_rows_pressure_test.go, and the
// reserved row segments are in outliner_render_rows_test.go.

func TestDeferIndicatorSeparatesPendingFromWoken(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		deferred time.Time
		want     string
	}{
		{"pending", now.AddDate(0, 0, 3), "defer "},
		{"woken inside the window", now.Add(-30 * time.Hour), "woke "},
		{"woken past the window", now.Add(-query.DeferWakeWindow - time.Hour), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{ID: "task", Title: "Task", DeferredUntil: &tc.deferred}
			plain := ansi.Strip(formatDeferIndicator(task, now))
			if tc.want == "" {
				if plain != "" {
					t.Fatalf("lapsed deferral past the wake window still rendered %q", plain)
				}
				return
			}
			if !strings.HasPrefix(plain, tc.want) {
				t.Fatalf("defer indicator %q does not start with %q", plain, tc.want)
			}
		})
	}
}

func TestDeferCountIndicatorStartsAtThirdDeferral(t *testing.T) {
	for _, tc := range []struct {
		count         int
		wantRepeated  bool
		wantIndicator string
	}{
		{count: repeatedDeferralThreshold - 1, wantRepeated: false, wantIndicator: ""},
		{count: repeatedDeferralThreshold, wantRepeated: true, wantIndicator: "deferred ×3"},
		{count: 12, wantRepeated: true, wantIndicator: "deferred ×12"},
	} {
		task := &models.Task{DeferCount: tc.count}
		if got := taskHasRepeatedDeferrals(task); got != tc.wantRepeated {
			t.Errorf("count %d repeated = %v, want %v", tc.count, got, tc.wantRepeated)
		}
		if got := ansi.Strip(formatDeferCountIndicator(task)); got != tc.wantIndicator {
			t.Errorf("count %d indicator = %q, want %q", tc.count, got, tc.wantIndicator)
		}
	}
}

func TestRepeatedDeferralSurfacesIgnoreHiddenDeferColumn(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	pending := now.Add(24 * time.Hour)
	outliner := &OutlinerModel{
		columnConfig: NewColumnConfig(map[string]bool{ColDefer: false}, nil, nil),
	}

	for _, tc := range []struct {
		count int
		want  string
	}{
		{count: repeatedDeferralThreshold - 1, want: ""},
		{count: repeatedDeferralThreshold, want: "deferred ×3"},
	} {
		task := &models.Task{DeferCount: tc.count, DeferredUntil: &pending}
		segments := outliner.renderTaskIndicatorSegments(outlinerRow{task: task})
		if segments.deferred != "" {
			t.Fatalf("count %d hidden defer column rendered defer date %q", tc.count, ansi.Strip(segments.deferred))
		}

		rowCount := ansi.Strip(segments.deferCount)
		peekCount := ansi.Strip(inlinePeekDatesLine(task, now, 80))
		if rowCount != tc.want || peekCount != tc.want {
			t.Fatalf("count %d hidden-column surfaces = row %q, Peek %q; want %q", tc.count, rowCount, peekCount, tc.want)
		}
	}
}

func TestWaitingAgeIndicatorReadsTheStateClock(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	waiting := &models.Task{
		ID:             "blocked",
		Title:          "Blocked",
		WorkflowState:  models.StateWaiting,
		StateChangedAt: now.AddDate(0, 0, -12),
		UpdatedAt:      now,
	}
	if got := ansi.Strip(formatWaitingAgeIndicator(waiting, now)); got != "waiting 12d" {
		t.Fatalf("waiting age = %q, want %q; a comment must not reset it", got, "waiting 12d")
	}

	for _, state := range []models.WorkflowState{models.StateBacklog, models.StatePaused, models.StateDone} {
		other := &models.Task{
			ID:             "other",
			Title:          "Other",
			WorkflowState:  state,
			StateChangedAt: now.AddDate(0, 0, -12),
		}
		if got := formatWaitingAgeIndicator(other, now); got != "" {
			t.Fatalf("state %q wore a waiting age: %q", state, ansi.Strip(got))
		}
	}
}

func TestDoingAgeIndicatorReadsTheStateClock(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	doing := &models.Task{
		ID:             "active",
		Title:          "Active",
		WorkflowState:  models.StateDoing,
		StateChangedAt: now.AddDate(0, 0, -9),
		UpdatedAt:      now,
	}
	if got := ansi.Strip(formatDoingAgeIndicator(doing, now)); got != "doing 9d" {
		t.Fatalf("doing age = %q, want %q; touching the task must not reset it", got, "doing 9d")
	}

	for _, state := range []models.WorkflowState{models.StateBacklog, models.StateWaiting, models.StateDone} {
		other := &models.Task{
			ID:             "other",
			Title:          "Other",
			WorkflowState:  state,
			StateChangedAt: now.AddDate(0, 0, -9),
		}
		if got := formatDoingAgeIndicator(other, now); got != "" {
			t.Fatalf("state %q wore a doing age: %q", state, ansi.Strip(got))
		}
	}
}

func TestTaskIndicatorSegmentsRenderDoingInOrder(t *testing.T) {
	segments := taskIndicatorSegments{
		deferCount:   "deferCount",
		recency:      "recency",
		waiting:      "waiting",
		doing:        "doing",
		check:        "check",
		due:          "due",
		deferred:     "deferred",
		checklist:    "checklist",
		comments:     "comments",
		attachments:  "attachments",
		dependencies: "dependencies",
	}
	plain := ansi.Strip(segments.render())
	previous := -1
	for _, part := range []string{
		"recency", "waiting", "doing", "check", "due", "deferred", "deferCount", "checklist", "comments", "attachments", "dependencies",
	} {
		index := strings.Index(plain, part)
		if index <= previous {
			t.Fatalf("indicator %q rendered out of order in %q", part, plain)
		}
		previous = index
	}
}

func TestIndicatorDropLadderOrder(t *testing.T) {
	segments := taskIndicatorSegments{
		deferCount:   "deferCount",
		recency:      "recency",
		waiting:      "waiting",
		doing:        "doing",
		check:        "check",
		due:          "due",
		deferred:     "deferred",
		checklist:    "checklist",
		comments:     "comments",
		attachments:  "attachments",
		dependencies: "dependencies",
	}
	want := segments
	if got := len(indicatorDropLadder); got != 11 {
		t.Fatalf("indicator drop ladder has %d entries, want 11", got)
	}
	for index, drop := range indicatorDropLadder {
		switch index {
		case 0:
			want.deferCount = ""
		case 1:
			want.checklist = ""
		case 2:
			want.comments = ""
		case 3:
			want.attachments = ""
		case 4:
			want.dependencies = ""
		case 5:
			want.recency = ""
		case 6:
			want.deferred = ""
		case 7:
			want.waiting = ""
		case 8:
			want.due = ""
		case 9:
			want.doing = ""
		case 10:
			want.check = ""
		}
		drop(&segments)
		if segments != want {
			t.Fatalf("indicator drop %d produced %+v, want %+v", index+1, segments, want)
		}
	}
}

func TestCheckBackIndicatorCountsTowardChasing(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		due  *time.Time
		want string
	}{
		{"no date to chase", nil, ""},
		{"overdue", timePtr(now.AddDate(0, 0, -5)), "check +5d"},
		{"today", timePtr(now.AddDate(0, 0, 0)), "check today"},
		{"upcoming", timePtr(now.AddDate(0, 0, 3)), "check 3d"},
		{"late in the day still counts as that day", timePtr(now.Add(11 * time.Hour)), "check today"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				ID:             "blocked",
				Title:          "Blocked",
				WorkflowState:  models.StateWaiting,
				StateChangedAt: now.AddDate(0, 0, -1),
				DueDate:        tc.due,
			}
			// The chip's padding cells are presentation, not content: trim them
			// so the case list stays a statement about the text. The empty case
			// stays honest because TrimSpace("") is "".
			if got := strings.TrimSpace(ansi.Strip(formatCheckBackIndicator(task, now))); got != tc.want {
				t.Fatalf("check-back = %q, want %q", got, tc.want)
			}
		})
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
