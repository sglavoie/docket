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
	return m.inlinePeekDocumentForRow(row, "  ", width, chrome.WorstCaseColumnLayout(width)).Body
}

// peekFirstPage is the first page of a document built straight from options,
// which is what the renderer and the viewport measurement both consume. Paging
// past it is inline_peek_paging_test.go's subject, not this file's.
func peekFirstPage(opts inlinePeekOptions) []string {
	return inlinePeekDocumentFor(opts).Lines(0)
}

func TestInlinePeekNilAndEmptyTask(t *testing.T) {
	if lines := peekFirstPage(inlinePeekOptions{}); lines != nil {
		t.Fatalf("nil task lines = %v, want nil", lines)
	}

	lines := peekFirstPage(inlinePeekOptions{
		Task:   &models.Task{},
		Indent: "  ",
		Width:  40,
	})
	if got := ansi.Strip(strings.Join(lines, "\n")); got != "  No additional details" {
		t.Fatalf("empty Peek = %q", got)
	}
}

// TestInlinePeekPageIsBoundedWhileTheDocumentIsNot is the split session 4 bought:
// the document holds every detail row the task has, and the five-row budget
// decides only how many of them are on screen. One of those five goes to the
// counter as soon as anything is off screen.
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
	if len(rendered) != inlinePeekBodyRows {
		t.Fatalf("first page = %d rows, want %d:\n%s", len(rendered), inlinePeekBodyRows, plain)
	}
	if !strings.HasPrefix(ansi.Strip(rendered[0]), "    Description: ") {
		t.Fatalf("page does not lead with the description:\n%s", plain)
	}
	if !strings.Contains(ansi.Strip(rendered[len(rendered)-1]), "0 above · ") {
		t.Fatalf("paged Peek did not reserve a counter row:\n%s", plain)
	}

	// The comment count is in the document even though the description pushed it
	// off the first page, and the checklist is nowhere: those stay outline rows.
	whole := ansi.Strip(strings.Join(doc.Body, "\n"))
	if !strings.Contains(whole, "Comments: 1") {
		t.Fatalf("document dropped the comment count:\n%s", whole)
	}
	for _, banned := range []string{"#41", "#42", "#99", "State:", "Priority:", "Due:", "Checklist:", "First item"} {
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

func TestInlinePeekDatesLineOmitsEmptyFields(t *testing.T) {
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
		{"no dates at all", &models.Task{}, ""},
		{
			"every field",
			&models.Task{
				CreatedAt:         now.Add(-12 * 24 * time.Hour),
				UpdatedAt:         now.Add(-2 * 24 * time.Hour),
				DueDate:           &due,
				DeferredUntil:     &woke,
				DeferCount:        3,
				NextCheckIn:       &checkIn,
				CheckInRepeatDays: 7,
				LastReviewedAt:    &lastReview,
			},
			"created 12d · updated 2d · due 2026-08-14 · check-in 2026-08-12 · repeat 7d · last review 3h · woke 1d · deferred ×3",
		},
		{
			"created only",
			&models.Task{CreatedAt: now.Add(-3 * time.Hour)},
			"created 3h",
		},
		{
			"a pending deferral is not a wake-up",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &pending},
			"created 1d",
		},
		{
			"a lapsed deferral past the wake window falls silent",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &stale},
			"created 1d",
		},
		{
			"repeat count keeps the existing dates line without a wake marker",
			&models.Task{DeferCount: 4},
			"deferred ×4",
		},
		{
			"one-off check-in names repetition as off",
			&models.Task{NextCheckIn: &checkIn},
			"check-in 2026-08-12 · repeat off",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ansi.Strip(inlinePeekDatesLine(tc.task, now, 80)); got != tc.want {
				t.Fatalf("dates line = %q, want %q", got, tc.want)
			}
		})
	}
}

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
	lines := inlinePeekDatesLines(task, now, "  ", width)
	plain := ansi.Strip(strings.Join(lines, "\n"))
	for _, want := range []string{"check-in", "2026-08-12", "repeat", "30d", "last", "review", "1d"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("narrow Peek dates missing %q:\n%s", want, plain)
		}
	}
	for index, line := range lines {
		if got, limit := lipgloss.Width(line), width+2; got > limit {
			t.Fatalf("narrow Peek line %d is %d columns, want <= %d: %q", index, got, limit, ansi.Strip(line))
		}
	}
}

