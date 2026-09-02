package config

import (
	"fmt"
	"slices"
	"sort"

	"github.com/sglavoie/tt/internal/models"
)

// This file owns catalog project membership: the durable color-slot assignment
// and the add, replace, repair, and remove paths. The catalog's own shape and
// validation live in domain.go.

// EnsureProjectColorSlots assigns durable color slots to every eligible
// project that is missing one or carries an out-of-range value. Valid existing
// assignments are never changed, including duplicates. Assignment order is
// deterministic and independent of the catalog slice order.
func (w *WorkspaceCatalog) EnsureProjectColorSlots() bool {
	if w == nil {
		return false
	}

	usage := projectColorSlotUsage(w.Projects, -1)
	pending := make([]int, 0)
	changed := false
	for i := range w.Projects {
		project := &w.Projects[i]
		if projectColorEligible(*project) {
			if !validProjectColorSlot(project.ColorSlot) {
				pending = append(pending, i)
			}
			continue
		}
		if project.ColorSlot < 0 || project.ColorSlot > ProjectColorSlotCount {
			project.ColorSlot = 0
			changed = true
		}
	}

	sort.SliceStable(pending, func(i, j int) bool {
		left := w.Projects[pending[i]]
		right := w.Projects[pending[j]]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})
	for _, index := range pending {
		slot := leastUsedProjectColorSlot(usage)
		w.Projects[index].ColorSlot = slot
		usage[slot]++
		changed = true
	}
	return changed
}

func projectColorEligible(project CatalogProject) bool {
	return true
}

func validProjectColorSlot(slot int) bool {
	return slot >= 1 && slot <= ProjectColorSlotCount
}

func projectColorSlotUsage(projects []CatalogProject, skipIndex int) [ProjectColorSlotCount + 1]int {
	var usage [ProjectColorSlotCount + 1]int
	for i, project := range projects {
		if i == skipIndex || !projectColorEligible(project) || !validProjectColorSlot(project.ColorSlot) {
			continue
		}
		usage[project.ColorSlot]++
	}
	return usage
}

func leastUsedProjectColorSlot(usage [ProjectColorSlotCount + 1]int) int {
	selected := 1
	for slot := 2; slot <= ProjectColorSlotCount; slot++ {
		if usage[slot] < usage[selected] {
			selected = slot
		}
	}
	return selected
}

func allProjectColorSlotsUsed(usage [ProjectColorSlotCount + 1]int) bool {
	for slot := 1; slot <= ProjectColorSlotCount; slot++ {
		if usage[slot] == 0 {
			return false
		}
	}
	return true
}

// RemoveProject drops a cataloged project while preserving the protected
// system Unfiled invariant. Matching remains compatible with slug
// references, but protection is decided from the matched project's durable ID.
// A rejected removal leaves the catalog unchanged.
func (w *WorkspaceCatalog) RemoveProject(id, slug string) (bool, error) {
	if w == nil {
		return false, nil
	}
	if w.IsSystemUnfiledProject(id) {
		return false, fmt.Errorf(
			"%w: project %q cannot be removed",
			ErrProtectedSystemUnfiled,
			id,
		)
	}
	if id == "" && slug != "" {
		for _, project := range w.Projects {
			if project.Slug == slug && w.IsSystemUnfiledProject(project.ID) {
				return false, fmt.Errorf(
					"%w: project %q cannot be removed",
					ErrProtectedSystemUnfiled,
					project.ID,
				)
			}
		}
	}

	changed := false
	kept := w.Projects[:0]
	for _, p := range w.Projects {
		if projectMatches(p.ID, p.Slug, id, slug) {
			changed = true
			continue
		}
		kept = append(kept, p)
	}
	w.Projects = kept
	return changed, nil
}

// UpsertProject adds a project to the catalog or, when one with the same ID
// already exists, replaces its snapshot in place. The project is stored in its
// portable catalog form (absolute Path and computed fields dropped), so a TUI
// that mutates catalog-owned flags on a rehydrated models.Project (Archive,
// Pin, rename) can write the change back through a checked catalog commit.
//
// The mutation is prepared on a defensive copy. Invalid updates therefore
// cannot partially repair color slots or alter the caller's catalog. Existing
// system Unfiled identity and active lifecycle state are immutable here;
// bootstrap code must use RepairSystemUnfiledProject for its explicit repair.
func (w *WorkspaceCatalog) UpsertProject(p *models.Project) error {
	return w.upsertProject(p, false)
}

