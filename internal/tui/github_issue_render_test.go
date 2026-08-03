package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestGitHubIssueAttachmentLabels(t *testing.T) {
	issueAttachment := models.Attachment{
		Filename: "https://github.com/acme/roadmap/issues/123",
		IsURL:    true,
	}
	plainURL := models.Attachment{
		Filename: "https://example.com/design.pdf",
		IsURL:    true,
	}
	model := NewTaskAttachmentsModel(&models.Task{}, &models.Project{}, nil, AttachmentCoordinator{})

	if got := model.attachmentTypeLabel(issueAttachment); got != "GITHUB ISSUE" {
		t.Fatalf("GitHub issue attachment label = %q, want GITHUB ISSUE", got)
	}
	if got := model.attachmentDisplayName(issueAttachment); got != "acme/roadmap#123" {
		t.Fatalf("GitHub issue attachment display name = %q, want compact issue ref", got)
	}
	if got := model.attachmentTypeLabel(plainURL); got != "URL " {
		t.Fatalf("plain URL label = %q, want URL", got)
	}
	if got := model.attachmentDisplayName(plainURL); got != plainURL.Filename {
		t.Fatalf("plain URL display name = %q, want original URL", got)
	}
}
