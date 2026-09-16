package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/theme"
	"github.com/sglavoie/tt/internal/util"
)

// peekDocumentBody is the complete detail document for a row, independent of
// whichever page a render happens to be showing. A test asking whether Peek
// *holds* something reads this; a test asking what is on screen reads a page.
func peekDocumentBody(m *OutlinerModel, row OutlinerRow, width int) []string {
	return m.inlinePeekDocumentForRow(row, "  ", width).Body
}

// peekFirstPage is the first window of a document built straight from options,
// which is what the dock draws and what the viewport reserves room for. Paging
// past it is inline_peek_paging_test.go's subject, not this file's.
func peekFirstPage(opts inlinePeekOptions) []string {
	return inlinePeekDocumentFor(opts).Lines(0)
}

// peekWindowText is the content a window carries, without the blank rows a short
// document is padded out to the dock's fixed height with and without the
// position row every window ends on. A test comparing Peek against exact text
// means this; a test about the dock's geometry means the row count instead.
func peekWindowText(lines []string) string {
	content := make([]string, 0, len(lines))
	for _, line := range lines[:min(len(lines), inlinePeekBodyRows)] {
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			continue
		}
		content = append(content, ansi.Strip(line))
	}
	return strings.Join(content, "\n")
}

// TestInlinePeekNilAndEmptyTask pins the two degenerate inputs. A row that
// resolves no task draws nothing at all, which is what tells the scrolling keys
// there is no pane to consume them. A task carrying nothing still has a state —
// every task is in one — so the summary line is the floor of the document and
// there is no "nothing to say" sentence for a task to fall back to.
func TestInlinePeekNilAndEmptyTask(t *testing.T) {
	if lines := peekFirstPage(inlinePeekOptions{}); lines != nil {
		t.Fatalf("nil task lines = %v, want nil", lines)
	}

	lines := peekFirstPage(inlinePeekOptions{
		Task:   &models.Task{},
		Indent: "  ",
		Width:  40,
	})
	if got := peekWindowText(lines); got != "  state backlog" {
		t.Fatalf("empty Peek = %q", got)
	}
	// The window is still the dock's fixed height: a task with one line to say
	// pads rather than shrinking the block the list gave up rows for.
	if len(lines) != inlinePeekWindowRows {
		t.Fatalf("empty Peek window = %d rows, want %d", len(lines), inlinePeekWindowRows)
	}
}

// TestInlinePeekPageIsBoundedWhileTheDocumentIsNot is the split the dock rests
// on: the document holds every detail row the task has, and the five-row budget
// decides only how many of them are on screen. The position row beneath them is
// always drawn, so the window is the same height whatever the document holds.
func TestInlinePeekPageIsBoundedWhileTheDocumentIsNot(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		ShortID:     99,
		Title:       "Selected",
		Description: strings.Repeat("Unicode café 界 content ", 12),
		Comments: []*models.Comment{
			{Text: "Decision: retained as a comment"},
		},
		Checklist: []models.ChecklistItem{
			{Text: "First item", Done: true},
			{Text: "Second item"},
			{Text: "Third item"},
		},
	}

	const width = 64
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "    ", Width: width})
	if !doc.Paged() {
		t.Fatalf("a description this long fit in one page:\n%s", ansi.Strip(strings.Join(doc.Body, "\n")))
	}
	if len(doc.Body) <= inlinePeekBodyRows {
		t.Fatalf("document body = %d rows, want more than the %d-row budget", len(doc.Body), inlinePeekBodyRows)
	}

	rendered := doc.Lines(0)
	plain := ansi.Strip(strings.Join(rendered, "\n"))
	if len(rendered) != inlinePeekWindowRows {
		t.Fatalf("first page = %d rows, want %d:\n%s", len(rendered), inlinePeekWindowRows, plain)
	}
	if got := ansi.Strip(rendered[0]); got != "    Selected" {
		t.Fatalf("page does not lead with the whole title: %q\n%s", got, plain)
	}
	if !strings.Contains(ansi.Strip(rendered[len(rendered)-1]), "0 above · ") {
		t.Fatalf("paged Peek did not reserve a counter row:\n%s", plain)
	}

	// The counts are in the document even though the description pushed them off
	// the first page. The checklist is a count and never its items: those are
	// outline rows of their own under the task.
	whole := ansi.Strip(strings.Join(doc.Body, "\n"))
	for _, want := range []string{"Comments: 1", "Checklist: 1/3 done"} {
		if !strings.Contains(whole, want) {
			t.Fatalf("document dropped %q:\n%s", want, whole)
		}
	}
	for _, banned := range []string{"#41", "#42", "#99", "First item", "Second item"} {
		if strings.Contains(whole, banned) {
			t.Fatalf("Peek repeated %q:\n%s", banned, whole)
		}
	}
	for lineNumber, line := range rendered {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", lineNumber, got, width, ansi.Strip(line))
		}
	}
}