func TestInlinePeekDatesKeyIncludesCheckInScheduleAndReviewHistory(t *testing.T) {
	task := &models.Task{}
	assertChanged := func(label string, mutate func()) {
		t.Helper()
		before := inlinePeekDatesKey(task)
		mutate()
		if after := inlinePeekDatesKey(task); after == before {
			t.Fatalf("%s did not change Peek dates identity", label)
		}
	}

	checkIn := time.Date(2026, time.August, 12, 0, 0, 0, 0, time.UTC)
	assertChanged("next check-in", func() { task.NextCheckIn = &checkIn })
	assertChanged("repeat", func() { task.CheckInRepeatDays = 7 })
	lastReview := time.Date(2026, time.August, 10, 9, 0, 0, 0, time.UTC)
	assertChanged("last review", func() { task.LastReviewedAt = &lastReview })
}

// TestInlinePeekBodyKeepsItsContentOrder locks what replaced the old drop
// ladder. Nothing is shed to make room any more, so the body's only decision is
// order: resume, description, dates, attachments, comments, references,
// context, tags — and the checklist stays out of it entirely.
func TestInlinePeekBodyKeepsItsContentOrder(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		ResumeNote:  "Left mid-refactor",
		Description: strings.Repeat("Wrapped description content ", 12),
		Context:     "platform-core",
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-time.Hour),
		Attachments: []models.Attachment{{Filename: "spec.pdf"}},
		Comments:    []*models.Comment{{Text: "Decision"}},
		Tags:        []string{"backend"},
		Checklist:   []models.ChecklistItem{{Text: "First item"}},
	}

	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Width: 80})
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	order := []string{
		"Resume: Left mid-refactor",
		"Description: Wrapped description content",
		"created 5d · updated 1h",
		"Attachments: spec.pdf",
		"Comments: 1",
		"Context: [platform-core]",
		"Tags: #backend",
	}
	cursor := 0
	for _, want := range order {
		found := strings.Index(body[cursor:], want)
		if found < 0 {
			t.Fatalf("body is missing %q or has it out of order at %d:\n%s", want, cursor, body)
		}
		cursor += found + len(want)
	}
	for _, banned := range []string{"Checklist:", "First item"} {
		if strings.Contains(body, banned) {
			t.Fatalf("Peek still renders %q:\n%s", banned, body)
		}
	}
	if len(doc.Lines(0)) != inlinePeekBodyRows {
		t.Fatalf("page = %d rows, want %d", len(doc.Lines(0)), inlinePeekBodyRows)
	}
}

// TestInlinePeekCompletesAClippedTitle locks the continuation block: it is bare
// (no label), it carries no ellipsis while it fits inside inlinePeekTitleRows,
// and it never repeats the head the row already showed.
func TestInlinePeekCompletesAClippedTitle(t *testing.T) {
	const head = "Investigate intermittent timeout when syncing "
	const tail = "very large project archives over slow connections"
	task := &models.Task{ID: "task", Title: head + tail}

	rendered := peekFirstPage(inlinePeekOptions{
		Task: task, TitleOverflow: tail, Indent: "  ", Width: 80,
	})
	plain := ansi.Strip(strings.Join(rendered, "\n"))
	if !strings.Contains(plain, tail) {
		t.Fatalf("Peek missing the title continuation:\n%s", plain)
	}
	if strings.Contains(plain, head) {
		t.Fatalf("Peek restated the head the row already showed:\n%s", plain)
	}
	for _, banned := range []string{"Title:", "…", "No additional details"} {
		if strings.Contains(plain, banned) {
			t.Fatalf("continuation rendered %q:\n%s", banned, plain)
		}
	}
	for lineNumber, line := range rendered {
		if got := lipgloss.Width(line); got > 80 {
			t.Fatalf("line %d width = %d, want <= 80", lineNumber, got)
		}
	}
}

// TestInlinePeekTitleContinuationIsHeaderRatherThanBody proves the continuation
// displaces nothing and pages with nothing: it sits above the body budget, so a
// paged document still spends all five of its rows on detail beneath it.
func TestInlinePeekTitleContinuationIsHeaderRatherThanBody(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		Title:       "Long title",
		Description: strings.Repeat("Wrapped description content ", 12),
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-time.Hour),
		Attachments: []models.Attachment{{Filename: "spec.pdf"}},
		Comments:    []*models.Comment{{Text: "Decision"}},
	}

	const overflow = "the remainder the row had no width for"
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, TitleOverflow: overflow, Width: 80})
	if len(doc.Header) != 1 {
		t.Fatalf("header = %d rows, want the continuation alone", len(doc.Header))
	}
	rendered := doc.Lines(0)
	if len(rendered) != len(doc.Header)+inlinePeekBodyRows {
		t.Fatalf("page = %d rows, want %d header rows plus the %d-row budget",
			len(rendered), len(doc.Header), inlinePeekBodyRows)
	}
	if !strings.HasPrefix(ansi.Strip(strings.Join(rendered, "\n")), overflow) {
		t.Fatalf("continuation is not the first Peek row:\n%s", ansi.Strip(strings.Join(rendered, "\n")))
	}

	// The continuation is still the first row at the end of the document: header
	// does not scroll out from under the body it heads.
	last := doc.Lines(doc.MaxOffset())
	if !strings.HasPrefix(ansi.Strip(last[0]), overflow) {
		t.Fatalf("the continuation paged away:\n%s", ansi.Strip(strings.Join(last, "\n")))
	}
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	for _, want := range []string{"created 5d · updated 1h", "Attachments: spec.pdf"} {
		if !strings.Contains(body, want) {
			t.Fatalf("continuation displaced %q:\n%s", want, body)
		}
	}
}

