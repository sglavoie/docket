package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// This file covers one indicator at a time: what each signal spells, which
// states may wear it, and the two orders the set is held to — the order they
// render in and the order they are dropped in. The row keeps three indicators
// now, so the signals it shed are covered here too — by the Peek line that
// inherited them, which is the whole justification for dropping them from the
// row. What a whole row does with them under width pressure is in
// outliner_render_rows_pressure_test.go, and the reserved row segments are in
// outliner_render_rows_test.go.

// TestPeekDatesLineSeparatesPendingFromWokenDeferral is the row's defer
// indicator, relocated. A deferral has two halves that never co-occur, and Peek
// must keep telling them apart now that it is the only place either is said.
func TestPeekDatesLineSeparatesPendingFromWokenDeferral(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		deferred time.Time
		want     string
		absent   string
	}{
		{"pending", now.AddDate(0, 0, 3), "defer ", "woke "},
		{"woken inside the window", now.Add(-30 * time.Hour), "woke ", "defer "},
		{"woken past the window", now.Add(-query.DeferWakeWindow - time.Hour), "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{ID: "task", Title: "Task", DeferredUntil: &tc.deferred}
			plain := ansi.Strip(inlinePeekDatesLine(task, now, 80))
			if tc.want == "" {
				if strings.Contains(plain, "defer") || strings.Contains(plain, "woke") {
					t.Fatalf("lapsed deferral past the wake window still rendered %q", plain)
				}
				return
			}
			if !strings.Contains(plain, tc.want) {
				t.Fatalf("Peek dates line %q does not name %q", plain, tc.want)
			}
			if tc.absent != "" && strings.Contains(plain, tc.absent) {
				t.Fatalf("Peek dates line %q named both halves of the deferral", plain)
			}
		})
	}
}

// TestRepeatedDeferralCountStartsAtThirdDeferralInPeek keeps the threshold that
// decides when a repeated deferral is worth saying. Only the surface moved: the
// count is a Peek field now, so the predicate and the line are checked together.
func TestRepeatedDeferralCountStartsAtThirdDeferralInPeek(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		count        int
		wantRepeated bool
		wantField    string
	}{
		{count: repeatedDeferralThreshold - 1, wantRepeated: false, wantField: ""},
		{count: repeatedDeferralThreshold, wantRepeated: true, wantField: "deferred ×3"},
		{count: 12, wantRepeated: true, wantField: "deferred ×12"},
	} {
		task := &models.Task{DeferCount: tc.count}
		if got := taskHasRepeatedDeferrals(task); got != tc.wantRepeated {
			t.Errorf("count %d repeated = %v, want %v", tc.count, got, tc.wantRepeated)
		}
		plain := ansi.Strip(inlinePeekDatesLine(task, now, 80))
		if tc.wantField == "" {
			if strings.Contains(plain, "deferred") {
				t.Errorf("count %d named a repeated deferral: %q", tc.count, plain)
			}
			continue
		}
		if !strings.Contains(plain, tc.wantField) {
			t.Errorf("count %d Peek line %q does not name %q", tc.count, plain, tc.wantField)
		}
	}
}

// TestTaskRowCarriesNoDeferralAtAnyCount is the removal stated as a rule rather
// than as an absence. The row's indicator set is three signals wide and a
// deferral — pending, woken, or repeated — is none of them, whatever the count.
func TestTaskRowCarriesNoDeferralAtAnyCount(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	pending := now.Add(24 * time.Hour)
	outliner := &OutlinerModel{columnConfig: listsurface.NewColumnConfig(nil, nil, nil)}

	for _, count := range []int{0, repeatedDeferralThreshold - 1, repeatedDeferralThreshold, 12} {
		task := &models.Task{DeferCount: count, DeferredUntil: &pending}
		segments := outliner.renderTaskIndicatorSegments(OutlinerRow{Task: task}, now)
		rendered := ansi.Strip(segments.render())
		if strings.Contains(rendered, "defer") || strings.Contains(rendered, "woke") {
			t.Fatalf("count %d row indicators named a deferral: %q", count, rendered)
		}
		if peek := ansi.Strip(inlinePeekDatesLine(task, now, 80)); !strings.Contains(peek, "defer") {
			t.Fatalf("count %d lost the deferral entirely; Peek line = %q", count, peek)
		}
	}
}

// TestPeekStateValueCarriesTheWaitingAndDoingAges is the two age indicators the
// row shed. They read StateChangedAt, so commenting on a blocked task still must
// not reset how long it has been blocked, and no other state wears an age.
func TestPeekStateValueCarriesTheWaitingAndDoingAges(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		state models.WorkflowState
		days  int
		want  string
	}{
		{models.StateWaiting, 12, "waiting 12d"},
		{models.StateDoing, 9, "doing 9d"},
	} {
		task := &models.Task{
			ID:             "aged",
			Title:          "Aged",
			WorkflowState:  tc.state,
			StateChangedAt: now.AddDate(0, 0, -tc.days),
			UpdatedAt:      now,
		}
		if got := inlinePeekStateValue(task, now); got != tc.want {
			t.Fatalf("state value = %q, want %q; a touch must not reset it", got, tc.want)
		}
		if rendered := ansi.Strip(segmentsRenderedFor(task, now)); strings.Contains(rendered, tc.want) {
			t.Fatalf("the row still wore the age: %q", rendered)
		}
	}

	for _, state := range []models.WorkflowState{models.StateBacklog, models.StatePaused, models.StateDone} {
		other := &models.Task{
			ID:             "other",
			Title:          "Other",
			WorkflowState:  state,
			StateChangedAt: now.AddDate(0, 0, -12),
		}
		if got := inlinePeekStateValue(other, now); got != state.String() {
			t.Fatalf("state %q wore an age: %q", state, got)
		}
	}
}