// TestInlinePeekScheduleLineNamesOnlyWhatIsStillOwed is the forward half of the
// split that replaced the old mixed dates line. Every field here is a date in
// front of the task — the deadline, the day a pending deferral brings it back,
// and the check-in with its cadence — and an instant the task has already spent
// may never appear on it.
func TestInlinePeekScheduleLineNamesOnlyWhatIsStillOwed(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)
	checkIn := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	lastReview := now.Add(-3 * time.Hour)
	woke := now.Add(-24 * time.Hour)
	pending := now.Add(24 * time.Hour)

	cases := []struct {
		name string
		task *models.Task
		want string
	}{
		{"nothing owed at all", &models.Task{}, ""},
		{
			"every owed field, in the order they are acted on",
			&models.Task{
				DueDate:           &due,
				DeferredUntil:     &pending,
				NextCheckIn:       &checkIn,
				CheckInRepeatDays: 7,
			},
			"due 2026-08-14 · defer " + chrome.FormatDeferDate(pending, now) + " · check-in 2026-08-12 · repeat 7d",
		},
		{
			// The row carries no deferral now, so Peek names the pending one — and
			// still names it as the deferral it is rather than as a wake-up.
			"a pending deferral names the day it comes back, not a wake-up",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &pending},
			"defer " + chrome.FormatDeferDate(pending, now),
		},
		{
			"a lapsed deferral is history and owes this line nothing",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &woke, DeferCount: 3},
			"",
		},
		{
			"one-off check-in names repetition as off",
			&models.Task{NextCheckIn: &checkIn},
			"check-in 2026-08-12 · repeat off",
		},
		{
			"spent instants never reach the schedule",
			&models.Task{
				CreatedAt:      now.Add(-12 * 24 * time.Hour),
				UpdatedAt:      now.Add(-2 * 24 * time.Hour),
				LastReviewedAt: &lastReview,
			},
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ansi.Strip(inlinePeekScheduleLine(tc.task, now)); got != tc.want {
				t.Fatalf("schedule line = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInlinePeekHistoryLineNamesOnlyWhatIsAlreadySpent is the other half. Age,
// last touch, last review, a recent wake, and the avoidance count are all
// records of something that happened, so they close the document rather than
// standing between the schedule and the task's own text — and the deadline and
// the check-in may never appear here.
func TestInlinePeekHistoryLineNamesOnlyWhatIsAlreadySpent(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	due := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)
	checkIn := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	lastReview := now.Add(-3 * time.Hour)
	woke := now.Add(-24 * time.Hour)
	pending := now.Add(24 * time.Hour)
	stale := now.Add(-query.DeferWakeWindow - time.Hour)

	cases := []struct {
		name string
		task *models.Task
		want string
	}{
		{"no history at all", &models.Task{}, ""},
		{
			"every spent field",
			&models.Task{
				CreatedAt:      now.Add(-12 * 24 * time.Hour),
				UpdatedAt:      now.Add(-2 * 24 * time.Hour),
				DeferredUntil:  &woke,
				DeferCount:     3,
				LastReviewedAt: &lastReview,
			},
			"created 12d · updated 2d · last review 3h · woke 1d · deferred ×3",
		},
		{
			"created only",
			&models.Task{CreatedAt: now.Add(-3 * time.Hour)},
			"created 3h",
		},
		{
			"a pending deferral has not woken, so history says nothing about it",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &pending},
			"created 1d",
		},
		{
			"a lapsed deferral past the wake window falls silent",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &stale},
			"created 1d",
		},
		{
			"the avoidance count stands without a wake marker",
			&models.Task{DeferCount: 4},
			"deferred ×4",
		},
		{
			"owed dates never reach history",
			&models.Task{DueDate: &due, NextCheckIn: &checkIn, CheckInRepeatDays: 7},
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ansi.Strip(inlinePeekHistoryLine(tc.task, now)); got != tc.want {
				t.Fatalf("history line = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInlinePeekCheckInFieldsWrapWithinNarrowWidth keeps the schedule and the
// review it answers reachable at a width that holds neither whole. They are two
// sections now, so the claim is read off the document rather than off one line:
// both wrap rather than eliding, and no row outgrows the pane.
func TestInlinePeekCheckInFieldsWrapWithinNarrowWidth(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	checkIn := util.StartOfDay(now.AddDate(0, 0, 2))
	lastReview := now.Add(-37 * time.Hour)
	task := &models.Task{
		NextCheckIn:       &checkIn,
		CheckInRepeatDays: 30,
		LastReviewedAt:    &lastReview,
	}
	const width = 22
	lines := append(
		inlinePeekScheduleLines(task, now, "  ", width),
		inlinePeekHistoryLines(task, now, "  ", width)...,
	)
	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"check-in", "2026-08-12", "repeat", "30d", "last", "review", "1d"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("narrow Peek dates missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "…") {
		t.Fatalf("narrow Peek elided a date instead of wrapping it:\n%s", plain)
	}
	for index, line := range lines {
		if got, limit := lipgloss.Width(line), width+2; got > limit {
			t.Fatalf("narrow Peek line %d is %d columns, want <= %d: %q", index, got, limit, ansi.Strip(line))
		}
	}
}

// TestInlinePeekDateKeysReadStoredInstantsAlone is the paging half of the split.
// Each section identifies its own content, so every stored value that reaches
// either line moves that line's key — and the deferral instant moves both, since
// the schedule reads its pending half and history reads its wake.
func TestInlinePeekDateKeysReadStoredInstantsAlone(t *testing.T) {
	assertChanged := func(label string, key func(*models.Task) string, task *models.Task, mutate func()) {
		t.Helper()
		before := key(task)
		mutate()
		if after := key(task); after == before {
			t.Fatalf("%s did not change the Peek section identity it belongs to", label)
		}
	}

	checkIn := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	due := time.Date(2026, time.August, 14, 0, 0, 0, 0, time.UTC)
	deferred := time.Date(2026, time.August, 9, 0, 0, 0, 0, time.UTC)
	lastReview := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)

	schedule := &models.Task{}
	assertChanged("deadline", inlinePeekScheduleKey, schedule, func() { schedule.DueDate = &due })
	assertChanged("deferral", inlinePeekScheduleKey, schedule, func() { schedule.DeferredUntil = &deferred })
	assertChanged("next check-in", inlinePeekScheduleKey, schedule, func() { schedule.NextCheckIn = &checkIn })
	assertChanged("repeat", inlinePeekScheduleKey, schedule, func() { schedule.CheckInRepeatDays = 7 })

	history := &models.Task{}
	assertChanged("creation", inlinePeekHistoryKey, history, func() { history.CreatedAt = due })
	assertChanged("touch", inlinePeekHistoryKey, history, func() { history.UpdatedAt = due })
	assertChanged("wake", inlinePeekHistoryKey, history, func() { history.DeferredUntil = &deferred })
	assertChanged("avoidance count", inlinePeekHistoryKey, history, func() { history.DeferCount = 3 })
	assertChanged("last review", inlinePeekHistoryKey, history, func() { history.LastReviewedAt = &lastReview })

	// The drifting half, stated against one task that carries every field of both
	// lines: ninety minutes of wall clock rewrite what history says, and neither
	// key moves with it. That is what keeps the minute tick from throwing a
	// reader back to the top of the document.
	early := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	late := early.Add(90 * time.Minute)
	steady := &models.Task{
		CreatedAt: early.Add(-90 * time.Minute), UpdatedAt: early.Add(-90 * time.Minute),
		DueDate: &due, DeferredUntil: &deferred, DeferCount: 4,
		NextCheckIn: &checkIn, CheckInRepeatDays: 7, LastReviewedAt: &lastReview,
	}
	scheduleKey, historyKey := inlinePeekScheduleKey(steady), inlinePeekHistoryKey(steady)
	if inlinePeekHistoryLine(steady, early) == inlinePeekHistoryLine(steady, late) {
		t.Fatal("the fixture's history reads the same at both instants, so it proves no drift")
	}
	for label, drifted := range map[string]string{
		"early history": ansi.Strip(inlinePeekHistoryLine(steady, early)),
		"late history":  ansi.Strip(inlinePeekHistoryLine(steady, late)),
		"schedule":      ansi.Strip(inlinePeekScheduleLine(steady, late)),
	} {
		if strings.Contains(scheduleKey, drifted) || strings.Contains(historyKey, drifted) {
			t.Fatalf("a Peek section key carries the %s rendering %q", label, drifted)
		}
	}
	if got := inlinePeekScheduleKey(steady); got != scheduleKey {
		t.Fatalf("schedule key is not stable across two reads: %q then %q", scheduleKey, got)
	}
	if got := inlinePeekHistoryKey(steady); got != historyKey {
		t.Fatalf("history key is not stable across two reads: %q then %q", historyKey, got)
	}
}

// TestInlinePeekBodyKeepsItsContentOrder locks what replaced the old drop
// ladder. Nothing is shed to make room any more, so the body's only decision is
// order: identity first (title, parent), then what the task is and what it still
// owes, then the task's own text, then the supporting counts and the references,
// then the narrowing axes, and the instants it has already spent last. The
// checklist is a count and never its items.
func TestInlinePeekBodyKeepsItsContentOrder(t *testing.T) {
	stateChanged := time.Now().Add(-4 * time.Hour)
	due := time.Now().AddDate(0, 0, 4)
	checkIn := util.StartOfDay(time.Now().AddDate(0, 0, 2))
	lastReview := time.Now().Add(-25 * time.Hour)
	task := &models.Task{
		ID:             "task",
		Title:          "Rework the sync path",
		WorkflowState:  models.StateDoing,
		StateChangedAt: stateChanged,
		Priority:       2,
		Pinned:         true,
		Review:         true,
		ResumeNote:     "Left mid-refactor",
		Description:    strings.Repeat("Wrapped description content ", 12),
		Context:        "platform-core",
		CreatedAt:      time.Now().Add(-5 * 24 * time.Hour),
		UpdatedAt:      time.Now().Add(-time.Hour),
		DueDate:        &due,
		NextCheckIn:    &checkIn,
		LastReviewedAt: &lastReview,
		Attachments:    []models.Attachment{{Filename: "spec.pdf"}},
		Comments:       []*models.Comment{{Text: "Decision"}},
		Tags:           []string{"backend"},
		Checklist:      []models.ChecklistItem{{Text: "First item"}, {Text: "Second item", Done: true}},
	}

	doc := inlinePeekDocumentFor(inlinePeekOptions{
		Task: task, Breadcrumb: "Launch platform ›", Planning: "today", Width: 80,
		Dependencies:     []DependencyDetail{{Label: "Platform #7 Blocking work", State: "doing"}},
		Dependents:       []DependencyDetail{{Label: "Platform #9 Waiting work", State: "waiting"}},
		DependencyCensus: dependencyCensus{total: 1, blocked: 1},
	})
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	order := []string{
		"Rework the sync path",
		"Parent: Launch platform",
		"state doing 4h · priority 2 · plan today · pinned · review",
		"due " + chrome.DueGrammarValue(task) + " · check-in " + checkIn.Format(time.DateOnly) + " · repeat off",
		"Resume: Left mid-refactor",
		"Description: Wrapped description content",
		"Checklist: 1/2 done",
		"Attachments 1: spec.pdf",
		"Comments: 1",
		"Dependencies ",
		"Dependents 1: ",
		"Context: [platform-core]",
		"Tags: #backend",
		"created 5d · updated 1h · last review 1d",
	}
	cursor := 0
	for _, want := range order {
		found := strings.Index(body[cursor:], want)
		if found < 0 {
			t.Fatalf("body is missing %q or has it out of order at %d:\n%s", want, cursor, body)
		}
		cursor += found + len(want)
	}
	for _, banned := range []string{"First item", "Second item"} {
		if strings.Contains(body, banned) {
			t.Fatalf("Peek still renders %q:\n%s", banned, body)
		}
	}
	if len(doc.Lines(0)) != inlinePeekWindowRows {
		t.Fatalf("page = %d rows, want %d", len(doc.Lines(0)), inlinePeekWindowRows)
	}
	// Every section is reachable by paging, which is the claim order has to rest
	// on: history moved to the end of a document longer than the window, so it is
	// only still readable if the last page holds it.
	last := ansi.Strip(strings.Join(doc.Lines(doc.MaxOffset()), "\n"))
	if !strings.Contains(last, "created 5d · updated 1h · last review 1d") {
		t.Fatalf("history is not on the document's last page:\n%s", last)
	}
}

// TestInlinePeekStatusLineNamesWhatTheRowAbbreviates is the summary line's own
// claim: the row spells state and priority as glyphs and says nothing at all
// about planning membership, and Peek spells all four in words a reader can
// search for. Each field is omitted when the task does not carry it, and the
// state is the one that never is.
func TestInlinePeekStatusLineNamesWhatTheRowAbbreviates(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		task     *models.Task
		planning string
		want     string
	}{
		{"bare task", &models.Task{}, "", "state backlog"},
		{
			"doing carries its age",
			&models.Task{WorkflowState: models.StateDoing, StateChangedAt: now.Add(-50 * time.Hour)},
			"",
			"state doing 2d",
		},
		{
			"waiting carries its age",
			&models.Task{WorkflowState: models.StateWaiting, StateChangedAt: now.Add(-90 * time.Minute)},
			"",
			"state waiting 1h",
		},
		{
			"a quiet state has no age to give",
			&models.Task{WorkflowState: models.StateSomeday, StateChangedAt: now.Add(-90 * time.Hour)},
			"",
			"state someday",
		},
		{
			"an unset priority names none",
			&models.Task{WorkflowState: models.StateInbox},
			"",
			"state inbox",
		},
		{
			"every axis at once",
			&models.Task{WorkflowState: models.StatePaused, Priority: 1, Pinned: true, Review: true},
			"next",
			"state paused · priority 1 · plan next · pinned · review",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ansi.Strip(inlinePeekStatusLine(tc.task, tc.planning, now)); got != tc.want {
				t.Fatalf("status line = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInlinePeekStatusKeyReadsStoredValuesAlone keeps the summary out of the
// fingerprint's drifting half: the state clock is keyed as an instant, so the
// rendered age moving from 3h to 4h is the same document and Peek's window stays
// where the reader left it.
func TestInlinePeekStatusKeyReadsStoredValuesAlone(t *testing.T) {
	task := &models.Task{WorkflowState: models.StateDoing, StateChangedAt: time.Now().Add(-3 * time.Hour)}
	before := inlinePeekStatusKey(task, "")
	if after := inlinePeekStatusKey(task, ""); after != before {
		t.Fatalf("status identity is not stable across two reads")
	}

	assertChanged := func(label string, mutate func()) {
		t.Helper()
		previous := inlinePeekStatusKey(task, "")
		mutate()
		if after := inlinePeekStatusKey(task, ""); after == previous {
			t.Fatalf("%s did not change Peek status identity", label)
		}
	}
	assertChanged("state", func() { task.WorkflowState = models.StateWaiting })
	assertChanged("state clock", func() { task.StateChangedAt = task.StateChangedAt.Add(-time.Hour) })
	assertChanged("priority", func() { task.Priority = 3 })
	assertChanged("pinned", func() { task.Pinned = true })
	assertChanged("review", func() { task.Review = true })
	if inlinePeekStatusKey(task, "today") == inlinePeekStatusKey(task, "next") {
		t.Fatal("planning membership does not change Peek status identity")
	}
}

// TestInlinePeekNamesTheWholeTitle locks the title block: the complete title,
// bare (no label), never elided, and never abbreviated to the part the row
// happened to clip. The row's own ellipsis now means "read this in Peek" rather
// than "the rest is pinned above the pane".
func TestInlinePeekNamesTheWholeTitle(t *testing.T) {
	const head = "Investigate intermittent timeout when syncing "
	const tail = "very large project archives over slow connections"
	task := &models.Task{ID: "task", Title: head + tail}

	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: 80})
	whole := ansi.Strip(strings.Join(doc.Body, "\n"))
	// The wrap drops the space it breaks on, so the title is read back word by
	// word; the claim is that no word went missing, not where the breaks fell.
	if flat := strings.Join(strings.Fields(whole), " "); !strings.HasPrefix(flat, head+tail) {
		t.Fatalf("Peek does not hold the whole title:\n%s", whole)
	}
	for _, banned := range []string{"Title:", "…"} {
		if strings.Contains(whole, banned) {
			t.Fatalf("title block rendered %q:\n%s", banned, whole)
		}
	}
	for lineNumber, line := range doc.Lines(0) {
		if got := lipgloss.Width(line); got > 80 {
			t.Fatalf("line %d width = %d, want <= 80", lineNumber, got)
		}
	}

	// A short title is named too. Peek is the task's whole document, so what it
	// shows does not depend on what the row had room for.
	short := inlinePeekDocumentFor(inlinePeekOptions{
		Task: &models.Task{ID: "task", Title: "Short"}, Indent: "  ", Width: 60,
	})
	if got := ansi.Strip(short.Body[0]); got != "  Short" {
		t.Fatalf("short title row = %q", got)
	}
}

// TestInlinePeekTitleIsBodyRatherThanHeader is the scrolling half of the same
// claim. Nothing in Peek sits outside the window any more: a title long enough
// to fill the dock pages away like a long description, and the window stays the
// same height however long the title is.
func TestInlinePeekTitleIsBodyRatherThanHeader(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		Title:       strings.Repeat("continuation ", 40),
		Description: strings.Repeat("Wrapped description content ", 12),
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-time.Hour),
		Attachments: []models.Attachment{{Filename: "spec.pdf"}},
		Comments:    []*models.Comment{{Text: "Decision"}},
	}

	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Width: 40})
	if len(doc.Lines(0)) != inlinePeekWindowRows {
		t.Fatalf("first page = %d rows, want the %d-row window", len(doc.Lines(0)), inlinePeekWindowRows)
	}
	if !strings.HasPrefix(ansi.Strip(doc.Lines(0)[0]), "continuation") {
		t.Fatalf("page does not lead with the title:\n%s", ansi.Strip(strings.Join(doc.Lines(0), "\n")))
	}
	// The title is not pinned: the last window of a document this long has paged
	// past it entirely and shows the detail beneath instead.
	last := ansi.Strip(strings.Join(doc.Lines(doc.MaxOffset()), "\n"))
	if strings.HasPrefix(last, "continuation") {
		t.Fatalf("the title stayed pinned above the body:\n%s", last)
	}
	if strings.Contains(ansi.Strip(strings.Join(doc.Body, "\n")), "…") {
		t.Fatalf("a long title was elided instead of wrapped")
	}
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	for _, want := range []string{"created 5d · updated 1h", "Attachments 1: spec.pdf"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the title displaced %q:\n%s", want, body)
		}
	}
}

// TestInlinePeekNamesThePromotedAncestorChain locks the surface the row's bare
// `‹` points at: the full chain, root-first, only when the row is a promoted
// section root.
func TestInlinePeekNamesThePromotedAncestorChain(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Child"}

	withParent := peekWindowText(peekFirstPage(inlinePeekOptions{
		Task: task, Breadcrumb: "Launch platform › User authentication ›", Width: 80,
	}))
	if !strings.Contains(withParent, "Parent: Launch platform › User authentication") {
		t.Fatalf("parent line = %q", withParent)
	}

	withoutParent := peekWindowText(peekFirstPage(inlinePeekOptions{Task: task, Width: 80}))
	if strings.Contains(withoutParent, "Parent:") {
		t.Fatalf("root task Peek gained a parent line: %q", withoutParent)
	}
	// The chain follows the title and precedes everything else: both are identity,
	// and a reader looking for where a promoted row belongs reads them together.
	if withParent != "Child\nParent: Launch platform › User authentication\nstate backlog" {
		t.Fatalf("identity block = %q", withParent)
	}
}

// TestInlinePeekWrapsALongAncestorChain keeps the chain whole at a width that
// cannot hold it. The row compacts a deep chain to a single glyph, so Peek is
// where every ancestor is named, and nothing here elides.
func TestInlinePeekWrapsALongAncestorChain(t *testing.T) {
	const chain = "Launch platform › User authentication › Token rotation › Refresh flow ›"

	const width = 34
	lines := inlinePeekParentLines(chain, "  ", width-2)
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if len(lines) < 2 {
		t.Fatalf("a chain this long did not wrap: %q", plain)
	}
	// The wrap drops the space it breaks on, so the chain is read back word by
	// word rather than by contiguous substring.
	if flat := strings.Join(strings.Fields(plain), " "); flat != "Parent: "+strings.TrimSuffix(chain, " ›") {
		t.Fatalf("wrapped chain is not the whole chain: %q", flat)
	}
	if strings.Contains(plain, "…") {
		t.Fatalf("wrapped chain was elided:\n%s", plain)
	}
	for index, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("chain row %d = %d columns, want <= %d: %q", index, got, width, ansi.Strip(line))
		}
	}
	if inlinePeekParentLines("", "  ", width) != nil {
		t.Fatal("a root task grew a Parent line")
	}
}

