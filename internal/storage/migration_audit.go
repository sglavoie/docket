package storage

import "github.com/sglavoie/tt/internal/models"

// MigrationAuditEntry records the retired legacy lifecycle values a v1-parsed
// task carried at load time. Under the workflow-state model the
// Status/Section/Stage fields are no longer operational; they are captured for
// migration reporting while parsing never mutates the task. The former active
// marker is deliberately discarded.
type MigrationAuditEntry struct {
	TaskID  string
	ShortID int
	Status  models.TaskStatus
	Section string
	Stage   string
}

// MigrationAudit is the loader-level capture of every task's retired legacy
// lifecycle values for one project. The v1 parser has no config or Focus Queue
// access, so this structure only records the parsed values for reporting.
type MigrationAudit struct {
	ProjectID string
	Entries   []MigrationAuditEntry
}

// NewMigrationAudit wraps the per-task entries captured during parsing (see
// ParseResult.MigrationAudit) with the owning project ID. The retired
// Status/Section/Stage values no longer exist as task fields; they are
// captured at parse time from the schema-v1 metadata a document carried, so the
// audit is the sole surviving record of them. A nil/empty entry slice still
// yields a non-nil audit with no entries.
func NewMigrationAudit(projectID string, entries []MigrationAuditEntry) *MigrationAudit {
	return &MigrationAudit{ProjectID: projectID, Entries: entries}
}
