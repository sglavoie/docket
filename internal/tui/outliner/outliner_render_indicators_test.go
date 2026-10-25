package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/theme"
	"github.com/sglavoie/docket/internal/util"
)

// This file covers one indicator at a time: what each signal spells, which
// states may wear it, and the two orders the set is held to — the order they
// render in and the order they are dropped in. The row keeps three indicators
// now, so the signals it shed are covered here too — by the Peek line that
// inherited them, which is the whole justification for dropping them from the
// row. What a whole row does with them under width pressure is in
// outliner_render_rows_pressure_test.go, and the reserved row segments are in
// outliner_render_rows_test.go.

// TestPeekSeparatesPendingFromWokenDeferral is the row's defer indicator,
// relocated. A deferral has two halves that never co-occur, and Peek must keep
// telling them apart now that it is the only place either is said. The two
// halves also sit on different lines: a deferral still pending is owed, so it
// joins the schedule, while a wake already happened and is history.
func TestPeekSeparatesPendingFromWokenDeferral(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		deferred time.Time
		want     string
		section  string
	}{
		{"pending", now.AddDate(0, 0, 3), "Defer: ", "schedule"},
		{"woken inside the window", now.Add(-30 * time.Hour), "Woke: ", "history"},
		{"woken past the window", now.Add(-query.DeferWakeWindow - time.Hour), "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{ID: "task", Title: "Task", DeferredUntil: &tc.deferred}
			lines := map[string]string{
				"schedule": ansi.Strip(inlinePeekScheduleLine(task, now)),
				"history":  ansi.Strip(inlinePeekHistoryLine(task, now)),
			}
			if tc.section == "" {
				for name, line := range lines {
					if strings.Contains(line, "Defer") || strings.Contains(line, "Woke") {
						t.Fatalf("lapsed deferral past the wake window still rendered %q on the %s line", line, name)
					}
				}
				return
			}
			if !strings.Contains(lines[tc.section], tc.want) {
				t.Fatalf("Peek %s line %q does not name %q", tc.section, lines[tc.section], tc.want)
			}
			for name, line := range lines {
				if name == tc.section {
					continue
				}
				if strings.Contains(line, "Defer") || strings.Contains(line, "Woke") {
					t.Fatalf("the %s line also named the deferral: %q", name, line)
				}
			}
		})
	}
}

// TestRepeatedDeferralCountStartsAtThirdDeferralInPeek keeps the threshold that
// decides when a repeated deferral is worth saying. Only the surface moved: the
// count is a Peek history field now, so the predicate and the line are checked
// together, and the schedule never carries it — an avoidance count is a record
// of what already happened rather than something the task still owes.
func TestRepeatedDeferralCountStartsAtThirdDeferralInPeek(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		count        int
		wantRepeated bool
		wantField    string
	}{
		{count: repeatedDeferralThreshold - 1, wantRepeated: false, wantField: ""},
		{count: repeatedDeferralThreshold, wantRepeated: true, wantField: "Deferred: ×3"},
		{count: 12, wantRepeated: true, wantField: "Deferred: ×12"},
	} {
		task := &models.Task{DeferCount: tc.count}
		if got := taskHasRepeatedDeferrals(task); got != tc.wantRepeated {
			t.Errorf("count %d repeated = %v, want %v", tc.count, got, tc.wantRepeated)
		}
		if schedule := ansi.Strip(inlinePeekScheduleLine(task, now)); strings.Contains(schedule, "Deferred") {
			t.Errorf("count %d put the avoidance count on the schedule line: %q", tc.count, schedule)
		}
		plain := ansi.Strip(inlinePeekHistoryLine(task, now))
		if tc.wantField == "" {
			if strings.Contains(plain, "Deferred") {
				t.Errorf("count %d named a repeated deferral: %q", tc.count, plain)
			}
			continue
		}
		if !strings.Contains(plain, tc.wantField) {
			t.Errorf("count %d Peek history line %q does not name %q", tc.count, plain, tc.wantField)
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
		rendered := strings.ToLower(ansi.Strip(segments.render()))
		if strings.Contains(rendered, "defer") || strings.Contains(rendered, "woke") {
			t.Fatalf("count %d row indicators named a deferral: %q", count, rendered)
		}
		if peek := ansi.Strip(inlinePeekScheduleLine(task, now)); !strings.Contains(peek, "Defer: ") {
			t.Fatalf("count %d lost the deferral entirely; Peek schedule line = %q", count, peek)
		}
	}
}

