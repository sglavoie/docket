package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestInlinePeekNilAndEmptyTask(t *testing.T) {
	if lines := inlinePeekLines(inlinePeekOptions{}); lines != nil {
		t.Fatalf("nil task lines = %v, want nil", lines)
	}

	lines := inlinePeekLines(inlinePeekOptions{
		Task:   &models.Task{},
		Indent: "  ",
		Width:  40,
	})
	if got := ansi.Strip(strings.Join(lines, "\n")); got != "  No additional details" {
		t.Fatalf("empty Peek = %q", got)
	}
}

func TestInlinePeekShowsAdditiveContentWithinFiveLines(t *testing.T) {
	project := &models.Project{ID: "project", Slug: "project", Name: "Project"}
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
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()

	const width = 64
	rendered := inlinePeekLines(inlinePeekOptions{
		Task: task, Indent: "    ", Width: width,
	})
	plain := ansi.Strip(strings.Join(rendered, "\n"))
	if len(rendered) != 4 || len(rendered) > inlinePeekMaxLines {
		t.Fatalf("Peek line count = %d, want 4 within cap %d:\n%s", len(rendered), inlinePeekMaxLines, plain)
	}
	if got := strings.Count(plain, "\n"); got != 3 {
		t.Fatalf("description rows = %d, want the raised budget of %d:\n%s", got, inlinePeekDescriptionRows, plain)
	}
	for _, want := range []string{
		"Description:", "Comments: 1",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Peek missing %q:\n%s", want, plain)
		}
	}
	for _, banned := range []string{"#41", "#42", "#99", "State:", "Priority:", "Due:", "Checklist:", "First item"} {
		if strings.Contains(plain, banned) {
			t.Fatalf("Peek repeated %q:\n%s", banned, plain)
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
	woke := now.Add(-24 * time.Hour)
	pending := now.Add(24 * time.Hour)
	stale := now.Add(-deferWakeWindow - time.Hour)

	cases := []struct {
		name string
		task *models.Task
		want string
	}{
		{"no dates at all", &models.Task{}, ""},
		{
			"every field",
			&models.Task{
				CreatedAt:     now.Add(-12 * 24 * time.Hour),
				UpdatedAt:     now.Add(-2 * 24 * time.Hour),
				DueDate:       &due,
				DeferredUntil: &woke,
				DeferCount:    3,
			},
			"created 12d · updated 2d · due 2026-08-14 · woke 1d · deferred ×3",
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ansi.Strip(inlinePeekDatesLine(tc.task, now, 80)); got != tc.want {
				t.Fatalf("dates line = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestInlinePeekSpendsTheCapOnDescriptionAndDates locks the drop order once the
// checklist line is gone: description takes three rows, the dates line sits
// directly beneath it, and the tail cut sheds whatever no longer fits.
func TestInlinePeekSpendsTheCapOnDescriptionAndDates(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		Description: strings.Repeat("Wrapped description content ", 12),
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-time.Hour),
		Attachments: []models.Attachment{{Filename: "spec.pdf"}},
		Comments:    []*models.Comment{{Text: "Decision"}},
		Tags:        []string{"backend"},
		Checklist:   []models.ChecklistItem{{Text: "First item"}},
	}

	rendered := inlinePeekLines(inlinePeekOptions{Task: task, Width: 80})
	if len(rendered) != inlinePeekMaxLines {
		t.Fatalf("Peek line count = %d, want %d", len(rendered), inlinePeekMaxLines)
	}
	plain := ansi.Strip(strings.Join(rendered, "\n"))
	if !strings.Contains(plain, "created 5d · updated 1h") {
		t.Fatalf("Peek dropped the dates line:\n%s", plain)
	}
	if !strings.Contains(plain, "Attachments: spec.pdf") {
		t.Fatalf("Peek dropped the attachments line:\n%s", plain)
	}
	for _, banned := range []string{"Checklist:", "First item"} {
		if strings.Contains(plain, banned) {
			t.Fatalf("Peek still renders %q:\n%s", banned, plain)
		}
	}
}

// TestInlinePeekCompletesAClippedTitle locks the continuation block: it is bare
// (no label), it carries no ellipsis while it fits inside inlinePeekTitleRows,
// and it never repeats the head the row already showed.
func TestInlinePeekCompletesAClippedTitle(t *testing.T) {
	const head = "Investigate intermittent timeout when syncing "
	const tail = "very large project archives over slow connections"
	task := &models.Task{ID: "task", Title: head + tail}

	rendered := inlinePeekLines(inlinePeekOptions{
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

// TestInlinePeekTitleContinuationRaisesTheCap proves the continuation displaces
// nothing: with a three-row description beside it, the dates line still renders
// because the cap grew by the continuation rows rather than absorbing them.
func TestInlinePeekTitleContinuationRaisesTheCap(t *testing.T) {
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
	rendered := inlinePeekLines(inlinePeekOptions{Task: task, TitleOverflow: overflow, Width: 80})
	if len(rendered) != inlinePeekMaxLines+1 {
		t.Fatalf("Peek line count = %d, want %d", len(rendered), inlinePeekMaxLines+1)
	}
	plain := ansi.Strip(strings.Join(rendered, "\n"))
	if !strings.HasPrefix(plain, overflow) {
		t.Fatalf("continuation is not the first Peek row:\n%s", plain)
	}
	for _, want := range []string{"created 5d · updated 1h", "Attachments: spec.pdf"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("continuation displaced %q:\n%s", want, plain)
		}
	}
}

// TestInlinePeekNamesThePromotedAncestorChain locks the surface the row's bare
// `‹` points at: the full chain, root-first, only when the row is a promoted
// section root.
func TestInlinePeekNamesThePromotedAncestorChain(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Child"}

	withParent := ansi.Strip(strings.Join(inlinePeekLines(inlinePeekOptions{
		Task: task, Breadcrumb: "Launch platform › User authentication ›", Width: 80,
	}), "\n"))
	if withParent != "Parent: Launch platform › User authentication" {
		t.Fatalf("parent line = %q", withParent)
	}

	withoutParent := inlinePeekLines(inlinePeekOptions{Task: task, Width: 80})
	if got := ansi.Strip(strings.Join(withoutParent, "\n")); got != "No additional details" {
		t.Fatalf("root task Peek gained a parent line: %q", got)
	}
}

// TestInlinePeekParentLineRaisesTheCap proves the parent line displaces nothing:
// beside a three-row description the tags line still renders, because the cap
// grew by the parent row exactly as it does for the title continuation.
func TestInlinePeekParentLineRaisesTheCap(t *testing.T) {
	task := &models.Task{
		ID:          "task",
		Title:       "Child",
		Description: strings.Repeat("Wrapped description content ", 12),
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour),
		UpdatedAt:   time.Now().Add(-time.Hour),
		Attachments: []models.Attachment{{Filename: "spec.pdf"}},
		Comments:    []*models.Comment{{Text: "Decision"}},
	}

	rendered := inlinePeekLines(inlinePeekOptions{Task: task, Breadcrumb: "Parent ›", Width: 80})
	if len(rendered) != inlinePeekMaxLines+1 {
		t.Fatalf("Peek line count = %d, want %d", len(rendered), inlinePeekMaxLines+1)
	}
	plain := ansi.Strip(strings.Join(rendered, "\n"))
	if !strings.HasPrefix(plain, "Parent: Parent\n") {
		t.Fatalf("parent line is not the first Peek row:\n%s", plain)
	}
	for _, want := range []string{"created 5d · updated 1h", "Attachments: spec.pdf"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("the parent line displaced %q:\n%s", want, plain)
		}
	}
}

func TestInlinePeekTitleContinuationEllipsizesPastItsBound(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Long title"}
	overflow := strings.Repeat("continuation ", 40)

	rendered := inlinePeekLines(inlinePeekOptions{Task: task, TitleOverflow: overflow, Width: 40})
	if len(rendered) != inlinePeekTitleRows {
		t.Fatalf("continuation rows = %d, want %d", len(rendered), inlinePeekTitleRows)
	}
	if plain := ansi.Strip(rendered[len(rendered)-1]); !strings.HasSuffix(plain, "…") {
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
	withEmpty := inlinePeekLines(opts)
	opts.TitleOverflow = ""
	if got := inlinePeekLines(opts); strings.Join(got, "\n") != strings.Join(withEmpty, "\n") {
		t.Fatalf("empty overflow changed the Peek block")
	}
	if strings.Contains(ansi.Strip(strings.Join(withEmpty, "\n")), "Short") {
		t.Fatalf("Peek restated a title the row could show in full")
	}
}

func TestInlinePeekShowsTagsWithinAvailableWidth(t *testing.T) {
	task := &models.Task{
		ID:   "task",
		Tags: []string{"backend", "security", "type:authentication"},
	}

	styled := inlinePeekTagsLine(task, 80)
	wantStyled := MutedStyle.Render("Tags: ") +
		TagInlineStyle.Bold(true).Render("#backend #security #type:authentication")
	if styled != wantStyled {
		t.Fatalf("styled Peek tags = %q, want %q", styled, wantStyled)
	}

	const width = 32
	lines := inlinePeekLines(inlinePeekOptions{Task: task, Indent: "  ", Width: width})
	if len(lines) != 1 {
		t.Fatalf("tags-only Peek lines = %d, want 1", len(lines))
	}
	plain := ansi.Strip(lines[0])
	if !strings.HasPrefix(plain, "  Tags: #backend #security") {
		t.Fatalf("Peek tags = %q", plain)
	}
	if !strings.HasSuffix(plain, "…") {
		t.Fatalf("truncated Peek tags missing ellipsis: %q", plain)
	}
	if got := lipgloss.Width(lines[0]); got > width {
		t.Fatalf("Peek tags width = %d, want <= %d", got, width)
	}
}

func TestInlinePeekFoldsOnlyTruncatedContextIntoTagsLine(t *testing.T) {
	short := &models.Task{Context: "platform"}
	if line := inlinePeekTagsLine(short, 80); line != "" {
		t.Fatalf("short context was duplicated in Peek: %q", ansi.Strip(line))
	}

	long := &models.Task{Context: "platform-core", Tags: []string{"bug"}}
	plain := ansi.Strip(inlinePeekTagsLine(long, 80))
	if plain != "Tags: [platform-core] #bug" {
		t.Fatalf("long context Peek line = %q", plain)
	}
}

func TestInlinePeekShowsAttachmentNames(t *testing.T) {
	task := &models.Task{
		Attachments: []models.Attachment{
			{Filename: "design-spec.pdf"},
			{Filename: "https://github.com/acme/roadmap/issues/123", IsURL: true},
			{Filename: "notes.md"},
		},
	}

	plain := ansi.Strip(strings.Join(inlinePeekLines(inlinePeekOptions{Task: task, Width: 80}), "\n"))
	for _, want := range []string{"Attachments: design-spec.pdf · acme/roadmap#123 · +1 more"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Peek missing %q:\n%s", want, plain)
		}
	}
}

func TestInlinePeekTruncatesLongCategoryLines(t *testing.T) {
	task := &models.Task{
		ID: "task",
		Attachments: []models.Attachment{
			{Filename: strings.Repeat("界", 40) + ".pdf"},
			{Filename: "second.pdf"},
			{Filename: "third.pdf"},
		},
	}
	lines := inlinePeekLines(inlinePeekOptions{Task: task, Indent: "  ", Width: 24})
	if len(lines) != 1 {
		t.Fatalf("attachments-only Peek lines = %d, want 1", len(lines))
	}
	if !strings.Contains(ansi.Strip(lines[0]), "…") {
		t.Fatalf("truncated attachments missing ellipsis: %q", ansi.Strip(lines[0]))
	}
	if got := lipgloss.Width(lines[0]); got > 24 {
		t.Fatalf("truncated attachments width = %d", got)
	}
}