func TestInlinePeekWrapsTagsRatherThanElidingThem(t *testing.T) {
	task := &models.Task{
		ID:   "task",
		Tags: []string{"backend", "security", "type:authentication"},
	}

	styled := inlinePeekTagsLines(task, "", 80)
	wantStyled := theme.MutedStyle.Render("Tags: ") +
		theme.TagChipStyle("backend").Render("#backend") + " " +
		theme.TagChipStyle("security").Render("#security") + " " +
		theme.TagChipStyle("type:authentication").Render("#type:authentication")
	if len(styled) != 1 || styled[0] != wantStyled {
		t.Fatalf("styled Peek tags = %q, want %q", styled, wantStyled)
	}

	// Narrow enough that the chips cannot share a row. Every tag is still named:
	// the chips hang under the label instead of being cut off behind an ellipsis.
	const width = 32
	lines := peekFirstPage(inlinePeekOptions{Task: task, Indent: "  ", Width: width})
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if len(lines) < 2 {
		t.Fatalf("narrow tags did not wrap: %q", plain)
	}
	if !strings.Contains(plain, "  Tags: #backend") {
		t.Fatalf("Peek tags = %q", plain)
	}
	for _, want := range []string{"#backend", "#security", "#type:authentication"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("wrapped tags dropped %q: %q", want, plain)
		}
	}
	if strings.Contains(plain, "…") {
		t.Fatalf("wrapped tags were also elided: %q", plain)
	}
	for lineNumber, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("tag line %d width = %d, want <= %d", lineNumber, got, width)
		}
	}
}

