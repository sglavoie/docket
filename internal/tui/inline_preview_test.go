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
	stale := now.Add(-snoozeWakeWindow - time.Hour)

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
			},
			"created 12d · updated 2d · due 2026-08-14 · woke 1d",
		},
		{
			"created only",
			&models.Task{CreatedAt: now.Add(-3 * time.Hour)},
			"created 3h",
		},
		{
			"a pending snooze is not a wake-up",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &pending},
			"created 1d",
		},
		{
			"a lapsed snooze past the wake window falls silent",
			&models.Task{CreatedAt: now.Add(-24 * time.Hour), DeferredUntil: &stale},
			"created 1d",
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
