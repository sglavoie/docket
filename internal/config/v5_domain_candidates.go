package config

import (
	"fmt"
	"sort"
	"time"
)

const V5ConfigSchemaVersion = 5

// V5WorkspaceCatalogCandidate is the lean catalog shape produced before the
// live runtime catalog cuts over to schema v5.
type V5WorkspaceCatalogCandidate struct {
	SchemaVersion        int                  `json:"schema_version"`
	WorkspaceID          string               `json:"workspace_id"`
	Projects             []V5ProjectCandidate `json:"projects"`
	SystemInboxProjectID string               `json:"system_inbox_project_id"`
}

// V5ProjectCandidate contains only the durable project fields retained by the
// simplified product.
type V5ProjectCandidate struct {
	ID          string     `json:"id"`
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Context     string     `json:"context"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	ColorSlot   int        `json:"color_slot"`
}

// V5PlanningCandidate persists Today membership and Next ordering only.
type V5PlanningCandidate struct {
	Today     []V5PlanningTaskRef `json:"today,omitempty"`
	NextOrder []V5PlanningTaskRef `json:"next_order,omitempty"`
}

type V5PlanningTaskRef = ProjectTaskLocator

// V5MachineCandidate retains only machine-local custom contexts and the
// selected built-in theme.
type V5MachineCandidate struct {
	CustomContexts map[string][]string `json:"custom_contexts,omitempty"`
	Theme          string              `json:"theme"`
}

type V5DomainCandidates struct {
	Catalog  V5WorkspaceCatalogCandidate
	Planning V5PlanningCandidate
	Machine  V5MachineCandidate
}

// IsSystemInboxProject reports whether id is the catalog-owned Inbox. The
// durable ID is the sole protection boundary; matching a display name or slug
// never grants or bypasses protection.
func (catalog V5WorkspaceCatalogCandidate) IsSystemInboxProject(id string) bool {
	return id != "" && catalog.SystemInboxProjectID == id
}

// EnsureProjectColorSlots repairs missing or invalid v5 color slots
// deterministically without recoloring valid assignments. All v5 projects are
// user-visible and therefore eligible for one automatic durable slot.
func (catalog *V5WorkspaceCatalogCandidate) EnsureProjectColorSlots() bool {
	if catalog == nil {
		return false
	}

	var usage [ProjectColorSlotCount + 1]int
	pending := make([]int, 0)
	for index := range catalog.Projects {
		slot := catalog.Projects[index].ColorSlot
		if validProjectColorSlot(slot) {
			usage[slot]++
		} else {
			pending = append(pending, index)
		}
	}
	sort.SliceStable(pending, func(i, j int) bool {
		left, right := catalog.Projects[pending[i]], catalog.Projects[pending[j]]
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})
	for _, index := range pending {
		slot := leastUsedProjectColorSlot(usage)
		catalog.Projects[index].ColorSlot = slot
		usage[slot]++
	}
	return len(pending) > 0
}

// UpsertProject applies one create/edit/pin/archive/restore snapshot to the lean
// v5 catalog. Existing projects retain their durable color slot; new and
// restored projects receive an automatic balanced slot. A system Inbox update
// may change ordinary metadata or pin state, but cannot rename or archive it.
// Rejected mutations leave the candidate unchanged.
func (catalog *V5WorkspaceCatalogCandidate) UpsertProject(project V5ProjectCandidate) error {
	if catalog == nil {
		return fmt.Errorf("v5 catalog is required")
	}
	if catalog.IsSystemInboxProject(project.ID) && (project.Archived || project.ArchivedAt != nil) {
		return fmt.Errorf("%w: project %q cannot be archived", ErrProtectedSystemUnfiled, project.ID)
	}
	if err := validateV5ProjectCandidate(project, true); err != nil {
		return err
	}

	next := *catalog
	next.Projects = append([]V5ProjectCandidate(nil), catalog.Projects...)
	next.EnsureProjectColorSlots()

	existingIndex := -1
	for index, existing := range next.Projects {
		if existing.ID == project.ID {
			existingIndex = index
			continue
		}
		if existing.Slug == project.Slug {
			return fmt.Errorf(
				"v5 catalog project slug %q is already owned by project %q",
				project.Slug,
				existing.ID,
			)
		}
	}
	if next.IsSystemInboxProject(project.ID) {
		if existingIndex >= 0 {
			existing := next.Projects[existingIndex]
			if existing.Name != project.Name || existing.Slug != project.Slug {
				return fmt.Errorf("%w: project %q cannot be renamed", ErrProtectedSystemUnfiled, project.ID)
			}
		}
	}

	if existingIndex >= 0 {
		project.ColorSlot = next.Projects[existingIndex].ColorSlot
		next.Projects[existingIndex] = project
	} else {
		var usage [ProjectColorSlotCount + 1]int
		for _, existing := range next.Projects {
			if validProjectColorSlot(existing.ColorSlot) {
				usage[existing.ColorSlot]++
			}
		}
		if !validProjectColorSlot(project.ColorSlot) ||
			(usage[project.ColorSlot] > 0 && !allProjectColorSlotsUsed(usage)) {
			project.ColorSlot = leastUsedProjectColorSlot(usage)
		}
		next.Projects = append(next.Projects, project)
	}

	*catalog = next
	return nil
}

// RemoveProject removes one lean v5 project by ID. The protected system Inbox
// can never be removed, and a rejection leaves the candidate unchanged.
func (catalog *V5WorkspaceCatalogCandidate) RemoveProject(projectID string) error {
	if catalog == nil {
		return fmt.Errorf("v5 catalog is required")
	}
	if catalog.IsSystemInboxProject(projectID) {
		return fmt.Errorf("%w: project %q cannot be removed", ErrProtectedSystemUnfiled, projectID)
	}
	kept := make([]V5ProjectCandidate, 0, len(catalog.Projects))
	for _, project := range catalog.Projects {
		if project.ID != projectID {
			kept = append(kept, project)
		}
	}
	catalog.Projects = kept
	return nil
}
