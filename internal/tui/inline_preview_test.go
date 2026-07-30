package tui

import (
	"strings"
	"testing"

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
	needed := &models.Task{ID: "needed", ShortID: 41, Title: "Prepare 日本語 API"}
	blocked := &models.Task{
		ID: "blocked", ShortID: 42, Title: "Release client",
		DependsOn: []string{"task"},
	}
	task := &models.Task{
		ID:          "task",
		ShortID:     99,
		Title:       "Selected",
		Description: strings.Repeat("Unicode café 界 content ", 12),
		DependsOn:   []string{needed.ID},
		Checklist: []models.ChecklistItem{
			{Text: "First item", Done: true},
			{Text: "Second item"},
			{Text: "Third item"},
		},
		Recurrence: &models.RecurrencePattern{Interval: 2, Unit: models.RecurrenceWeekly},
	}
	project.Tasks = []*models.Task{needed, task, blocked}
	project.BuildTaskTree()

	const width = 64
	rendered := inlinePeekLines(inlinePeekOptions{
		Task: task, Project: project, Projects: []*models.Project{project},
		Indent: "    ", Width: width,
	})
	plain := ansi.Strip(strings.Join(rendered, "\n"))
	if len(rendered) != inlinePeekMaxLines {
		t.Fatalf("Peek line count = %d, want %d:\n%s", len(rendered), inlinePeekMaxLines, plain)
	}
	for _, want := range []string{
		"Description:", "Checklist:", "Needs: Prepare", "Blocks: Release", "Repeats: every 2 weeks",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Peek missing %q:\n%s", want, plain)
		}
	}
	for _, banned := range []string{"#41", "#42", "#99", "State:", "Priority:", "Due:"} {
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

func TestInlinePeekShowsAttachmentNamesAndDecisionCount(t *testing.T) {
	task := &models.Task{
		Attachments: []models.Attachment{
			{Filename: "design-spec.pdf"},
			{Filename: "https://github.com/acme/roadmap/issues/123", IsURL: true},
			{Filename: "notes.md"},
		},
		Decisions: []*models.Decision{{Text: "Use the compact layout"}, {Text: "Keep Peek concise"}},
	}

	plain := ansi.Strip(strings.Join(inlinePeekLines(inlinePeekOptions{Task: task, Width: 80}), "\n"))
	for _, want := range []string{
		"Attachments: design-spec.pdf · acme/roadmap#123 · +1 more",
		"Decisions: 2",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Peek missing %q:\n%s", want, plain)
		}
	}
}

func TestInlinePeekQualifiesCrossProjectDependencyWithoutTaskID(t *testing.T) {
	owner := &models.Project{ID: "owner", Slug: "own", Name: "Owner"}
	other := &models.Project{ID: "other", Slug: "api", Name: "API"}
	task := &models.Task{ID: "task", Title: "Ship", DependsOn: []string{"other:dependency"}}
	dependency := &models.Task{ID: "dependency", ShortID: 7, Title: "Publish schema"}
	owner.Tasks = []*models.Task{task}
	other.Tasks = []*models.Task{dependency}

	plain := ansi.Strip(strings.Join(inlinePeekLines(inlinePeekOptions{
		Task: task, Project: owner, Projects: []*models.Project{owner, other}, Width: 80,
	}), "\n"))
	if !strings.Contains(plain, "Needs: [api] Publish schema") {
		t.Fatalf("cross-project Peek = %q", plain)
	}
	if strings.Contains(plain, "#7") {
		t.Fatalf("cross-project Peek leaked short ID: %q", plain)
	}
}

func TestInlinePeekTruncatesLongCategoryLines(t *testing.T) {
	task := &models.Task{
		ID: "task",
		Checklist: []models.ChecklistItem{
			{Text: strings.Repeat("界", 40)},
			{Text: "second"},
			{Text: "third"},
		},
	}
	lines := inlinePeekLines(inlinePeekOptions{Task: task, Indent: "  ", Width: 24})
	if len(lines) != 1 {
		t.Fatalf("checklist-only Peek lines = %d, want 1", len(lines))
	}
	if !strings.Contains(ansi.Strip(lines[0]), "…") {
		t.Fatalf("truncated checklist missing ellipsis: %q", ansi.Strip(lines[0]))
	}
	if got := lipgloss.Width(lines[0]); got > 24 {
		t.Fatalf("truncated checklist width = %d", got)
	}
}