// TestPeekStatusValueCarriesTheWaitingAndDoingAges is the two age indicators the
// row shed. They read StateChangedAt, so commenting on a blocked task still must
// not reset how long it has been blocked, and no other state wears an age.
func TestPeekStatusValueCarriesTheWaitingAndDoingAges(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		state models.WorkflowState
		days  int
		want  string
	}{
		{models.StateWaiting, 12, "Waiting 12d"},
		{models.StateDoing, 9, "Doing 9d"},
	} {
		task := &models.Task{
			ID:             "aged",
			Title:          "Aged",
			WorkflowState:  tc.state,
			StateChangedAt: now.AddDate(0, 0, -tc.days),
			UpdatedAt:      now,
		}
		if got := inlinePeekStatusValue(task, now); got != tc.want {
			t.Fatalf("status value = %q, want %q; a touch must not reset it", got, tc.want)
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
		if got := inlinePeekStatusValue(other, now); got != chrome.PlanStatusValue(state.String()) {
			t.Fatalf("status %q wore an age: %q", state, got)
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

// A review on its day wears the due-today chip; a late one wears the amber
// warning chip rather than the teal of a missed deadline.
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
				{"late", util.StartOfDay(now.AddDate(0, 0, -5)), theme.ReviewLateStyle.Render("review +5d")},
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
					t.Errorf("%s review indicator = %q, want %q", tc.name, got, tc.want)
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

// The Review lane names a flag-only review already, so a row inside it drops the
// bare word; a check-in on or past its day still says so, and the same flag on
// a row outside the lane keeps its badge.
func TestReviewLaneRowDropsTheBareReviewOnly(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	outliner := &OutlinerModel{columnConfig: listsurface.NewColumnConfig(nil, nil, nil)}
	late := util.StartOfDay(now.AddDate(0, 0, -2))
	today := util.StartOfDay(now)

	cases := []struct {
		name string
		task *models.Task
		lane bool
		want string
	}{
		{"flag in lane", &models.Task{WorkflowState: models.StateBacklog, Review: true}, true, ""},
		{"flag outside lane", &models.Task{WorkflowState: models.StateBacklog, Review: true}, false, "review"},
		{"today in lane", &models.Task{WorkflowState: models.StateBacklog, NextCheckIn: &today}, true, "review today"},
		{"late in lane", &models.Task{WorkflowState: models.StateBacklog, Review: true, NextCheckIn: &late}, true, "review +2d"},
	}
	for _, tc := range cases {
		row := OutlinerRow{Task: tc.task}
		if tc.lane {
			row.PromotedTag = reviewLaneKey
		}
		if got := strings.TrimSpace(ansi.Strip(outliner.renderTaskIndicatorSegments(row, now).review)); got != tc.want {
			t.Errorf("%s: review segment = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestDependencyCensusKeepsAChipsLeadingGap holds the bare census to the
// distance a padded chip keeps: one column of its own ahead of the separator,
// alone or after a chip.
func TestDependencyCensusKeepsAChipsLeadingGap(t *testing.T) {
	alone := taskIndicatorSegments{dependencies: "~1/2"}.render()
	if alone != "  ~1/2" {
		t.Fatalf("lone census = %q, want two leading columns", alone)
	}
	after := taskIndicatorSegments{due: " due ", dependencies: "~1/2"}.render()
	if after != "  due   ~1/2" {
		t.Fatalf("census after a chip = %q, want the chip's gap", after)
	}
	if got := (taskIndicatorSegments{due: " due "}).render(); got != "  due " {
		t.Fatalf("a row without a census changed: %q", got)
	}
}
