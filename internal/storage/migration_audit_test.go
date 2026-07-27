package storage

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseMigrationAudit_RecordsRetiredFields(t *testing.T) {
	// The retired schema-v1 lifecycle values are captured at parse time from the
	// legacy metadata keys, since the task fields no longer exist.
	content := `# Project: Legacy

## Task: In progress with stage
<!-- id: a, status: in_progress, section: Next, stage: Design, active: true, priority: 1 -->

## Task: Plain inbox
<!-- id: b, status: todo, section: Inbox, priority: 2 -->

## Task: Completed
<!-- id: c, status: done, section: Completed, active: true, priority: 3 -->
`

	result, err := ParseTasksMarkdownDocument(content)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	audit := NewMigrationAudit("proj-1", result.MigrationAudit)
	if audit.ProjectID != "proj-1" {
		t.Errorf("ProjectID = %q, want proj-1", audit.ProjectID)
	}
	if len(audit.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(audit.Entries))
	}

	// The retired values are recorded verbatim from the schema-v1 metadata.
	got := audit.Entries[0]
	want := MigrationAuditEntry{TaskID: "a", ShortID: 1, Status: string(models.StateInProgress), Section: "Next", Stage: "Design"}
	if got != want {
		t.Errorf("entry[0] = %+v, want %+v", got, want)
	}
}
