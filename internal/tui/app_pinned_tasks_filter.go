package tui

import (
	"sort"

	"github.com/sglavoie/tt/internal/fuzzy"
)

// rebuildPinnedPopupFilter rebuilds the filtered indices based on current filter text.
func (m *AppModel) rebuildPinnedPopupFilter() {
	items := m.pinnedPopupItems
	if m.pinnedPopupFilter == "" {
		m.pinnedPopupFiltered = make([]int, len(items))
		m.pinnedPopupTitlePos = make([][]int, len(items))
		for i := range items {
			m.pinnedPopupFiltered[i] = i
		}
		return
	}

	type scored struct {
		idx            int
		score          int
		titlePositions []int
	}
	var results []scored
	for i, entry := range items {
		text := entry.Title + " " + entry.ProjectName
		score := fuzzy.ScoreTerms(text, m.pinnedPopupFilter)
		if score <= 0 {
			continue
		}
		_, titlePos := fuzzy.ScoreTermsWithPositions(entry.Title, m.pinnedPopupFilter)
		results = append(results, scored{idx: i, score: score, titlePositions: titlePos})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	m.pinnedPopupFiltered = make([]int, 0, len(results))
	m.pinnedPopupTitlePos = make([][]int, 0, len(results))
	for _, s := range results {
		m.pinnedPopupFiltered = append(m.pinnedPopupFiltered, s.idx)
		m.pinnedPopupTitlePos = append(m.pinnedPopupTitlePos, s.titlePositions)
	}
}

// pinnedPopupFilteredLen returns the number of visible (filtered) entries.
func (m *AppModel) pinnedPopupFilteredLen() int {
	return len(m.pinnedPopupFiltered)
}

// pinnedPopupClampCursor clamps the cursor to the filtered list bounds.
func (m *AppModel) pinnedPopupClampCursor() {
	n := m.pinnedPopupFilteredLen()
	if n == 0 {
		m.pinnedPopupCursor = 0
	} else if m.pinnedPopupCursor >= n {
		m.pinnedPopupCursor = n - 1
	}
}
