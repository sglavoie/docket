package storage

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestArchiveTaskSummaryContractCoversArchiveAnalytics(t *testing.T) {
	completedAt := time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC)
	startedAt := completedAt.Add(-48 * time.Hour)

	summary := ArchiveTaskSummary{
		ID:             "task-123",
		ShortID:        123,
		Type:           models.TaskTypeStory,
		Title:          "Archive summary contract",
		WorkflowState:  models.StateDone,
		Priority:       2,
		CreatedAt:      completedAt.AddDate(0, 0, -7),
		CompletedAt:    &completedAt,
		FirstStartedAt: &startedAt,
		EstimatedDays:  3,
		Effort:         models.EffortMedium,
		StoryPoints:    5,
		Source: ArchiveTaskSource{
			FileName:    "completed.2.md",
			HeadingLine: 42,
		},
	}

	if summary.ID == "" {
		t.Fatal("summary ID must be present for prefix lookup and de-duplication")
	}
	if !summary.WorkflowState.IsTerminal() || summary.CompletedAt == nil {
		t.Fatal("archive summaries must represent resolved work")
	}
	if summary.FirstStartedAt == nil {
		t.Fatal("cycle-time metrics need first in-progress timestamp without full activity parsing")
	}
	if summary.Effort == "" || summary.StoryPoints == 0 || summary.EstimatedDays == 0 {
		t.Fatal("roadmap velocity needs compact effort fields")
	}
	if summary.Source.FileName == "" {
		t.Fatal("detail lookup needs a source archive file")
	}

	payload, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	if strings.Contains(string(payload), "description") || strings.Contains(string(payload), "activity") {
		t.Fatalf("summary JSON should not duplicate full task bodies: %s", payload)
	}

	var roundTrip ArchiveTaskSummary
	if err := json.Unmarshal(payload, &roundTrip); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if roundTrip.ID != summary.ID || roundTrip.Source.FileName != summary.Source.FileName {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", roundTrip, summary)
	}
}

func TestArchiveIndexContractDescribesDerivedState(t *testing.T) {
	generatedAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	index := ArchiveIndex{
		Version:     archiveIndexVersion,
		GeneratedAt: generatedAt,
		Sources: []ArchiveIndexSource{
			{
				FileName:  archiveFileName,
				SizeBytes: 1024,
				ModTime:   generatedAt.Add(-time.Minute),
				SHA256:    strings.Repeat("a", 64),
				TaskCount: 10,
			},
		},
		Tasks: []ArchiveTaskSummary{
			{
				ID:            "task-1",
				WorkflowState: models.StateDone,
				CompletedAt:   &generatedAt,
				Source:        ArchiveTaskSource{FileName: archiveFileName},
			},
		},
	}

	if index.Version != archiveIndexVersion {
		t.Fatalf("Version = %d, want %d", index.Version, archiveIndexVersion)
	}
	if !index.GeneratedAt.Equal(generatedAt) {
		t.Fatalf("GeneratedAt = %v, want %v", index.GeneratedAt, generatedAt)
	}
	if len(index.Sources) != 1 || index.Sources[0].FileName != archiveFileName {
		t.Fatalf("index must record source archive files: %+v", index.Sources)
	}
	if len(index.Tasks) != 1 || index.Tasks[0].Source.FileName != archiveFileName {
		t.Fatalf("index tasks must point back to canonical archive files: %+v", index.Tasks)
	}
}