// TestInlinePeekNamesTheContextInItsOwnSection replaced the old fold into the
// tags line. The row reserves a context segment at every width and truncates it
// there, so Peek shows the whole value on a line of its own whether or not the
// row could fit it.
func TestInlinePeekNamesTheContextInItsOwnSection(t *testing.T) {
	short := &models.Task{Context: "platform"}
	if got := ansi.Strip(strings.Join(inlinePeekContextLines(short, "", 80), "\n")); got != "Context: [platform]" {
		t.Fatalf("short context line = %q", got)
	}

	long := &models.Task{Context: "platform-core", Tags: []string{"bug"}}
	body := ansi.Strip(strings.Join(inlinePeekDocumentFor(inlinePeekOptions{Task: long, Width: 80}).Body, "\n"))
	if body != "state backlog\nContext: [platform-core]\nTags: #bug" {
		t.Fatalf("context and tags body = %q", body)
	}

	if inlinePeekContextLines(&models.Task{}, "", 80) != nil {
		t.Fatal("a task with no context grew a Context line")
	}
}

func TestInlinePeekNamesEveryAttachment(t *testing.T) {
	task := &models.Task{
		Attachments: []models.Attachment{
			{Filename: "design-spec.pdf"},
			{Filename: "https://github.com/acme/roadmap/issues/123", IsURL: true},
			{Filename: "notes.md"},
		},
	}

	lines := inlinePeekAttachmentsLines(task, "", 80)
	if len(lines) != 3 {
		t.Fatalf("attachment rows = %d, want one per attachment:\n%s", len(lines), ansi.Strip(strings.Join(lines, "\n")))
	}
	want := []string{
		"Attachments 3: design-spec.pdf",
		"               GH acme/roadmap#123",
		"               notes.md",
	}
	for i, expected := range want {
		if got := ansi.Strip(lines[i]); got != expected {
			t.Fatalf("attachment row %d = %q, want %q", i, got, expected)
		}
	}
	if strings.Contains(ansi.Strip(strings.Join(lines, "\n")), "more") {
		t.Fatalf("attachments still counted an overflow instead of naming it")
	}
}

