package outliner

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// TestInlinePeekStatusLineNamesWhatTheRowAbbreviates is the summary line's own
// claim: the row spells state and priority as glyphs and says nothing at all
// about planning membership, and Peek spells all four in words a reader can
// search for. Each field is omitted when the task does not carry it, and the
// status is the one that never is.
//
// Plan leads status, which is the vocabulary's own order and the order the
// workflow chord and the capture route read in; the two are adjacent so a task
// planned for Today reports both halves of what one sigil decided.
func TestInlinePeekStatusLineNamesWhatTheRowAbbreviates(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		task     *models.Task
		planning string
		want     string
	}{
		{"bare task", &models.Task{}, "", "Status: Backlog"},
		{
			"doing carries its age",
			&models.Task{WorkflowState: models.StateDoing, StateChangedAt: now.Add(-50 * time.Hour)},
			"",
			"Status: Doing 2d",
		},
		{
			"waiting carries its age",
			&models.Task{WorkflowState: models.StateWaiting, StateChangedAt: now.Add(-90 * time.Minute)},
			"",
			"Status: Waiting 1h",
		},
		{
			"a quiet state has no age to give",
			&models.Task{WorkflowState: models.StateSomeday, StateChangedAt: now.Add(-90 * time.Hour)},
			"",
			"Status: Someday",
		},
		{
			"an unset priority names none",
			&models.Task{WorkflowState: models.StateInbox},
			"",
			"Status: Inbox",
		},
		{
			// The pair the session is about: planning for Today leaves the task in
			// Backlog, and the line reports the membership and the state side by
			// side rather than one of them.
			"a task planned for today reports both halves",
			&models.Task{WorkflowState: models.StateBacklog},
			"Today",
			"Plan: Today · Status: Backlog",
		},
		{
			"every axis at once",
			&models.Task{WorkflowState: models.StatePaused, Priority: 1, Pinned: true, Review: true},
			"Next",
			"Plan: Next · Status: Paused · Priority: 1 high · Pinned · Review",
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

	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: 80, Window: testPeekWindow()})
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

	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Width: 40, Window: testPeekWindow()})
	if len(doc.Lines(0)) != testPeekWindowRows+inlinePeekPositionRows {
		t.Fatalf("first page = %d rows, want the %d-row window", len(doc.Lines(0)), testPeekWindowRows+inlinePeekPositionRows)
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
	for _, want := range []string{"Created: 5d ago · Updated: 1h ago", "Attachments: 1 · spec.pdf"} {
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
	// The chain follows the title and status, before the remaining metadata.
	if withParent != "Child\nStatus: Backlog\nParent: Launch platform › User authentication" {
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
	body := ansi.Strip(strings.Join(inlinePeekDocumentFor(inlinePeekOptions{Task: long, Width: 80, Window: testPeekWindow()}).Body, "\n"))
	if body != "Status: Backlog\nContext: [platform-core]\nTags: #bug" {
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
		"Attachments: 3 · design-spec.pdf",
		"                 GH acme/roadmap#123",
		"                 notes.md",
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
	for _, narrow := range []int{22, 24} {
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

// TestInlinePeekLinksEachWrappedAttachmentRow pins the OSC 8 half: every row of
// a wrapped URL opens and closes its own link to the whole URL, a shortened
// GitHub name links to the issue, a code reference links its file, and a
// stored file links nothing. The links change no width.
func TestInlinePeekLinksEachWrappedAttachmentRow(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("segment/", 12)
	task := &models.Task{
		Attachments: []models.Attachment{
			{Filename: long, IsURL: true},
			{Filename: "https://github.com/acme/roadmap/issues/123", IsURL: true},
			{OriginalPath: "/src/app/main.go", LineNumber: 12, IsCodeRef: true},
			{Filename: "notes.md"},
		},
	}
	const width = 40
	lines := inlinePeekAttachmentsLines(task, "", width)
	open := func(uri string) string { return ansi.SetHyperlink(uri) }
	var urlRows int
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("row %d width = %d, want <= %d", i, got, width)
		}
		plain := ansi.Strip(line)
		switch {
		case strings.Contains(plain, "GH acme/roadmap#123"):
			if !strings.Contains(line, open("https://github.com/acme/roadmap/issues/123")) {
				t.Fatalf("GitHub row %q does not link the issue", line)
			}
		case strings.Contains(plain, "main.go:12"):
			if !strings.Contains(line, open("file:///src/app/main.go")) {
				t.Fatalf("code reference row %q does not link its file", line)
			}
		case strings.Contains(plain, "notes.md"):
			if strings.Contains(line, "\x1b]8;;h") || strings.Contains(line, "\x1b]8;;f") {
				t.Fatalf("stored file row %q carries a link", line)
			}
		default:
			urlRows++
			if !strings.Contains(line, open(long)) || !strings.Contains(line, ansi.ResetHyperlink()) {
				t.Fatalf("URL row %d %q does not open and close its own link", i, line)
			}
		}
	}
	if urlRows < 2 {
		t.Fatalf("the long URL wrapped over %d rows, want several", urlRows)
	}
}