// TestInlinePeekNamesThePromotedAncestorChain locks the surface the row's bare
// `‹` points at: the full chain, root-first, only when the row is a promoted
// section root.
func TestInlinePeekNamesThePromotedAncestorChain(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Child"}

	withParent := ansi.Strip(strings.Join(peekFirstPage(inlinePeekOptions{
		Task: task, Breadcrumb: "Launch platform › User authentication ›", Width: 80,
	}), "\n"))
	if withParent != "Parent: Launch platform › User authentication" {
		t.Fatalf("parent line = %q", withParent)
	}

	withoutParent := peekFirstPage(inlinePeekOptions{Task: task, Width: 80})
	if got := ansi.Strip(strings.Join(withoutParent, "\n")); got != "No additional details" {
		t.Fatalf("root task Peek gained a parent line: %q", got)
	}
}

// TestInlinePeekParentLineIsHeaderRatherThanBody is the ancestor chain's half of
// the same claim: it heads the pane beside the continuation and takes nothing
// from the body's budget.
func TestInlinePeekParentLineIsHeaderRatherThanBody(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		Title:       "Child",
		Description: strings.Repeat("Wrapped description content ", 12),
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-time.Hour),
		Attachments: []models.Attachment{{Filename: "spec.pdf"}},
		Comments:    []*models.Comment{{Text: "Decision"}},
	}

	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Breadcrumb: "Parent ›", Width: 80})
	rendered := doc.Lines(0)
	if len(rendered) != len(doc.Header)+inlinePeekBodyRows {
		t.Fatalf("page = %d rows, want %d header rows plus the %d-row budget",
			len(rendered), len(doc.Header), inlinePeekBodyRows)
	}
	if !strings.HasPrefix(ansi.Strip(strings.Join(rendered, "\n")), "Parent: Parent\n") {
		t.Fatalf("parent line is not the first Peek row:\n%s", ansi.Strip(strings.Join(rendered, "\n")))
	}
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	for _, want := range []string{"created 5d · updated 1h", "Attachments: spec.pdf"} {
		if !strings.Contains(body, want) {
			t.Fatalf("the parent line displaced %q:\n%s", want, body)
		}
	}
}

func TestInlinePeekTitleContinuationEllipsizesPastItsBound(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Long title"}
	overflow := strings.Repeat("continuation ", 40)

	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, TitleOverflow: overflow, Width: 40})
	if len(doc.Header) != inlinePeekTitleRows {
		t.Fatalf("continuation rows = %d, want %d", len(doc.Header), inlinePeekTitleRows)
	}
	if plain := ansi.Strip(doc.Header[len(doc.Header)-1]); !strings.HasSuffix(plain, "…") {
		t.Fatalf("bounded continuation missing ellipsis: %q", plain)
	}
}

// TestInlinePeekWithoutOverflowIsUnchanged pins the no-clip case: a fully
// visible title adds no block, so Peek renders exactly what it did before.
func TestInlinePeekWithoutOverflowIsUnchanged(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		Title:       "Short",
		Description: "Some detail",
		Tags:        []string{"backend"},
	}
	opts := inlinePeekOptions{Task: task, Indent: "  ", Width: 60}
	withEmpty := peekFirstPage(opts)
	opts.TitleOverflow = ""
	if got := peekFirstPage(opts); strings.Join(got, "\n") != strings.Join(withEmpty, "\n") {
		t.Fatalf("empty overflow changed the Peek block")
	}
	if strings.Contains(ansi.Strip(strings.Join(withEmpty, "\n")), "Short") {
		t.Fatalf("Peek restated a title the row could show in full")
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
	if !strings.HasPrefix(plain, "  Tags: #backend") {
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
	if body != "Context: [platform-core]\nTags: #bug" {
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
		"Attachments: design-spec.pdf",
		"             GH acme/roadmap#123",
		"             notes.md",
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
