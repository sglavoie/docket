package tui

import "testing"

func TestCalculateSectionNav_Empty(t *testing.T) {
	result := CalculateSectionNav(nil, nil, nil, nil)
	if result.TotalItems != 0 {
		t.Errorf("expected TotalItems=0, got %d", result.TotalItems)
	}
	if len(result.SectionOrder) != 0 {
		t.Errorf("expected empty SectionOrder, got %v", result.SectionOrder)
	}
}

func TestCalculateSectionNav_AllFilteredByTab(t *testing.T) {
	sections := []SectionEntry{
		{ID: 0, Count: 3},
		{ID: 1, Count: 5},
	}
	visibleSet := map[int]bool{99: true} // neither 0 nor 1
	result := CalculateSectionNav(sections, nil, visibleSet, nil)
	if result.TotalItems != 0 {
		t.Errorf("expected TotalItems=0, got %d", result.TotalItems)
	}
}

func TestCalculateSectionNav_MixedCollapsed(t *testing.T) {
	sections := []SectionEntry{
		{ID: 0, Count: 3},
		{ID: 1, Count: 5},
		{ID: 2, Count: 2},
	}
	collapsed := map[int]bool{1: true} // section 1 is collapsed

	result := CalculateSectionNav(sections, collapsed, nil, nil)

	// Section 0: header(1) + 3 items = 4
	// Section 1: header(1) + 0 (collapsed) = 1
	// Section 2: header(1) + 2 items = 3
	// Total = 8
	if result.TotalItems != 8 {
		t.Errorf("expected TotalItems=8, got %d", result.TotalItems)
	}
	if len(result.SectionOrder) != 3 {
		t.Errorf("expected 3 sections, got %d", len(result.SectionOrder))
	}

	// Verify section starts
	expectedStarts := []int{0, 4, 5}
	for i, want := range expectedStarts {
		if result.SectionStarts[i] != want {
			t.Errorf("SectionStarts[%d]: expected %d, got %d", i, want, result.SectionStarts[i])
		}
	}
}

func TestCalculateSectionNav_FocusedSection(t *testing.T) {
	sections := []SectionEntry{
		{ID: 0, Count: 3},
		{ID: 1, Count: 5},
		{ID: 2, Count: 2},
	}
	focused := 1

	result := CalculateSectionNav(sections, nil, nil, &focused)

	if len(result.SectionOrder) != 1 {
		t.Fatalf("expected 1 section, got %d", len(result.SectionOrder))
	}
	if result.SectionOrder[0] != 1 {
		t.Errorf("expected section ID 1, got %d", result.SectionOrder[0])
	}
	// header(1) + 5 items = 6
	if result.TotalItems != 6 {
		t.Errorf("expected TotalItems=6, got %d", result.TotalItems)
	}
}

func TestCalculateSectionNav_VisibleSetFilters(t *testing.T) {
	sections := []SectionEntry{
		{ID: 0, Count: 3},
		{ID: 1, Count: 5},
		{ID: 2, Count: 2},
	}
	visibleSet := map[int]bool{0: true, 2: true}

	result := CalculateSectionNav(sections, nil, visibleSet, nil)

	if len(result.SectionOrder) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result.SectionOrder))
	}
	if result.SectionOrder[0] != 0 || result.SectionOrder[1] != 2 {
		t.Errorf("expected section IDs [0,2], got %v", result.SectionOrder)
	}
	// Section 0: 1+3=4, Section 2: 1+2=3, Total=7
	if result.TotalItems != 7 {
		t.Errorf("expected TotalItems=7, got %d", result.TotalItems)
	}
}

func TestCalculateSectionNav_EmptySectionsSkipped(t *testing.T) {
	sections := []SectionEntry{
		{ID: 0, Count: 0},
		{ID: 1, Count: 3},
		{ID: 2, Count: 0},
	}

	result := CalculateSectionNav(sections, nil, nil, nil)

	if len(result.SectionOrder) != 1 {
		t.Fatalf("expected 1 section, got %d", len(result.SectionOrder))
	}
	if result.SectionOrder[0] != 1 {
		t.Errorf("expected section ID 1, got %d", result.SectionOrder[0])
	}
}
