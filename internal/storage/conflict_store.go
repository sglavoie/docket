package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/util"
)

// Persisted file names inside a conflict's directory. The three documents are
// stored as canonical v2 markdown so a human (or an external merge tool) can
// read and edit them directly, while the structured metadata lives in meta.json.
const (
	conflictMetaFileName   = "meta.json"
	conflictBaseFileName   = "base.md"
	conflictLocalFileName  = "local.md"
	conflictRemoteFileName = "remote.md"
)

// ConflictRecord is the structured, machine-readable metadata for one persisted
// three-way merge conflict. It records which project diverged, the revisions of
// the three inputs, what mutation triggered the conflict, and the list of
// structured MergeConflicts the merge produced. The base/local/remote documents
// themselves are stored alongside it as separate markdown files (see
// ConflictStore) so the metadata stays small and the documents stay canonical.
type ConflictRecord struct {
	ID             string `json:"id"`
	ProjectID      string `json:"project_id"`
	ProjectName    string `json:"project_name"`
	ProjectSlug    string `json:"project_slug"`
	ProjectPath    string `json:"project_path"`
	Action         string `json:"action,omitempty"`
	Context        string `json:"context,omitempty"`
	BaseRevision   string `json:"base_revision"`
	LocalRevision  string `json:"local_revision"`
	RemoteRevision string `json:"remote_revision"`
	// RequiresOperationRetry is a pointer so old conflict records that predate
	// this safety marker remain distinguishable from an explicit project-only
	// false. A missing value fails closed through OperationRetryRequired: an old
	// bundle may have come from an aborted multi-artifact transaction.
	RequiresOperationRetry *bool           `json:"requires_operation_retry,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	Conflicts              []MergeConflict `json:"conflicts"`
}

// OperationRetryRequired reports whether resolving this bundle must be
// non-applying and require the original operation to be retried. Missing policy
// metadata defaults to true so pre-policy bundles cannot silently apply only one
// project from what may have been a larger aborted transaction.
func (r ConflictRecord) OperationRetryRequired() bool {
	return r.RequiresOperationRetry == nil || *r.RequiresOperationRetry
}

// ConflictBundle is a fully-loaded conflict: its metadata plus the three parsed
// documents (the common base, the caller's local edits, and the remote/on-disk
// state at the time the conflict was recorded). It is the unit `tt conflict show`
// and the TUI resolution view render. Explicit project-only bundles may be
// applied by the resolver; commit-origin and pre-policy bundles are retry-only.
type ConflictBundle struct {
	Record ConflictRecord
	Base   *ProjectDocument
	Local  *ProjectDocument
	Remote *ProjectDocument
}

// ConflictStore persists and enumerates conflict bundles under a single durable
// directory (typically config.ConflictsDir(dataDir)). Each conflict occupies its
// own subdirectory named by a generated UUID, holding meta.json plus the three
// document markdown files. All writes go through util.AtomicWriteFile for
// durability, and every path is contained beneath the store directory via
// util.SafeJoin so a malformed id can never escape it.
type ConflictStore struct {
	dir string
}

// NewConflictStore returns a store rooted at dir. dir is created lazily on the
// first Save.
func NewConflictStore(dir string) *ConflictStore {
	return &ConflictStore{dir: dir}
}

// Dir returns the store's root directory.
func (s *ConflictStore) Dir() string { return s.dir }

// Save persists a conflict bundle and returns its generated id. The record's ID
// and CreatedAt are assigned here (any caller-supplied values are overwritten) so
// ids are always unique and timestamps always reflect the moment of recording.
// The three documents are written as canonical v2 markdown; base/local/remote may
// be nil (an empty document is written) but that should not happen in practice.
func (s *ConflictStore) Save(record ConflictRecord, base, local, remote *ProjectDocument) (string, error) {
	id := uuid.NewString()
	record.ID = id
	record.CreatedAt = time.Now().UTC()

	dir, err := util.SafeJoin(s.dir, id)
	if err != nil {
		return "", fmt.Errorf("resolving conflict directory: %w", err)
	}
	if mkErr := os.MkdirAll(dir, DirPermissions); mkErr != nil {
		return "", fmt.Errorf("creating conflict directory: %w", mkErr)
	}

	metaBytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling conflict metadata: %w", err)
	}
	if writeErr := util.AtomicWriteFile(filepath.Join(dir, conflictMetaFileName), metaBytes, FilePermissions); writeErr != nil {
		return "", fmt.Errorf("writing conflict metadata: %w", writeErr)
	}

	for name, doc := range map[string]*ProjectDocument{
		conflictBaseFileName:   base,
		conflictLocalFileName:  local,
		conflictRemoteFileName: remote,
	} {
		if writeErr := util.AtomicWriteFile(filepath.Join(dir, name), []byte(documentMarkdown(doc)), FilePermissions); writeErr != nil {
			return "", fmt.Errorf("writing conflict document %s: %w", name, writeErr)
		}
	}

	return id, nil
}

// List returns every stored conflict's metadata, newest first. A subdirectory
// without a readable meta.json is skipped rather than aborting the whole listing,
// so one corrupt bundle never hides the others.
func (s *ConflictStore) List() ([]ConflictRecord, error) {
	entries, err := os.ReadDir(s.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading conflicts directory: %w", err)
	}

	var records []ConflictRecord
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := s.loadRecord(entry.Name())
		if err != nil {
			continue
		}
		records = append(records, *record)
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	return records, nil
}

// Load reads a conflict's metadata and its three documents. A missing conflict
// returns an error naming the id.
func (s *ConflictStore) Load(id string) (*ConflictBundle, error) {
	record, err := s.loadRecord(id)
	if err != nil {
		return nil, err
	}

	dir, err := util.SafeJoin(s.dir, id)
	if err != nil {
		return nil, fmt.Errorf("resolving conflict directory: %w", err)
	}

	base, err := loadDocumentFile(filepath.Join(dir, conflictBaseFileName))
	if err != nil {
		return nil, fmt.Errorf("loading base document: %w", err)
	}
	local, err := loadDocumentFile(filepath.Join(dir, conflictLocalFileName))
	if err != nil {
		return nil, fmt.Errorf("loading local document: %w", err)
	}
	remote, err := loadDocumentFile(filepath.Join(dir, conflictRemoteFileName))
	if err != nil {
		return nil, fmt.Errorf("loading remote document: %w", err)
	}

	return &ConflictBundle{Record: *record, Base: base, Local: local, Remote: remote}, nil
}

// Delete removes a conflict's whole directory. It is a no-op when the conflict is
// already gone, so resolving the same id twice does not error.
func (s *ConflictStore) Delete(id string) error {
	dir, err := util.SafeJoin(s.dir, id)
	if err != nil {
		return fmt.Errorf("resolving conflict directory: %w", err)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("removing conflict %s: %w", id, err)
	}
	return nil
}

// loadRecord reads and unmarshals a single conflict's meta.json.
func (s *ConflictStore) loadRecord(id string) (*ConflictRecord, error) {
	dir, err := util.SafeJoin(s.dir, id)
	if err != nil {
		return nil, fmt.Errorf("resolving conflict directory: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, conflictMetaFileName))
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("conflict %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("reading conflict metadata: %w", err)
	}
	var record ConflictRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("parsing conflict metadata: %w", err)
	}
	return &record, nil
}

// documentMarkdown renders a document to canonical v2 markdown, treating a nil
// document as empty content.
func documentMarkdown(doc *ProjectDocument) string {
	if doc == nil {
		return ""
	}
	return doc.WriteV2()
}

// loadDocumentFile reads and parses a stored document markdown file back into a
// ProjectDocument. A missing file yields a nil document rather than an error, so
// a partially-written bundle degrades instead of blocking resolution entirely.
func loadDocumentFile(path string) (*ProjectDocument, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ParseProjectDocument(string(content))
}
