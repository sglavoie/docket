package format

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTrip_Attachments(t *testing.T) {
	now := time.Now()
	task := &models.Task{
		ID:       "att-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		Attachments: []models.Attachment{
			{
				Filename:     "report.pdf",
				IsURL:        false,
				IsCodeRef:    false,
				OriginalPath: "/home/user/docs/report.pdf",
				AddedAt:      now,
			},
			{
				Filename:     "https://example.com/page",
				IsURL:        true,
				OriginalPath: "https://example.com/page",
				AddedAt:      now,
			},
			{
				Filename:     "main.go",
				IsURL:        false,
				IsCodeRef:    true,
				LineNumber:   42,
				OriginalPath: "/home/user/src/main.go",
				AddedAt:      now,
			},
		},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if len(parsed.Attachments) != 3 {
		t.Fatalf("got %d attachments, want 3", len(parsed.Attachments))
	}

	got := parsed.Attachments[0]
	if got.Filename != "report.pdf" {
		t.Errorf("att[0].Filename = %q, want %q", got.Filename, "report.pdf")
	}
	if got.IsURL {
		t.Errorf("att[0].IsURL = true, want false")
	}
	if got.IsCodeRef {
		t.Errorf("att[0].IsCodeRef = true, want false")
	}
	if got.OriginalPath != "/home/user/docs/report.pdf" {
		t.Errorf("att[0].OriginalPath = %q, want %q", got.OriginalPath, "/home/user/docs/report.pdf")
	}
	if got.AddedAt.Unix() != now.Unix() {
		t.Errorf("att[0].AddedAt.Unix() = %d, want %d", got.AddedAt.Unix(), now.Unix())
	}

	got = parsed.Attachments[1]
	if got.Filename != "https://example.com/page" {
		t.Errorf("att[1].Filename = %q, want %q", got.Filename, "https://example.com/page")
	}
	if !got.IsURL {
		t.Errorf("att[1].IsURL = false, want true")
	}
	if got.OriginalPath != "https://example.com/page" {
		t.Errorf("att[1].OriginalPath = %q, want %q", got.OriginalPath, "https://example.com/page")
	}
	if got.AddedAt.Unix() != now.Unix() {
		t.Errorf("att[1].AddedAt.Unix() = %d, want %d", got.AddedAt.Unix(), now.Unix())
	}

	got = parsed.Attachments[2]
	if got.Filename != "main.go" {
		t.Errorf("att[2].Filename = %q, want %q", got.Filename, "main.go")
	}
	if !got.IsCodeRef {
		t.Errorf("att[2].IsCodeRef = false, want true")
	}
	if got.LineNumber != 42 {
		t.Errorf("att[2].LineNumber = %d, want 42", got.LineNumber)
	}
	if got.OriginalPath != "/home/user/src/main.go" {
		t.Errorf("att[2].OriginalPath = %q, want %q", got.OriginalPath, "/home/user/src/main.go")
	}
	if got.AddedAt.Unix() != now.Unix() {
		t.Errorf("att[2].AddedAt.Unix() = %d, want %d", got.AddedAt.Unix(), now.Unix())
	}
}

func TestParse_AttachmentMalformedDoubleBar(t *testing.T) {
	metaStr := "id: att-bad, status: todo, priority: 1, attachments: report.pdf||/path/with||double@1609459200"

	task := &models.Task{}
	warnings := ParseMetadata(task, metaStr)

	if len(warnings) == 0 {
		t.Error("expected at least one warning for malformed attachment, got none")
	}

	found := false
	for _, w := range warnings {
		if w.Field == "attachments" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a warning with Field=attachments, got: %v", warnings)
	}

	if len(task.Attachments) == 0 {
		t.Error("expected attachment to be present even when malformed (graceful degradation)")
	}
}
