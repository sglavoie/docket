package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

// Versioned trash layout
//
// Each deleted project becomes an immutable trash entry keyed by the project's
// stable ID and the deletion time, never by the raw slug. This lets multiple
// deleted generations that share a slug coexist without one silently purging
// another:
//
//	<data>/.tt/trash/<projectID>__<deletedAtNano>/
//	    tombstone.json   -- identity, path, reason, config references
//	    data/            -- the moved project directory (tasks.md, .attachments, ...)
//
// Legacy pre-versioned trash (a bare <slug>/ directory with tasks.md at its
// root and no tombstone) is still listed and restorable via a best-effort
// fallback so a workspace deleted under the old layout is not stranded.
const (
	trashTombstoneSchemaVersion = 1
	trashTombstoneFile          = "tombstone.json"
	trashDataDirName            = "data"
)

// TrashTombstone records everything needed to faithfully restore a deleted
// project. One tombstone.json is written per versioned trash entry.
type TrashTombstone struct {
	SchemaVersion  int             `json:"schema_version"`
	EntryID        string          `json:"entry_id"`
	Project        *models.Project `json:"project"`
	OriginalPath   string          `json:"original_path"`
	DeletedAt      time.Time       `json:"deleted_at"`
	DeletionReason string          `json:"deletion_reason,omitempty"`
	// ConfigRefs is an opaque, caller-owned snapshot of the configuration
	// references that pointed at this project (focus queue, daily plan, view
	// state, etc.). Storage stores and returns it verbatim so it need not
	// depend on the config package.
	ConfigRefs json.RawMessage `json:"config_refs,omitempty"`
}

// TrashEntry is a summarized trash entry used for listing and selection.
type TrashEntry struct {
	EntryID   string
	ProjectID string
	Slug      string
	Name      string
	DeletedAt time.Time
	Reason    string
	// Legacy is true for pre-versioned entries that lack a tombstone.
	Legacy bool
}

// getTrashDir returns the path to the trash directory within the data directory.
func getTrashDir(dataDir string) string {
	return filepath.Join(dataDir, ".tt", "trash")
}

// MoveProjectToTrash moves a project's data directory into a new versioned trash
// entry keyed by the project's immutable ID and the deletion time, writing a
// tombstone that captures the project metadata, path identity, deletion reason,
// and the caller-supplied configuration-reference snapshot. It never overwrites
// or purges an existing entry, so multiple deleted generations that share a slug
// coexist. Returns the created entry ID.
func MoveProjectToTrash(project *models.Project, dataDir, reason string, configRefs json.RawMessage) (string, error) {
	if project == nil {
		return "", fmt.Errorf("project cannot be nil")
	}
	if project.ID == "" {
		return "", fmt.Errorf("project ID cannot be empty")
	}
	// The slug is used to derive the restore destination path, so guard it here
	// (before it ever reaches the filesystem) against traversal or separators.
	if err := SafeChildName(project.Slug); err != nil {
		return "", fmt.Errorf("invalid project slug %q: %w", project.Slug, err)
	}
	if err := SafeChildName(project.ID); err != nil {
		return "", fmt.Errorf("invalid project id %q: %w", project.ID, err)
	}

	deletedAt := time.Now()
	trashDir := getTrashDir(dataDir)
	if err := os.MkdirAll(trashDir, DirPermissions); err != nil {
		return "", fmt.Errorf("failed to create trash directory: %w", err)
	}

	entryID, entryPath, err := allocateTrashEntry(trashDir, project.ID, deletedAt)
	if err != nil {
		return "", err
	}
	if mkErr := os.MkdirAll(entryPath, DirPermissions); mkErr != nil {
		return "", fmt.Errorf("failed to create trash entry: %w", mkErr)
	}

	dataPath, err := SafeJoin(entryPath, trashDataDirName)
	if err != nil {
		_ = os.RemoveAll(entryPath)
		return "", err
	}
	if err := os.Rename(project.Path, dataPath); err != nil {
		_ = os.RemoveAll(entryPath)
		return "", fmt.Errorf("failed to move project to trash: %w", err)
	}

	tomb := &TrashTombstone{
		SchemaVersion:  trashTombstoneSchemaVersion,
		EntryID:        entryID,
		Project:        project,
		OriginalPath:   project.Path,
		DeletedAt:      deletedAt,
		DeletionReason: reason,
		ConfigRefs:     configRefs,
	}
	if err := writeTombstone(entryPath, tomb); err != nil {
		// Roll the data move back so a failed tombstone write doesn't strand data.
		_ = os.Rename(dataPath, project.Path)
		_ = os.RemoveAll(entryPath)
		return "", err
	}

	return entryID, nil
}

