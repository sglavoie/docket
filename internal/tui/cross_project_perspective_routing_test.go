package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectRebuildRowsPerspectiveRouting(t *testing.T) {
	projAlpha := &models.Project{
		ID:   "route-alpha",
		Name: "Alpha",
		Tasks: []*models.Task{
			makeTask("ra1", "Alpha task", ""),
		},
	}
	projAlpha.BuildTaskTree()

	projBeta := &models.Project{
		ID:   "route-beta",
		Name: "Beta",
		Tasks: []*models.Task{
			makeTask("rb1", "Beta task", ""),
		},
	}
	projBeta.BuildTaskTree()

	t.Run("All perspective routes to buildCrossProjectRowsWithSectionHeaders", func(t *testing.T) {
		m := &OutlinerModel{
			collapsedSections: make(map[string]bool),
			expandedNodes:     make(map[string]bool),
			MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
			activePerspective: PerspectiveAll,
			loadedProjects:    []*models.Project{projAlpha, projBeta},
		}

		m.crossProjectRebuildRows()

		if len(m.rows) == 0 {
			t.Fatal("expected non-empty rows for All perspective")
		}

		sectionHeaderCount := 0
		separatorCount := 0
		for _, row := range m.rows {
			if row.IsSectionHeader {
				sectionHeaderCount++
			}
			if row.IsSeparator {
				separatorCount++
			}
		}

		if sectionHeaderCount == 0 {
			t.Error("All perspective: expected at least one IsSectionHeader row")
		}
		if separatorCount != 0 {
			t.Errorf("All perspective: expected zero IsSeparator rows, got %d", separatorCount)
		}
	})

	t.Run("non-All perspective routes to buildProjectGroupedRows", func(t *testing.T) {
		p1task := makeTask("np1", "P1 active task", "")
		p1task.Status = models.StatusInProgress
		p1 := &models.Project{
			ID:    "nonall-p1",
			Name:  "P1",
			Tasks: []*models.Task{p1task},
		}
		p1.BuildTaskTree()

		p2task := makeTask("np2", "P2 active task", "")
		p2task.Status = models.StatusInProgress
		p2 := &models.Project{
			ID:    "nonall-p2",
			Name:  "P2",
			Tasks: []*models.Task{p2task},
		}
		p2.BuildTaskTree()

		m := &OutlinerModel{
			collapsedSections: make(map[string]bool),
			expandedNodes:     make(map[string]bool),
			MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
			activePerspective: PerspectiveAll + 1,
			loadedProjects:    []*models.Project{p1, p2},
		}

		m.crossProjectRebuildRows()

		if len(m.rows) == 0 {
			t.Fatal("expected non-empty rows for non-All perspective with two projects")
		}

		sectionHeaderCount := 0
		separatorCount := 0
		for _, row := range m.rows {
			if row.IsSectionHeader {
				sectionHeaderCount++
			}
			if row.IsSeparator {
				separatorCount++
			}
		}

		if separatorCount == 0 {
			t.Error("non-All perspective with two projects: expected at least one IsSeparator row")
		}
		if sectionHeaderCount != 0 {
			t.Errorf("non-All perspective: expected zero IsSectionHeader rows, got %d", sectionHeaderCount)
		}
	})
}

func TestCrossProjectGroupingByPerspective(t *testing.T) {
	task1 := makeTask("g1", "Task 1", "")
	task1.Status = models.StatusInProgress
	proj1 := &models.Project{ID: "grp-p1", Name: "P1", Tasks: []*models.Task{task1}}
	proj1.BuildTaskTree()

	task2 := makeTask("g2", "Task 2", "")
	task2.Status = models.StatusInProgress
	proj2 := &models.Project{ID: "grp-p2", Name: "P2", Tasks: []*models.Task{task2}}
	proj2.BuildTaskTree()

	newModel := func(perspective int) *OutlinerModel {
		return &OutlinerModel{
			collapsedSections: make(map[string]bool),
			expandedNodes:     make(map[string]bool),
			MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
			activePerspective: perspective,
			loadedProjects:    []*models.Project{proj1, proj2},
		}
	}

	t.Run("All perspective produces section-grouped rows", func(t *testing.T) {
		m := newModel(PerspectiveAll)
		m.crossProjectRebuildRows()

		var sectionHeaders, separators int
		for _, row := range m.rows {
			if row.IsSectionHeader {
				sectionHeaders++
			}
			if row.IsSeparator {
				separators++
			}
		}
		if sectionHeaders == 0 {
			t.Error("expected IsSectionHeader rows for All perspective, got none")
		}
		if separators != 0 {
			t.Errorf("expected no IsSeparator rows for All perspective, got %d", separators)
		}
	})

	t.Run("non-All perspective produces project-grouped rows with IsSeparator", func(t *testing.T) {
		m := newModel(PerspectiveAll + 1)
		m.crossProjectRebuildRows()

		var sectionHeaders, separators int
		for _, row := range m.rows {
			if row.IsSectionHeader {
				sectionHeaders++
			}
			if row.IsSeparator {
				separators++
			}
		}
		if separators == 0 {
			t.Error("expected IsSeparator rows between project groups for non-All perspective, got none")
		}
		if sectionHeaders != 0 {
			t.Errorf("expected no IsSectionHeader rows for non-All perspective, got %d", sectionHeaders)
		}
	})
}
