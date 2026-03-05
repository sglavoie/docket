package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestInlinePreviewLines_NilTask(t *testing.T) {
	lines := inlinePreviewLines(nil, "  ", 80)
	if lines != nil {
		t.Errorf("expected nil for nil task, got %v", lines)
	}
}

func TestInlinePreviewLines_NoDescriptionNoChecklist(t *testing.T) {
	task := &models.Task{}
	lines := inlinePreviewLines(task, "  ", 80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// Should contain "(no description)"
	if lines[0] == "" {
		t.Error("expected non-empty line for no-description task")
	}
}

func TestInlinePreviewLines_DescriptionOnly(t *testing.T) {
	task := &models.Task{Description: "This is a test description"}
	lines := inlinePreviewLines(task, "  ", 80)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line for single-line description, got %d", len(lines))
	}
}

func TestInlinePreviewLines_MultiLineDescription(t *testing.T) {
	task := &models.Task{Description: "Line one\nLine two\nLine three"}
	lines := inlinePreviewLines(task, "  ", 80)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines for multi-line description (capped at 2), got %d", len(lines))
	}
}

func TestInlinePreviewLines_ChecklistOnly(t *testing.T) {
	task := &models.Task{
		Checklist: []models.ChecklistItem{
			{Text: "Item 1", Done: false},
			{Text: "Item 2", Done: true},
		},
	}
	lines := inlinePreviewLines(task, "  ", 80)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines for 2 checklist items, got %d", len(lines))
	}
}

func TestInlinePreviewLines_ChecklistWithMore(t *testing.T) {
	task := &models.Task{
		Checklist: []models.ChecklistItem{
			{Text: "Item 1", Done: false},
			{Text: "Item 2", Done: true},
			{Text: "Item 3", Done: false},
			{Text: "Item 4", Done: false},
		},
	}
	lines := inlinePreviewLines(task, "  ", 80)
	// 2 items + "+2 more"
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (2 items + more), got %d", len(lines))
	}
}

func TestInlinePreviewLines_DescriptionAndChecklist(t *testing.T) {
	task := &models.Task{
		Description: "Some description",
		Checklist: []models.ChecklistItem{
			{Text: "Check this", Done: false},
		},
	}
	lines := inlinePreviewLines(task, "  ", 80)
	// 1 description + 1 checklist = 2
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestInlinePreviewLineCount_NilTask(t *testing.T) {
	count := inlinePreviewLineCount(nil)
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestInlinePreviewLineCount_NoContent(t *testing.T) {
	task := &models.Task{}
	count := inlinePreviewLineCount(task)
	if count != 1 {
		t.Errorf("expected 1 (no description placeholder), got %d", count)
	}
}

func TestInlinePreviewLineCount_DescAndChecklist(t *testing.T) {
	task := &models.Task{
		Description: "Hello\nWorld",
		Checklist: []models.ChecklistItem{
			{Text: "A", Done: false},
			{Text: "B", Done: true},
			{Text: "C", Done: false},
		},
	}
	// 2 desc lines + 2 checklist + 1 "+more" = 5
	count := inlinePreviewLineCount(task)
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}
