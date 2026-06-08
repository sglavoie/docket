package storage

import (
	"time"

	"github.com/sglavoie/tt/internal/models"
)

const (
	archiveIndexFileName = ".completed.index.json"
	archiveIndexVersion  = 1
)

// ArchiveIndexIssue identifies why the derived archive index needs rebuilding.
type ArchiveIndexIssue string

const (
	ArchiveIndexIssueNone    ArchiveIndexIssue = ""
	ArchiveIndexIssueMissing ArchiveIndexIssue = "missing"
	ArchiveIndexIssueStale   ArchiveIndexIssue = "stale"
	ArchiveIndexIssueCorrupt ArchiveIndexIssue = "corrupt"
)

// ArchiveIndexStatus describes the derived archive index without repairing it.
type ArchiveIndexStatus struct {
	IndexPath        string
	IndexExists      bool
	HasArchives      bool
	ArchiveFileCount int
	SourceCount      int
	TaskCount        int
	Fresh            bool
	NeedsRebuild     bool
	Issue            ArchiveIndexIssue
	Detail           string
}

// ArchiveIndex is derived from the canonical archive markdown files.
//
// Invariants:
//   - completed.md and completed.N.md remain canonical; this index is rebuildable.
//   - Version identifies the index schema, not the archive markdown schema.
//   - Sources describes every archive file used to build Tasks.
//   - Tasks is ordered by archive file order, then task order within each file.
type ArchiveIndex struct {
	Version     int                  `json:"version"`
	GeneratedAt time.Time            `json:"generated_at"`
	Sources     []ArchiveIndexSource `json:"sources"`
	Tasks       []ArchiveTaskSummary `json:"tasks"`
}

// ArchiveIndexSource records the file metadata needed to detect stale index
// entries without parsing archive markdown on hot paths.
type ArchiveIndexSource struct {
	FileName  string    `json:"file_name"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"mod_time"`
	SHA256    string    `json:"sha256,omitempty"`
	TaskCount int       `json:"task_count"`
}

// ArchiveTaskSource points an archived task summary back to the canonical
// archive file. Detail views can parse only this file to recover the full task.
type ArchiveTaskSource struct {
	FileName    string `json:"file_name"`
	HeadingLine int    `json:"heading_line,omitempty"`
}

// ArchiveTaskSummary is the compact archive representation consumed by
// analytics and archive list views.
//
// Invariants:
//   - ID is non-empty and unique within a project archive index.
//   - Status is a resolved status and CompletedAt is set.
//   - Source.FileName names completed.md or a numeric completed.N.md file.
//   - Large task bodies, comments, decisions, attachments, and full activity
//     logs stay in canonical markdown and are intentionally not duplicated here.
type ArchiveTaskSummary struct {
	ID             string                `json:"id"`
	ShortID        int                   `json:"short_id"`
	Type           models.TaskType       `json:"type"`
	Title          string                `json:"title"`
	Status         models.TaskStatus     `json:"status"`
	Priority       int                   `json:"priority"`
	CreatedAt      time.Time             `json:"created_at"`
	CompletedAt    *time.Time            `json:"completed_at"`
	FirstStartedAt *time.Time            `json:"first_started_at,omitempty"`
	EstimatedDays  float64               `json:"estimated_days,omitempty"`
	Effort         models.EffortEstimate `json:"effort,omitempty"`
	StoryPoints    int                   `json:"story_points,omitempty"`
	Source         ArchiveTaskSource     `json:"source"`
}