func segmentsRenderedFor(task *models.Task, now time.Time) string {
	outliner := &OutlinerModel{columnConfig: listsurface.NewColumnConfig(nil, nil, nil)}
	return outliner.renderTaskIndicatorSegments(OutlinerRow{Task: task}, now).render()
}

// TestTaskIndicatorSegmentsRenderInOrder pins the three that survived and the
// order they read in: the attention the task is owed, the deadline it is held
// to, then the references it names.
func TestTaskIndicatorSegmentsRenderInOrder(t *testing.T) {
	segments := taskIndicatorSegments{
		review:       "review",
		due:          "due",
		dependencies: "dependencies",
	}
	plain := ansi.Strip(segments.render())
	previous := -1
	for _, part := range []string{"review", "due", "dependencies"} {
		index := strings.Index(plain, part)
		if index <= previous {
			t.Fatalf("indicator %q rendered out of order in %q", part, plain)
		}
		previous = index
	}
}

// TestIndicatorDropLadderOrder holds the shedding order to the reverse of what
// the row is for: the reference census goes first because it is a count of other
// work, then the attention the user can find again in Review, and the deadline
// last because nothing else on the row says a task is late.
func TestIndicatorDropLadderOrder(t *testing.T) {
	segments := taskIndicatorSegments{
		review:       "review",
		due:          "due",
		dependencies: "dependencies",
	}
	want := segments
	if got := len(indicatorDropLadder); got != 3 {
		t.Fatalf("indicator drop ladder has %d entries, want 3", got)
	}
	for index, drop := range indicatorDropLadder {
		switch index {
		case 0:
			want.dependencies = ""
		case 1:
			want.review = ""
		case 2:
			want.due = ""
		}
		drop(&segments)
		if segments != want {
			t.Fatalf("indicator drop %d produced %+v, want %+v", index+1, segments, want)
		}
	}
}

func TestReviewIndicatorRendersOnlyArrivedEligibleCheckIns(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		checkIn  *time.Time
		deferred *time.Time
		state    models.WorkflowState
		want     string
	}{
		{name: "no schedule", state: models.StateBacklog},
		{name: "overdue", checkIn: timePtr(now.AddDate(0, 0, -5)), state: models.StateBacklog, want: "review +5d"},
		{name: "today", checkIn: timePtr(util.StartOfDay(now)), state: models.StateBacklog, want: "review today"},
		{name: "upcoming stays quiet", checkIn: timePtr(now.AddDate(0, 0, 3)), state: models.StateBacklog},
		{name: "deferred schedule is suppressed", checkIn: timePtr(now.AddDate(0, 0, -2)), deferred: timePtr(now.AddDate(0, 0, 1)), state: models.StateBacklog},
		{name: "terminal schedule is suppressed defensively", checkIn: timePtr(now.AddDate(0, 0, -2)), state: models.StateDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				ID:            "scheduled",
				Title:         "Scheduled",
				WorkflowState: tc.state,
				NextCheckIn:   tc.checkIn,
				DeferredUntil: tc.deferred,
			}
			if got := strings.TrimSpace(ansi.Strip(formatReviewIndicator(task, now))); got != tc.want {
				t.Fatalf("review indicator = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReviewIndicatorSharesArrivedDueChipEmphasis(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})

			cases := []struct {
				name string
				day  time.Time
				want string
			}{
				{"overdue", util.StartOfDay(now.AddDate(0, 0, -5)), theme.OverdueStyle.Render("review +5d")},
				{"today", util.StartOfDay(now), theme.DueTodayStyle.Render("review today")},
			}
			for _, tc := range cases {
				day := tc.day
				task := &models.Task{
					ID:            "scheduled",
					Title:         "Scheduled",
					WorkflowState: models.StateBacklog,
					NextCheckIn:   &day,
				}
				if got := formatReviewIndicator(task, now); got != tc.want {
					t.Errorf("%s review indicator = %q, want the arrived due tier's render %q", tc.name, got, tc.want)
				}
			}
		})
	}
}

func TestReviewIndicatorCountsCalendarDaysAcrossDST(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	checkIn := time.Date(2026, time.November, 1, 0, 0, 0, 0, location)
	now := time.Date(2026, time.November, 2, 0, 1, 0, 0, location)
	task := &models.Task{
		ID:            "dst",
		Title:         "DST boundary",
		WorkflowState: models.StateBacklog,
		NextCheckIn:   &checkIn,
	}
	if got := strings.TrimSpace(ansi.Strip(formatReviewIndicator(task, now))); got != "review +1d" {
		t.Fatalf("DST review indicator = %q, want one calendar day", got)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
