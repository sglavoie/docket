package tui

// SectionEntry represents a section with its ID and item count for navigation calculation.
type SectionEntry struct {
	ID    int
	Count int
}

// SectionNavResult holds the computed navigation indices for a set of sections.
type SectionNavResult struct {
	SectionOrder  []int
	SectionStarts []int
	TotalItems    int
}

// CalculateSectionNav builds navigation indices from a list of sections,
// filtering by tab visibility and focus. Each visible, non-empty section
// contributes a header (1 navigable item) plus its item count (if not collapsed).
//
// visibleSet: if non-nil, only sections whose ID is in this set are included.
// focusedSection: if non-nil, only the section matching this ID is included.
func CalculateSectionNav(
	sections []SectionEntry,
	collapsedSections map[int]bool,
	visibleSet map[int]bool,
	focusedSection *int,
) SectionNavResult {
	var result SectionNavResult

	for _, s := range sections {
		if s.Count == 0 {
			continue
		}
		if visibleSet != nil && !visibleSet[s.ID] {
			continue
		}
		if focusedSection != nil && *focusedSection != s.ID {
			continue
		}
		result.SectionOrder = append(result.SectionOrder, s.ID)
		result.SectionStarts = append(result.SectionStarts, result.TotalItems)
		result.TotalItems++ // header
		if !collapsedSections[s.ID] {
			result.TotalItems += s.Count
		}
	}

	return result
}
