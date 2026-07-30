package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

// Runtime configuration is split into three explicit persistence domains:
//
//   - WorkspaceCatalog -> <data>/.tt/workspace.json
//   - PlanningState    -> <data>/.tt/planning.json
//   - MachineState     -> ~/.tt/runtime/<workspace-id>/machine.json
//
// Each aggregate owns its defaults, validation, project-reference cleanup, and
// persistence lifecycle. RuntimeState composes the three loaded domains for
// long-lived consumers, while mutations commit only the owning domain (or an
// explicit checked multi-domain transaction).

// CatalogProject is the persisted, portable description of a project inside the
// WorkspaceCatalog. It deliberately omits the absolute on-disk Path that the
// legacy models.Project carries: a project's location is derived from the data
// directory and its safe slug (see CatalogPath), never stored, so a workspace
// can be moved between machines or storage backends without rewriting paths.
type CatalogProject struct {
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
	ColorSlot   int        `json:"color_slot,omitempty"`
}

const (
	// ProjectColorSlotCount is the fixed number of durable project identity
	// colors. Slot zero represents a legacy or otherwise unassigned project.
	ProjectColorSlotCount = 48

	// SystemInboxName and SystemInboxSlug are the user-facing identity of the
	// workspace-owned capture project. The catalog reference, rather than the
	// display name or slug, is the durable identity used by callers.
	SystemInboxName = "Inbox"
	SystemInboxSlug = "inbox"

	// LegacyActionInboxSlug identifies the hidden predecessor migrated by the
	// TUI when it first opens a workspace created by an older release.
	LegacyActionInboxSlug = "action-inbox"
)

var (
	// ErrProtectedSystemInbox marks a project-catalog mutation that would
	// rename, archive, remove, or otherwise invalidate the project identified
	// by WorkspaceCatalog.SystemInboxProjectID. The ID is the protection
	// boundary; display names and slugs never grant protected status.
	ErrProtectedSystemInbox = errors.New("config: protected system Inbox")
)

// CatalogPath derives a project's data directory from the workspace data
// directory and the project slug. Absolute paths are never persisted; this is
// the single derivation point for a catalog project's location.
func (p CatalogProject) CatalogPath(dataDir string) string {
	return filepath.Join(dataDir, "projects", p.Slug)
}

// catalogProjectFromModel snapshots a models.Project into its portable catalog
// form, dropping the absolute Path and all computed (unpersisted) fields.
func catalogProjectFromModel(p *models.Project) CatalogProject {
	if p == nil {
		return CatalogProject{}
	}
	return CatalogProject{
		ID:          p.ID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Context:     p.Context,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		Archived:    p.Archived,
		ArchivedAt:  p.ArchivedAt,
		Pinned:      p.Pinned,
		PinnedAt:    p.PinnedAt,
		ColorSlot:   p.ColorSlot,
	}
}

// ToModel rehydrates a catalog project into a models.Project, deriving Path
// from the supplied data directory. Computed fields (Tasks, Sections, etc.)
// are left zero for storage to populate on load.
func (p CatalogProject) ToModel(dataDir string) *models.Project {
	return &models.Project{
		ID:          p.ID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Context:     p.Context,
		Path:        p.CatalogPath(dataDir),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		Archived:    p.Archived,
		ArchivedAt:  p.ArchivedAt,
		Pinned:      p.Pinned,
		PinnedAt:    p.PinnedAt,
		ColorSlot:   p.ColorSlot,
	}
}

// WorkspaceCatalog owns the workspace identity and the project catalog. It maps
// to <data>/.tt/workspace.json.
type WorkspaceCatalog struct {
	SchemaVersion        int              `json:"schema_version"`
	WorkspaceID          string           `json:"workspace_id,omitempty"`
	Projects             []CatalogProject `json:"projects"`
	SystemInboxProjectID string           `json:"system_inbox_project_id,omitempty"`
}

// IsSystemInboxProject reports whether id is the catalog-owned Inbox. Names and
// slugs are deliberately ignored so identity stays stable across presentation
// changes and legacy migrations.
func (w *WorkspaceCatalog) IsSystemInboxProject(id string) bool {
	return w != nil && id != "" && w.SystemInboxProjectID == id
}

// NewWorkspaceCatalog returns a catalog seeded with defaults.
func NewWorkspaceCatalog() *WorkspaceCatalog {
	return &WorkspaceCatalog{
		SchemaVersion: CurrentConfigVersion,
		Projects:      []CatalogProject{},
	}
}

// Validate reports structural problems with the catalog: a project must have a
// non-empty ID and canonical safe slug, IDs and slugs must be unique, and the
// system inbox reference must point at a cataloged project when set.
func (w *WorkspaceCatalog) Validate() error {
	return w.validate(false)
}

