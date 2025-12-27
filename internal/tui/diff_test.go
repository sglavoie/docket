package tui

import (
	"testing"
)

func TestComputeContextualDiff_IdenticalContent(t *testing.T) {
	content := "line1\nline2\nline3\n"
	result := ComputeContextualDiff(content, content, 0)

	for i, line := range result {
		if line.Kind != DiffContext {
			t.Errorf("line %d: expected DiffContext, got %v", i, line.Kind)
		}
	}
}

func TestComputeContextualDiff_SimpleAdd(t *testing.T) {
	old := "line1\nline2\n"
	new := "line1\ninserted\nline2\n"

	result := ComputeContextualDiff(old, new, 0)

	expected := []DiffLine{
		{Kind: DiffContext, Content: "line1"},
		{Kind: DiffAdd, Content: "inserted"},
		{Kind: DiffContext, Content: "line2"},
	}

	if len(result) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %+v", len(expected), len(result), result)
	}

	for i, exp := range expected {
		if result[i].Kind != exp.Kind || result[i].Content != exp.Content {
			t.Errorf("line %d: expected {%v, %q}, got {%v, %q}",
				i, exp.Kind, exp.Content, result[i].Kind, result[i].Content)
		}
	}
}

func TestComputeContextualDiff_SimpleRemove(t *testing.T) {
	old := "line1\nremoved\nline2\n"
	new := "line1\nline2\n"

	result := ComputeContextualDiff(old, new, 0)

	expected := []DiffLine{
		{Kind: DiffContext, Content: "line1"},
		{Kind: DiffRemove, Content: "removed"},
		{Kind: DiffContext, Content: "line2"},
	}

	if len(result) != len(expected) {
		t.Fatalf("expected %d lines, got %d: %+v", len(expected), len(result), result)
	}

	for i, exp := range expected {
		if result[i].Kind != exp.Kind || result[i].Content != exp.Content {
			t.Errorf("line %d: expected {%v, %q}, got {%v, %q}",
				i, exp.Kind, exp.Content, result[i].Kind, result[i].Content)
		}
	}
}

func TestComputeContextualDiff_Modification(t *testing.T) {
	old := "line1\nold content\nline3\n"
	new := "line1\nnew content\nline3\n"

	result := ComputeContextualDiff(old, new, 0)

	// Should show: context, remove, add, context
	hasRemove := false
	hasAdd := false
	for _, line := range result {
		if line.Kind == DiffRemove && line.Content == "old content" {
			hasRemove = true
		}
		if line.Kind == DiffAdd && line.Content == "new content" {
			hasAdd = true
		}
	}

	if !hasRemove {
		t.Error("expected to find removal of 'old content'")
	}
	if !hasAdd {
		t.Error("expected to find addition of 'new content'")
	}
}

func TestComputeContextualDiff_EmptyOld(t *testing.T) {
	old := ""
	new := "line1\nline2\n"

	result := ComputeContextualDiff(old, new, 0)

	for _, line := range result {
		if line.Kind != DiffAdd {
			t.Errorf("expected all DiffAdd, got %v for %q", line.Kind, line.Content)
		}
	}
}

func TestComputeContextualDiff_EmptyNew(t *testing.T) {
	old := "line1\nline2\n"
	new := ""

	result := ComputeContextualDiff(old, new, 0)

	for _, line := range result {
		if line.Kind != DiffRemove {
			t.Errorf("expected all DiffRemove, got %v for %q", line.Kind, line.Content)
		}
	}
}

func TestComputeContextualDiff_BothEmpty(t *testing.T) {
	result := ComputeContextualDiff("", "", 0)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d lines", len(result))
	}
}

