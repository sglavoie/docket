package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
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
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Archived    bool       `json:"archived"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	Pinned      bool       `json:"pinned"`
	PinnedAt    *time.Time `json:"pinned_at,omitempty"`
	ColorSlot   int        `json:"color_slot,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

const (
	// ProjectColorSlotCount is the fixed number of durable project identity
	// colors. Slot zero represents a legacy or otherwise unassigned project.
	ProjectColorSlotCount = 48

	// SystemUnfiledName and SystemUnfiledSlug are the user-facing identity of the
	// workspace-owned capture project. The catalog reference, rather than the
	// display name or slug, is the durable identity used by callers.
	SystemUnfiledName = "Unfiled"
	SystemUnfiledSlug = "unfiled"

	// LegacyActionInboxSlug identifies the hidden predecessor migrated by the
	// TUI when it first opens a workspace created by an older release.
	LegacyActionInboxSlug = "action-inbox"
)

var (
	// ErrProtectedSystemUnfiled marks a project-catalog mutation that would
	// rename, archive, remove, or otherwise invalidate the project identified
	// by WorkspaceCatalog.SystemUnfiledProjectID. The ID is the protection
	// boundary; display names and slugs never grant protected status.
	ErrProtectedSystemUnfiled = errors.New("config: protected system Unfiled")
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
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		Archived:    p.Archived,
		ArchivedAt:  p.ArchivedAt,
		Pinned:      p.Pinned,
		PinnedAt:    p.PinnedAt,
		ColorSlot:   p.ColorSlot,
		Tags:        append([]string(nil), p.Tags...),
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
		Path:        p.CatalogPath(dataDir),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
		Archived:    p.Archived,
		ArchivedAt:  p.ArchivedAt,
		Pinned:      p.Pinned,
		PinnedAt:    p.PinnedAt,
		ColorSlot:   p.ColorSlot,
		Tags:        append([]string(nil), p.Tags...),
	}
}

// WorkspaceCatalog owns the workspace identity and the project catalog. It maps
// to <data>/.tt/workspace.json.
type WorkspaceCatalog struct {
	SchemaVersion          int              `json:"schema_version"`
	WorkspaceID            string           `json:"workspace_id,omitempty"`
	Projects               []CatalogProject `json:"projects"`
	SystemUnfiledProjectID string           `json:"system_unfiled_project_id"`
}

func (w *WorkspaceCatalog) protectedProjectID() string {
	if w == nil {
		return ""
	}
	return w.SystemUnfiledProjectID
}

// IsSystemUnfiledProject reports whether id is the catalog-owned Unfiled. Names and
// slugs are deliberately ignored so identity stays stable across presentation
// changes and legacy migrations.
func (w *WorkspaceCatalog) IsSystemUnfiledProject(id string) bool {
	return w != nil && id != "" && w.protectedProjectID() == id
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
// protected Unfiled reference must point at a cataloged project when set.
func (w *WorkspaceCatalog) Validate() error {
	return w.validate(false, CurrentConfigVersion)
}

// validateForLoad is the single seam every catalog-loading path validates
// through. It permits invalid project color slots so callers can repair them
// with EnsureProjectColorSlots; save paths continue to use Validate, which
// rejects them.
func (w *WorkspaceCatalog) validateForLoad() error {
	return w.validate(true, CurrentConfigVersion)
}

// validateAtSchema measures a catalog against the schema a migration is
// activating rather than the schema this build loads, because an activation
// publishes its target schema before that target becomes the loadable one.
// Every ordinary save and load keeps measuring against CurrentConfigVersion.
func (w *WorkspaceCatalog) validateAtSchema(supported int) error {
	return w.validate(false, supported)
}

func (w *WorkspaceCatalog) validate(allowRepairableColorSlots bool, supported int) error {
	if w.SchemaVersion <= 0 {
		return fmt.Errorf("workspace catalog: schema version must be positive, got %d", w.SchemaVersion)
	}
	if w.SchemaVersion > supported {
		return fmt.Errorf(
			"workspace catalog: schema version %d is newer than supported version %d; upgrade tt",
			w.SchemaVersion,
			supported,
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
		normalizedTags, err := models.NormalizeProjectTags(p.Tags)
		if err != nil {
			return fmt.Errorf("workspace catalog: project %q has invalid tags: %w", p.ID, err)
		}
		if !slices.Equal(normalizedTags, p.Tags) {
			return fmt.Errorf(
				"workspace catalog: project %q tags %v are not canonical (canonical form %v)",
				p.ID, p.Tags, normalizedTags,
			)
		}
		seenID[p.ID] = true
		seenSlug[p.Slug] = true
	}
	protectedID := w.protectedProjectID()
	if protectedID != "" {
		if !seenID[protectedID] {
			return fmt.Errorf("workspace catalog: system Unfiled references unknown project %q", protectedID)
		}
		for _, project := range w.Projects {
			if project.ID != protectedID {
				continue
			}
			if project.Archived || project.ArchivedAt != nil {
				return fmt.Errorf(
					"%w: project %q cannot be archived",
					ErrProtectedSystemUnfiled,
					protectedID,
				)
			}
			if models.ProjectTagsParked(project.Tags) {
				return fmt.Errorf(
					"%w: project %q cannot be tagged %q",
					ErrProtectedSystemUnfiled,
					protectedID,
					models.ParkedProjectTag,
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

// PlanningState owns the two schema-6 durable ordered memberships.
type PlanningState struct {
	Today []ProjectTaskLocator `json:"today,omitempty"`
	Next  []ProjectTaskLocator `json:"next,omitempty"`
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
	seenNext := make(map[ProjectTaskLocator]bool, len(p.Next))
	for i, item := range p.Next {
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
	p.Next, nextChanged = filterProjectRefSlice(p.Next, func(item ProjectTaskLocator) string {
		return item.ProjectID
	}, func(pid string) bool {
		return !refMatches(pid, id, slug)
	})
	return todayChanged || nextChanged
}

// MachineState contains the only machine-local value retained by schema 6.
type MachineState struct {
	Theme string `json:"theme"`
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
