package config

import (
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
)

const (
	// V6ConfigSchemaVersion is the independently validated configuration
	// payload version produced by this dry run. Normal startup remains on v5
	// until the explicit live cutover.
	V6ConfigSchemaVersion = 6

	V6UnfiledProjectName = "Unfiled"
	V6UnfiledProjectSlug = "unfiled"
)

// V6WorkspaceDryRunOptions identifies the schema-5 workspace and optionally
// supplies an isolated machine-state path for migration rehearsals.
type V6WorkspaceDryRunOptions struct {
	DataDir          string
	MachineStatePath string
}

// V6CatalogProjectCandidate is the retained project shape in schema 6.
type V6CatalogProjectCandidate struct {
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

// V6WorkspaceCatalogCandidate renames the protected reference while retaining
// the stable project and workspace identities.
type V6WorkspaceCatalogCandidate struct {
	SchemaVersion          int                         `json:"schema_version"`
	WorkspaceID            string                      `json:"workspace_id"`
	Projects               []V6CatalogProjectCandidate `json:"projects"`
	SystemUnfiledProjectID string                      `json:"system_unfiled_project_id"`
}

type V6PlanningTaskRef = ProjectTaskLocator

// V6PlanningCandidate owns two explicit, ordered membership lists. Lifecycle
// state no longer carries Next membership.
type V6PlanningCandidate struct {
	Today []V6PlanningTaskRef `json:"today,omitempty"`
	Next  []V6PlanningTaskRef `json:"next,omitempty"`
}

// V6MachineCandidate retains only the selected built-in theme.
type V6MachineCandidate struct {
	Theme string `json:"theme"`
}

type V6DomainCandidates struct {
	Catalog  V6WorkspaceCatalogCandidate
	Planning V6PlanningCandidate
	Machine  V6MachineCandidate
}

// V6DomainMigrationAuditEntry records deterministic catalog, planning, and
// machine repairs made while assembling the schema-6 candidate.
type V6DomainMigrationAuditEntry struct {
	Domain    string `json:"domain"`
	ProjectID string `json:"project_id,omitempty"`
	EntityID  string `json:"entity_id,omitempty"`
	Action    string `json:"action"`
	Field     string `json:"field"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Reason    string `json:"reason"`
}

type V6DomainConversionResult struct {
	Domains V6DomainCandidates
	Audit   []V6DomainMigrationAuditEntry
}

// V6ProjectDocumentCandidate pairs one validated schema-6 document with its
// source and eventual target paths. Archived projects with no source task file
// receive an explicit empty candidate.
type V6ProjectDocumentCandidate struct {
	ProjectID    string
	SourcePath   string
	Path         string
	SourceExists bool
	Document     storage.V6ProjectDocument
}

// V6WorkspaceArtifactCandidate contains exact target bytes for domains,
// project documents, and retained notes. SourcePath may differ from Path when
// the protected Inbox directory becomes unfiled.
type V6WorkspaceArtifactCandidate struct {
	SourcePath   string
	Path         string
	Kind         string
	ProjectID    string
	SourceExists bool
	Data         []byte
}

// V6RetainedAttachmentCandidate records one validated local attachment move.
// Target paths always use the canonical hashed task directory, even when the
// v5 source uses a legacy raw task-ID directory.
type V6RetainedAttachmentCandidate struct {
	ProjectID  string
	TaskID     string
	Filename   string
	SourcePath string
	Path       string
	Size       int64
	SHA256     string
}

// V6ObsoleteAttachmentCandidate is a legacy attachment directory that belongs
// to no retained task. Its bytes remain available in the immutable pre-v6
// snapshot, but activation removes the live directory instead of carrying an
// orphan across the schema boundary.
type V6ObsoleteAttachmentCandidate struct {
	ProjectID string
	Path      string
}

// V6WorkspaceConversionPlan is a filesystem-free publication candidate after
// construction: all source files have been read, every target byte is fixed,
// and retained attachment topology is represented by checked source/target
// pairs. Building the plan never writes or renames live data.
type V6WorkspaceConversionPlan struct {
	Domains             V6DomainConversionResult
	Projects            []V6ProjectDocumentCandidate
	Artifacts           []V6WorkspaceArtifactCandidate
	Attachments         []V6RetainedAttachmentCandidate
	ObsoleteAttachments []V6ObsoleteAttachmentCandidate
	TaskAudit           []format.V6TaskMigrationAuditEntry
}

type v6CandidateTask struct {
	ProjectID string
	Task      format.V6TaskRecord
	Next      bool
}