// validateForLoad permits invalid project color slots so callers can repair
// them with EnsureProjectColorSlots after the catalog's structural fields have
// been validated. Save paths continue to use Validate and reject invalid slots.
func (w *WorkspaceCatalog) validateForLoad() error {
	return w.validate(true)
}

func (w *WorkspaceCatalog) validate(allowRepairableColorSlots bool) error {
	if w.SchemaVersion <= 0 {
		return fmt.Errorf("workspace catalog: schema version must be positive, got %d", w.SchemaVersion)
	}
	if w.SchemaVersion > CurrentConfigVersion {
		return fmt.Errorf(
			"workspace catalog: schema version %d is newer than supported version %d; upgrade tt",
			w.SchemaVersion,
			CurrentConfigVersion,
		)
	}
	seenID := make(map[string]bool, len(w.Projects))
	seenSlug := make(map[string]bool, len(w.Projects))
	for i, p := range w.Projects {
		if p.ID == "" {
			return fmt.Errorf("workspace catalog: project %d has empty id", i)
		}
		if err := models.ValidateProjectSlug(p.Slug); err != nil {
			return fmt.Errorf("workspace catalog: project %q has invalid slug: %w", p.ID, err)
		}
		if seenID[p.ID] {
			return fmt.Errorf("workspace catalog: duplicate project id %q", p.ID)
		}
		if seenSlug[p.Slug] {
			return fmt.Errorf("workspace catalog: duplicate project slug %q", p.Slug)
		}
		if !allowRepairableColorSlots && (p.ColorSlot < 0 || p.ColorSlot > ProjectColorSlotCount) {
			return fmt.Errorf("workspace catalog: project %q has invalid color slot %d", p.ID, p.ColorSlot)
		}
		seenID[p.ID] = true
		seenSlug[p.Slug] = true
	}
	if w.SystemInboxProjectID != "" {
		if !seenID[w.SystemInboxProjectID] {
			return fmt.Errorf("workspace catalog: system inbox references unknown project %q", w.SystemInboxProjectID)
		}
		for _, project := range w.Projects {
			if project.ID == w.SystemInboxProjectID && (project.Archived || project.ArchivedAt != nil) {
				return fmt.Errorf(
					"%w: project %q cannot be archived",
					ErrProtectedSystemInbox,
					w.SystemInboxProjectID,
				)
			}
		}
	}
	return nil
}

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
// system Inbox invariant. Matching remains compatible with legacy slug
// references, but protection is decided from the matched project's durable ID.
// A rejected removal leaves the catalog unchanged.
func (w *WorkspaceCatalog) RemoveProject(id, slug string) (bool, error) {
	if w == nil {
		return false, nil
	}
	if w.IsSystemInboxProject(id) {
		return false, fmt.Errorf(
			"%w: project %q cannot be removed",
			ErrProtectedSystemInbox,
			id,
		)
	}
	if id == "" && slug != "" {
		for _, project := range w.Projects {
			if project.Slug == slug && w.IsSystemInboxProject(project.ID) {
				return false, fmt.Errorf(
					"%w: project %q cannot be removed",
					ErrProtectedSystemInbox,
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

// RemoveProjectReferences is the compatibility form used by reference-cleanup
// callers. It deliberately fails closed for the protected system Inbox. New
// lifecycle adapters should call RemoveProject so they can surface the typed
// rejection.
func (w *WorkspaceCatalog) RemoveProjectReferences(id, slug string) bool {
	changed, _ := w.RemoveProject(id, slug)
	return changed
}

// UpsertProject adds a project to the catalog or, when one with the same ID
// already exists, replaces its snapshot in place. The project is stored in its
// portable catalog form (absolute Path and computed fields dropped), so a TUI
// that mutates catalog-owned flags on a rehydrated models.Project (Archive,
// Pin, rename) can write the change back through a checked catalog commit.
//
// The mutation is prepared on a defensive copy. Invalid updates therefore
// cannot partially repair color slots or alter the caller's catalog. Existing
// system Inbox identity and active lifecycle state are immutable here; startup
// compatibility code must use RepairSystemInboxProject for its one explicit
// legacy normalization.
func (w *WorkspaceCatalog) UpsertProject(p *models.Project) error {
	return w.upsertProject(p, false)
}

// RepairSystemInboxProject installs or normalizes the catalog-owned Inbox
// during workspace bootstrap. It is intentionally separate from ordinary
// project updates so user-facing edit/archive operations cannot use legacy
// repair authority. The repaired project must remain active.
func (w *WorkspaceCatalog) RepairSystemInboxProject(p *models.Project) error {
	if w == nil {
		return fmt.Errorf("workspace catalog is required")
	}
	if p == nil {
		return fmt.Errorf("system Inbox project is required")
	}
	if w.SystemInboxProjectID != "" && w.SystemInboxProjectID != p.ID {
		return fmt.Errorf(
			"%w: cannot replace project %q with %q",
			ErrProtectedSystemInbox,
			w.SystemInboxProjectID,
			p.ID,
		)
	}
	if p.Archived || p.ArchivedAt != nil {
		return fmt.Errorf("%w: project %q cannot be archived", ErrProtectedSystemInbox, p.ID)
	}
	return w.upsertProject(p, true)
}

func (w *WorkspaceCatalog) upsertProject(p *models.Project, repairSystemInbox bool) error {
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

	if next.IsSystemInboxProject(cp.ID) {
		if cp.Archived || cp.ArchivedAt != nil {
			return fmt.Errorf("%w: project %q cannot be archived", ErrProtectedSystemInbox, cp.ID)
		}
		if existingIndex >= 0 && !repairSystemInbox {
			existing := next.Projects[existingIndex]
			if existing.Name != cp.Name || existing.Slug != cp.Slug {
				return fmt.Errorf("%w: project %q cannot be renamed", ErrProtectedSystemInbox, cp.ID)
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

	if repairSystemInbox {
		next.SystemInboxProjectID = cp.ID
	}

	*w = next
	p.ColorSlot = cp.ColorSlot
	return nil
}

// PlanningState owns the two durable planning collections retained by v5:
// project-qualified Today membership/order and the preferred order of StateNext
// tasks. StateNext remains authoritative for Next membership.
type PlanningState struct {
	Today     []ProjectTaskLocator `json:"today,omitempty"`
	NextOrder []ProjectTaskLocator `json:"next_order,omitempty"`
}

// NewPlanningState returns an empty planning state.
func NewPlanningState() *PlanningState {
	return &PlanningState{}
}

// Validate reports problems with the project-qualified planning collections.
func (p *PlanningState) Validate() error {
	seenToday := make(map[ProjectTaskLocator]bool, len(p.Today))
	for i, item := range p.Today {
		if !item.Valid() {
			return fmt.Errorf("planning: Today item %d has incomplete identity", i)
		}
		if seenToday[item] {
			return fmt.Errorf("planning: Today repeats task %q in project %q", item.TaskID, item.ProjectID)
		}
		seenToday[item] = true
	}
	seenNext := make(map[ProjectTaskLocator]bool, len(p.NextOrder))
	for i, item := range p.NextOrder {
		if !item.Valid() {
			return fmt.Errorf("planning: Next order item %d has incomplete identity", i)
		}
		if seenNext[item] {
			return fmt.Errorf("planning: Next order repeats task %q in project %q", item.TaskID, item.ProjectID)
		}
		seenNext[item] = true
	}
	return nil
}

// RemoveProjectReferences drops Today and Next-order entries that point at the
// removed project. It returns true when anything changed.
func (p *PlanningState) RemoveProjectReferences(id, slug string) bool {
	var todayChanged, nextChanged bool
	p.Today, todayChanged = filterProjectRefSlice(p.Today, func(item ProjectTaskLocator) string {
		return item.ProjectID
	}, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	p.NextOrder, nextChanged = filterProjectRefSlice(p.NextOrder, func(item ProjectTaskLocator) string {
		return item.ProjectID
	}, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	return todayChanged || nextChanged
}

// MachineState contains only the machine-local values retained by v5.
type MachineState struct {
	CustomContexts map[string][]string `json:"custom_contexts,omitempty"`
	Theme          string              `json:"theme"`
}

// NewMachineState returns the default built-in theme selection.
func NewMachineState() *MachineState {
	return &MachineState{Theme: "dark"}
}

// Validate reports problems with machine-local preferences.
func (m *MachineState) Validate() error {
	if m.Theme != "" && m.Theme != "dark" && m.Theme != "light" {
		return fmt.Errorf("machine: theme %q must be dark or light", m.Theme)
	}
	return nil
}

// RemoveProjectReferences removes a project from saved custom contexts.
func (m *MachineState) RemoveProjectReferences(id, slug string) bool {
	return filterProjectIDMap(m.CustomContexts, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
}

// refMatches reports whether a stored project reference targets the identified
// project, matching on either the ID or (when non-empty) the slug.
func refMatches(ref, id, slug string) bool {
	if ref == "" {
		return false
	}
	return ref == id || (slug != "" && ref == slug)
}

// projectMatches reports whether a catalog project (identified by its own id and
// slug) is the one being removed.
func projectMatches(projID, projSlug, id, slug string) bool {
	return refMatches(projID, id, slug) || refMatches(projSlug, id, slug)
}