// RepairSystemUnfiledProject installs or normalizes the catalog-owned Unfiled
// during workspace bootstrap. It is intentionally separate from ordinary
// project updates so user-facing edit/archive operations cannot use legacy
// repair authority. The repaired project must remain active.
func (w *WorkspaceCatalog) RepairSystemUnfiledProject(p *models.Project) error {
	if w == nil {
		return fmt.Errorf("workspace catalog is required")
	}
	if p == nil {
		return fmt.Errorf("system Unfiled project is required")
	}
	protectedID := w.protectedProjectID()
	if protectedID != "" && protectedID != p.ID {
		return fmt.Errorf(
			"%w: cannot replace project %q with %q",
			ErrProtectedSystemUnfiled,
			protectedID,
			p.ID,
		)
	}
	if p.Archived || p.ArchivedAt != nil {
		return fmt.Errorf("%w: project %q cannot be archived", ErrProtectedSystemUnfiled, p.ID)
	}
	return w.upsertProject(p, true)
}

func (w *WorkspaceCatalog) upsertProject(p *models.Project, repairSystemUnfiled bool) error {
	if w == nil {
		return fmt.Errorf("workspace catalog is required")
	}
	if p == nil {
		return fmt.Errorf("project is required")
	}

	next := *w
	next.Projects = append([]CatalogProject(nil), w.Projects...)
	next.EnsureProjectColorSlots()
	cp := catalogProjectFromModel(p)
	tags, err := models.NormalizeProjectTags(cp.Tags)
	if err != nil {
		return fmt.Errorf("workspace catalog: project %q has invalid tags: %w", cp.ID, err)
	}
	cp.Tags = tags
	if repairSystemUnfiled {
		// Bootstrap repair runs before SystemUnfiledProjectID is bound, so the
		// protected-project branch below cannot yet recognize this project.
		cp.Tags = slices.DeleteFunc(cp.Tags, func(tag string) bool { return tag == models.ParkedProjectTag })
	}

	existingIndex := -1
	for i, existing := range next.Projects {
		if existing.ID == cp.ID {
			existingIndex = i
			continue
		}
		if existing.Slug == cp.Slug {
			return fmt.Errorf(
				"workspace catalog: project slug %q is already owned by project %q",
				cp.Slug,
				existing.ID,
			)
		}
	}

	if next.IsSystemUnfiledProject(cp.ID) {
		if cp.Archived || cp.ArchivedAt != nil {
			return fmt.Errorf("%w: project %q cannot be archived", ErrProtectedSystemUnfiled, cp.ID)
		}
		if models.ProjectTagsParked(cp.Tags) {
			return fmt.Errorf(
				"%w: project %q cannot be tagged %q",
				ErrProtectedSystemUnfiled,
				cp.ID,
				models.ParkedProjectTag,
			)
		}
		if existingIndex >= 0 && !repairSystemUnfiled {
			existing := next.Projects[existingIndex]
			if existing.Name != cp.Name || existing.Slug != cp.Slug {
				return fmt.Errorf("%w: project %q cannot be renamed", ErrProtectedSystemUnfiled, cp.ID)
			}
		}
	}

	if existingIndex >= 0 {
		storedSlot := next.Projects[existingIndex].ColorSlot
		if !validProjectColorSlot(storedSlot) && projectColorEligible(cp) {
			usage := projectColorSlotUsage(next.Projects, existingIndex)
			storedSlot = leastUsedProjectColorSlot(usage)
		}
		if !validProjectColorSlot(storedSlot) {
			storedSlot = 0
		}
		cp.ColorSlot = storedSlot
		next.Projects[existingIndex] = cp
	} else {
		if projectColorEligible(cp) {
			usage := projectColorSlotUsage(next.Projects, -1)
			if !validProjectColorSlot(cp.ColorSlot) || (usage[cp.ColorSlot] > 0 && !allProjectColorSlotsUsed(usage)) {
				cp.ColorSlot = leastUsedProjectColorSlot(usage)
			}
		} else if !validProjectColorSlot(cp.ColorSlot) {
			cp.ColorSlot = 0
		}
		next.Projects = append(next.Projects, cp)
	}

	if repairSystemUnfiled {
		next.SystemUnfiledProjectID = cp.ID
	}

	*w = next
	p.ColorSlot = cp.ColorSlot
	p.Tags = append([]string(nil), cp.Tags...)
	return nil
}