// allocateTrashEntry returns a unique, filesystem-safe entry ID and its path.
func allocateTrashEntry(trashDir, projectID string, deletedAt time.Time) (string, string, error) {
	base := projectID + "__" + strconv.FormatInt(deletedAt.UnixNano(), 10)
	for i := 0; i < 1000; i++ {
		candidate := base
		if i > 0 {
			candidate = base + "-" + strconv.Itoa(i)
		}
		if err := SafeChildName(candidate); err != nil {
			return "", "", fmt.Errorf("invalid trash entry id %q: %w", candidate, err)
		}
		entryPath, err := SafeJoin(trashDir, candidate)
		if err != nil {
			return "", "", err
		}
		if _, statErr := os.Stat(entryPath); os.IsNotExist(statErr) {
			return candidate, entryPath, nil
		}
	}
	return "", "", fmt.Errorf("could not allocate a unique trash entry for project %q", projectID)
}

func writeTombstone(entryPath string, tomb *TrashTombstone) error {
	data, err := marshalTombstone(tomb)
	if err != nil {
		return err
	}
	tombPath, err := SafeJoin(entryPath, trashTombstoneFile)
	if err != nil {
		return err
	}
	if err := util.AtomicWriteFile(tombPath, data, FilePermissions); err != nil {
		return fmt.Errorf("failed to write tombstone: %w", err)
	}
	return nil
}

// marshalTombstone renders a tombstone to its canonical indented JSON bytes, the
// same form writeTombstone persists. The journaled trash transaction stages these
// bytes as an OpWrite so the tombstone lands atomically with the directory move.
func marshalTombstone(tomb *TrashTombstone) ([]byte, error) {
	data, err := json.MarshalIndent(tomb, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal tombstone: %w", err)
	}
	return data, nil
}

func readTombstoneAt(entryPath string) (*TrashTombstone, error) {
	tombPath, err := SafeJoin(entryPath, trashTombstoneFile)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(tombPath)
	if err != nil {
		return nil, err
	}
	var tomb TrashTombstone
	if err := json.Unmarshal(data, &tomb); err != nil {
		return nil, fmt.Errorf("failed to parse tombstone %s: %w", tombPath, err)
	}
	return &tomb, nil
}

// ReadTrashTombstone loads the tombstone for a trash entry by ID.
func ReadTrashTombstone(dataDir, entryID string) (*TrashTombstone, error) {
	if entryID == "" {
		return nil, fmt.Errorf("trash entry id cannot be empty")
	}
	entryPath, err := SafeJoin(getTrashDir(dataDir), entryID)
	if err != nil {
		return nil, fmt.Errorf("invalid trash entry id %q: %w", entryID, err)
	}
	return readTombstoneAt(entryPath)
}

// ListTrashEntries returns all trash entries, newest deletion first.
func ListTrashEntries(dataDir string) ([]TrashEntry, error) {
	trashDir := getTrashDir(dataDir)
	dirEntries, err := os.ReadDir(trashDir)
	if os.IsNotExist(err) {
		return []TrashEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read trash directory: %w", err)
	}

	entries := make([]TrashEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if !de.IsDir() {
			continue
		}
		entryID := de.Name()
		entryPath, joinErr := SafeJoin(trashDir, entryID)
		if joinErr != nil {
			continue
		}
		entry := TrashEntry{EntryID: entryID}
		if tomb, tErr := readTombstoneAt(entryPath); tErr == nil && tomb.Project != nil {
			entry.ProjectID = tomb.Project.ID
			entry.Slug = tomb.Project.Slug
			entry.Name = tomb.Project.Name
			entry.DeletedAt = tomb.DeletedAt
			entry.Reason = tomb.DeletionReason
		} else {
			// Legacy slug-keyed entry: the directory name is the slug and there
			// is no recorded metadata. Surface it so it stays restorable.
			entry.Slug = entryID
			entry.Name = entryID
			entry.Legacy = true
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if !entries[i].DeletedAt.Equal(entries[j].DeletedAt) {
			return entries[i].DeletedAt.After(entries[j].DeletedAt)
		}
		return entries[i].EntryID < entries[j].EntryID
	})
	return entries, nil
}

// FindTrashEntries returns the trash entries matching ref. An exact entry-ID
// match returns that single entry; otherwise ref is treated as a slug and every
// generation sharing that slug is returned (newest first). Returns an empty
// slice when nothing matches.
func FindTrashEntries(dataDir, ref string) ([]TrashEntry, error) {
	if ref == "" {
		return nil, fmt.Errorf("trash reference cannot be empty")
	}
	all, err := ListTrashEntries(dataDir)
	if err != nil {
		return nil, err
	}
	var byID, bySlug []TrashEntry
	for _, e := range all {
		if e.EntryID == ref {
			byID = append(byID, e)
		}
		if e.Slug == ref {
			bySlug = append(bySlug, e)
		}
	}
	if len(byID) > 0 {
		return byID, nil
	}
	return bySlug, nil
}

