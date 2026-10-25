package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// Runtime configuration is split into three explicit persistence domains:
//
//   - WorkspaceCatalog -> <data>/.tt/workspace.json
//   - PlanningState    -> <data>/.tt/planning.json
//   - MachineState     -> ~/.docket/runtime/<workspace-id>/machine.json
//
// Each aggregate owns its defaults, validation, project-reference cleanup, and
// persistence lifecycle. RuntimeState composes the three loaded domains for
// long-lived consumers, while mutations commit only the owning domain (or an
// explicit checked multi-domain transaction).
//
// This file holds the catalog: the portable project record and the workspace
// catalog's shape, identity, and validation. Catalog project mutation lives in
// domain_catalog_projects.go, and the other two aggregates in
// domain_planning.go and domain_machine.go.

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
	// Sync is present exactly when the workspace is enrolled in device sync,
	// and then the catalog is at schema 22 or later; see validateSyncEnrollment.
	Sync *CatalogSync `json:"sync,omitempty"`
}

// SyncExchangeFormat is the exchange-folder layout an enrolled workspace
// publishes through.
const SyncExchangeFormat = "tt-sync/v1"

// CatalogSync is the portable record that a workspace is enrolled in device
// sync. It is written once, by enrollment, and every device that holds the
// workspace carries it unchanged: where each device keeps its exchange folder
// is that device's own business and never synced.
type CatalogSync struct {
	// EnrolledAt is when the workspace was enrolled.
	EnrolledAt time.Time `json:"enrolled_at"`
	// Exchange is the exchange-folder layout, SyncExchangeFormat.
	Exchange string `json:"exchange"`
}

// Enrolled reports whether the catalog belongs to a workspace enrolled in
// device sync.
func (w *WorkspaceCatalog) Enrolled() bool { return w != nil && w.Sync != nil }

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
	return w.validate(false, SupportedConfigVersion)
}

// validateForLoad is the single seam every catalog-loading path validates
// through. It permits invalid project color slots so callers can repair them
// with EnsureProjectColorSlots; save paths continue to use Validate, which
// rejects them.
func (w *WorkspaceCatalog) validateForLoad() error {
	return w.validate(true, SupportedConfigVersion)
}

// validateAtSchema measures a catalog against the schema a migration is
// activating rather than the schema this build loads, because an activation
// publishes its target schema before that target becomes the loadable one.
// Every ordinary save and load measures against SupportedConfigVersion.
func (w *WorkspaceCatalog) validateAtSchema(supported int) error {
	return w.validate(false, supported)
}

func (w *WorkspaceCatalog) validate(allowRepairableColorSlots bool, supported int) error {
	if w.SchemaVersion <= 0 {
		return fmt.Errorf("workspace catalog: schema version must be positive, got %d", w.SchemaVersion)
	}
	if w.SchemaVersion > supported {
		return fmt.Errorf(
			"workspace catalog: schema version %d is newer than supported version %d; upgrade Docket",
			w.SchemaVersion,
			supported,
		)
	}
	if err := w.validateSyncEnrollment(); err != nil {
		return err
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

// validateSyncEnrollment ties the sync block to a schema that every Docket older
// than sync refuses. Schema 22 was reserved for enrolled catalogs, so a catalog
// still at it must carry the block; from schema 23 the block alone marks
// enrollment, and a catalog older than 22 carries none. No Docket that predates
// sync can therefore be handed a catalog it would read as an ordinary
// workspace.
func (w *WorkspaceCatalog) validateSyncEnrollment() error {
	switch {
	case w.SchemaVersion == V22ConfigSchemaVersion && w.Sync == nil:
		return fmt.Errorf("workspace catalog: schema version %d is reserved for workspaces enrolled in sync, and this catalog has no sync block", w.SchemaVersion)
	case w.Sync == nil:
		return nil
	case w.SchemaVersion < V22ConfigSchemaVersion:
		return fmt.Errorf("workspace catalog: a sync block requires schema version %d or later, got %d", V22ConfigSchemaVersion, w.SchemaVersion)
	case w.Sync.Exchange != SyncExchangeFormat:
		return fmt.Errorf("workspace catalog: sync exchange %q is not supported; want %q", w.Sync.Exchange, SyncExchangeFormat)
	case w.Sync.EnrolledAt.IsZero():
		return fmt.Errorf("workspace catalog: sync enrollment has no enrolled_at instant")
	}
	return nil
}