// TestInlinePeekWrapsALongAttachmentNameInsteadOfEliding is the wide-character
// half of the same claim: a name longer than the pane hangs under its label over
// as many rows as it needs, and no row outgrows the pane.
func TestInlinePeekWrapsALongAttachmentNameInsteadOfEliding(t *testing.T) {
	task := &models.Task{
		ID: "task",
		Attachments: []models.Attachment{
			{Filename: strings.Repeat("界", 40) + ".pdf"},
			{Filename: "second.pdf"},
			{Filename: "third.pdf"},
		},
	}
	const width = 40
	lines := inlinePeekAttachmentsLines(task, "  ", width-2)
	if len(lines) < 5 {
		t.Fatalf("long attachment name did not wrap: %d rows", len(lines))
	}
	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"second.pdf", "third.pdf"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("wrapped attachments dropped %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "…") {
		t.Fatalf("wrapped attachments were also elided:\n%s", plain)
	}
	if got := strings.Count(plain, "界"); got != 40 {
		t.Fatalf("wrapped attachment kept %d of its 40 wide characters:\n%s", got, plain)
	}
	for lineNumber, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("attachment row %d width = %d, want <= %d: %q", lineNumber, got, width, ansi.Strip(line))
		}
	}

	// The narrowest pane the product supports has room for less than one name.
	// The claim there is only that nothing is elided and nothing overflows: a
	// name too long for a row is broken across rows, never cut short.
	for _, narrow := range []int{20, 24} {
		narrowLines := inlinePeekAttachmentsLines(task, "  ", narrow-2)
		narrowPlain := ansi.Strip(strings.Join(narrowLines, "\n"))
		if strings.Contains(narrowPlain, "…") {
			t.Fatalf("width %d elided an attachment:\n%s", narrow, narrowPlain)
		}
		if got := strings.Count(narrowPlain, "界"); got != 40 {
			t.Fatalf("width %d kept %d of the 40 wide characters:\n%s", narrow, got, narrowPlain)
		}
		for lineNumber, line := range narrowLines {
			if got := lipgloss.Width(line); got > narrow {
				t.Fatalf("width %d row %d = %d columns: %q", narrow, lineNumber, got, ansi.Strip(line))
			}
		}
	}
}