// RestoreTrashEntry moves a trash entry's data back to its project directory
// under the data root and returns the faithfully reconstructed project — the
// original ID, slug, name, context, timestamps, and archive/pin state are
// preserved from the tombstone, while task data is reloaded from disk. The
// tombstone is returned so callers can re-establish configuration references and
// roll back (via MoveProjectToTrash) if a later step fails. The trash entry is
// removed only after the data is safely in place.
func RestoreTrashEntry(dataDir, entryID string) (*models.Project, *TrashTombstone, error) {
	if entryID == "" {
		return nil, nil, fmt.Errorf("trash entry id cannot be empty")
	}
	trashDir := getTrashDir(dataDir)
	entryPath, err := SafeJoin(trashDir, entryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid trash entry id %q: %w", entryID, err)
	}
	if _, statErr := os.Stat(entryPath); os.IsNotExist(statErr) {
		return nil, nil, fmt.Errorf("%w: '%s' not in trash", tterrors.ErrProjectNotFound, entryID)
	}

	tomb, dataSource, err := loadTrashEntryForRestore(entryPath, entryID)
	if err != nil {
		return nil, nil, err
	}

	slug := tomb.Project.Slug
	if nameErr := SafeChildName(slug); nameErr != nil {
		return nil, nil, fmt.Errorf("invalid restored project slug %q: %w", slug, nameErr)
	}
	projectPath, err := SafeJoin(filepath.Join(dataDir, "projects"), slug)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid restored project slug %q: %w", slug, err)
	}
	if _, statErr := os.Stat(projectPath); statErr == nil {
		return nil, nil, fmt.Errorf("%w: '%s' already exists; rename or delete it first", tterrors.ErrProjectExists, slug)
	}
	if err := os.MkdirAll(filepath.Dir(projectPath), DirPermissions); err != nil {
		return nil, nil, fmt.Errorf("failed to create projects directory: %w", err)
	}
	if err := os.Rename(dataSource, projectPath); err != nil {
		return nil, nil, fmt.Errorf("failed to restore project from trash: %w", err)
	}

	project := tomb.Project
	project.Path = projectPath
	if err := reloadProjectComputedFields(project); err != nil {
		// Roll the data back into the trash entry so nothing is lost.
		_ = os.Rename(projectPath, dataSource)
		return nil, nil, err
	}

	// Data is safely restored; drop the (now data-less) trash entry.
	_ = os.RemoveAll(entryPath)

	return project, tomb, nil
}

// loadTrashEntryForRestore resolves the tombstone and the on-disk data source
// for a trash entry, synthesizing a minimal tombstone for legacy entries that
// predate the versioned layout.
func loadTrashEntryForRestore(entryPath, entryID string) (*TrashTombstone, string, error) {
	dataPath, err := SafeJoin(entryPath, trashDataDirName)
	if err != nil {
		return nil, "", err
	}

	tomb, tombErr := readTombstoneAt(entryPath)
	if tombErr == nil && tomb.Project != nil {
		if _, statErr := os.Stat(dataPath); statErr != nil {
			return nil, "", fmt.Errorf("trash entry %q is missing its data directory: %w", entryID, statErr)
		}
		return tomb, dataPath, nil
	}

	// Legacy fallback: no tombstone. The entry directory itself holds the
	// project files and its name is the original slug. Reconstruct just enough
	// metadata to restore it (a fresh ID and defaults, mirroring the pre-v2
	// behavior) rather than stranding the data.
	now := time.Now()
	legacy := &models.Project{
		ID:        uuid.New().String(),
		Slug:      entryID,
		Name:      entryID,
		Context:   "personal",
		CreatedAt: now,
		UpdatedAt: now,
	}
	return &TrashTombstone{
		SchemaVersion: trashTombstoneSchemaVersion,
		EntryID:       entryID,
		Project:       legacy,
		DeletedAt:     now,
	}, entryPath, nil
}

// reloadProjectComputedFields loads tasks.md into the project's computed fields
// (Tasks, sections, tags, NextShortID) without disturbing its config-level
// identity (ID, name, context, timestamps, archive/pin state). It also parses a
// project name from the file when the tombstone carried none (legacy entries).
func reloadProjectComputedFields(project *models.Project) error {
	loaded := &models.Project{Path: project.Path}
	if err := LoadProject(loaded); err != nil {
		return fmt.Errorf("failed to load restored project tasks: %w", err)
	}
	project.Tasks = loaded.Tasks
	project.NextShortID = loaded.NextShortID
	project.Sections = loaded.Sections
	project.SectionColors = loaded.SectionColors
	project.Tags = loaded.Tags
	project.PinnedTags = loaded.PinnedTags

	if project.Name == "" || project.Name == project.Slug {
		if name, err := ParseProjectNameFromFile(tasksFilePath(project.Path)); err == nil && name != "" {
			project.Name = name
		}
	}
	if project.Name == "" {
		project.Name = project.Slug
	}
	if project.Context == "" {
		project.Context = "personal"
	}
	return nil
}

// PurgeTrashEntry permanently deletes a single trash entry by ID. It is a no-op
// when the entry does not exist.
func PurgeTrashEntry(dataDir, entryID string) error {
	if entryID == "" {
		return fmt.Errorf("trash entry id cannot be empty")
	}
	entryPath, err := SafeJoin(getTrashDir(dataDir), entryID)
	if err != nil {
		return fmt.Errorf("invalid trash entry id %q: %w", entryID, err)
	}
	if _, statErr := os.Stat(entryPath); os.IsNotExist(statErr) {
		return nil
	}
	return os.RemoveAll(entryPath)
}