func TestFilterWithContext(t *testing.T) {
	lines := []DiffLine{
		{Kind: DiffContext, Content: "ctx1"},
		{Kind: DiffContext, Content: "ctx2"},
		{Kind: DiffContext, Content: "ctx3"},
		{Kind: DiffRemove, Content: "removed"},
		{Kind: DiffAdd, Content: "added"},
		{Kind: DiffContext, Content: "ctx4"},
		{Kind: DiffContext, Content: "ctx5"},
		{Kind: DiffContext, Content: "ctx6"},
		{Kind: DiffContext, Content: "ctx7"},
	}

	result := filterWithContext(lines, 2)

	// With contextLines=2, should include:
	// ctx2, ctx3 (2 before change), removed, added, ctx4, ctx5 (2 after change)
	// ctx1 and ctx6, ctx7 should be excluded

	if len(result) != 6 {
		t.Fatalf("expected 6 lines with context=2, got %d: %+v", len(result), result)
	}

	expectedContents := []string{"ctx2", "ctx3", "removed", "added", "ctx4", "ctx5"}
	for i, exp := range expectedContents {
		if result[i].Content != exp {
			t.Errorf("line %d: expected %q, got %q", i, exp, result[i].Content)
		}
	}
}

func TestFilterWithContext_NoChanges(t *testing.T) {
	lines := []DiffLine{
		{Kind: DiffContext, Content: "ctx1"},
		{Kind: DiffContext, Content: "ctx2"},
		{Kind: DiffContext, Content: "ctx3"},
	}

	result := filterWithContext(lines, 2)

	// No changes means no lines should be included
	if len(result) != 0 {
		t.Errorf("expected 0 lines when no changes, got %d", len(result))
	}
}

func TestFilterWithContext_Empty(t *testing.T) {
	result := filterWithContext([]DiffLine{}, 3)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d lines", len(result))
	}
}

func TestComputeContextualDiff_WithContextFiltering(t *testing.T) {
	old := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	new := "a\nb\nc\nd\nX\nf\ng\nh\ni\nj\n"

	// With contextLines=1, should only show d, e->X, f
	result := ComputeContextualDiff(old, new, 1)

	// Find the change
	hasRemove := false
	hasAdd := false
	for _, line := range result {
		if line.Kind == DiffRemove && line.Content == "e" {
			hasRemove = true
		}
		if line.Kind == DiffAdd && line.Content == "X" {
			hasAdd = true
		}
	}

	if !hasRemove || !hasAdd {
		t.Errorf("expected to find the e->X change in filtered result: %+v", result)
	}

	// Should not include lines far from the change
	for _, line := range result {
		if line.Content == "a" || line.Content == "b" || line.Content == "i" || line.Content == "j" {
			t.Errorf("unexpected distant line included: %q", line.Content)
		}
	}
}

func TestSummarizeDiff(t *testing.T) {
	lines := []DiffLine{
		{Kind: DiffContext, Content: "ctx"},
		{Kind: DiffRemove, Content: "r1"},
		{Kind: DiffRemove, Content: "r2"},
		{Kind: DiffAdd, Content: "a1"},
		{Kind: DiffContext, Content: "ctx2"},
	}

	added, removed := SummarizeDiff(lines)

	if added != 1 {
		t.Errorf("expected 1 added, got %d", added)
	}
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
}

func TestFilterDiffToChanges(t *testing.T) {
	lines := []DiffLine{
		{Kind: DiffContext, Content: "ctx1"},
		{Kind: DiffRemove, Content: "removed"},
		{Kind: DiffContext, Content: "ctx2"},
		{Kind: DiffAdd, Content: "added"},
		{Kind: DiffContext, Content: "ctx3"},
	}

	result := FilterDiffToChanges(lines)

	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}

	if result[0].Kind != DiffRemove || result[0].Content != "removed" {
		t.Errorf("expected first line to be remove, got %+v", result[0])
	}
	if result[1].Kind != DiffAdd || result[1].Content != "added" {
		t.Errorf("expected second line to be add, got %+v", result[1])
	}
}

func TestComputeContextualDiff_LineReordering(t *testing.T) {
	// Myers algorithm should produce optimal diff for reordering
	old := "A\nB\nC\nD\n"
	new := "A\nC\nD\nB\n"

	result := ComputeContextualDiff(old, new, 0)

	// Count the changes - Myers should minimize edits
	added, removed := SummarizeDiff(result)

	// Moving B from position 2 to position 4 should result in
	// removing B from old position and adding it at new position
	if added != 1 || removed != 1 {
		t.Errorf("expected optimal diff with 1 add and 1 remove for line move, got +%d/-%d", added, removed)
	}
}
