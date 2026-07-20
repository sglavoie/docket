package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRenderGitHubIssueIndicator(t *testing.T) {
	task := &models.Task{
		URL: "https://github.com/acme/roadmap/issues/123",
		Attachments: []models.Attachment{
			{Filename: "https://github.com/acme/roadmap/issues/456", IsURL: true},
		},
	}

	got := ansi.Strip(renderGitHubIssueIndicator(task))
	if !strings.Contains(got, "GH acme/roadmap#123 +1") {
		t.Fatalf("expected compact issue indicator with overflow count, got %q", got)
	}

	plainTask := &models.Task{URL: "https://example.com/acme/roadmap/issues/123"}
	if got := renderGitHubIssueIndicator(plainTask); got != "" {
		t.Fatalf("expected no issue indicator for non-GitHub URL, got %q", got)
	}
}

func TestGitHubIssueMetadataPreservesAttachmentCount(t *testing.T) {
	task := &models.Task{
		Attachments: []models.Attachment{
			{Filename: "https://github.com/acme/roadmap/issues/123", IsURL: true},
			{Filename: "notes.md"},
		},
	}
	wantIssue := ansi.Strip(renderGitHubIssueIndicator(task))
	wantAttachments := ansi.Strip(renderAttachmentIndicator(task))

	outliner := &OutlinerModel{
		columnConfig: NewColumnConfig(map[string]bool{ColAttachments: true}, nil, DefaultOutlinerColumns, ""),
	}
	outlinerContent := ansi.Strip(outliner.renderTaskContentGroup(task, &models.Project{ID: "p"}))
	if !strings.Contains(outlinerContent, wantIssue) {
		t.Fatalf("outliner content missing GitHub issue indicator %q: %q", wantIssue, outlinerContent)
	}
	if !strings.Contains(outlinerContent, wantAttachments) {
		t.Fatalf("outliner content missing generic attachment count %q: %q", wantAttachments, outlinerContent)
	}
}

func TestCrossProjectTaskLineShowsGitHubIssueIndicator(t *testing.T) {
	task := &models.Task{
		ID:            "t1",
		Title:         "Fix sync",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		URL:           "https://github.com/acme/roadmap/issues/123",
	}
	proj := &models.Project{ID: "p1", Name: "Roadmap"}

	got := ansi.Strip(RenderCrossProjectTaskLine(task, proj, nil, false, 120, 40, CrossProjectTaskLineOpts{}))
	if !strings.Contains(got, "GH acme/roadmap#123") {
		t.Fatalf("expected cross-project task line to show GitHub issue indicator, got %q", got)
	}
}

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

func TestGitHubIssueAttachmentSurfacesUseCompactLabel(t *testing.T) {
	task := &models.Task{
		Attachments: []models.Attachment{
			{Filename: "https://github.com/acme/roadmap/issues/123", IsURL: true},
			{Filename: "https://example.com/design.pdf", IsURL: true},
		},
	}

	detail := ansi.Strip((&TaskDetailModel{task: task}).renderAttachmentsSection(80))
	if !strings.Contains(detail, "GH acme/roadmap#123") {
		t.Fatalf("task detail attachments should label GitHub issue compactly, got:\n%s", detail)
	}

	sidebar := ansi.Strip(renderSidebarAttachmentsCard(task, 80))
	if !strings.Contains(sidebar, "GH acme/roadmap#123") {
		t.Fatalf("sidebar attachments should label GitHub issue compactly, got:\n%s", sidebar)
	}

	peek := ansi.Strip(strings.Join(peekAttachmentsBlock(task, 40), "\n"))
	if !strings.Contains(peek, "GitHub issue acme/roadmap#123") {
		t.Fatalf("peek attachments should label GitHub issue compactly, got:\n%s", peek)
	}
}

func TestPeekGitHubIssueAttachmentDoesNotOverflowWidth(t *testing.T) {
	task := &models.Task{
		Attachments: []models.Attachment{
			{
				Filename: "https://github.com/superlongownername/superlongrepositoryname/issues/123456",
				IsURL:    true,
			},
		},
	}

	const width = 28
	lines := peekAttachmentsBlock(task, width)
	if len(lines) == 0 {
		t.Fatal("expected lines, got none")
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d exceeds width %d: width=%d, content=%q", i, width, got, line)
		}
	}
}

func TestKanbanGitHubIssueIndicatorDoesNotOverflowCard(t *testing.T) {
	project := testutil.NewTestProject("p")
	task := testutil.NewTestTask("Implement the issue metadata marker without overflowing", models.TaskTypeTask)
	task.Attachments = []models.Attachment{
		{Filename: "https://github.com/acme/roadmap/issues/123", IsURL: true},
	}
	col := KanbanColumn{
		ID:    "inbox",
		Label: "inbox",
		Icon:  "·",
		Tasks: []KanbanCard{{Task: task, Project: project}},
	}

	m := newRenderKanban(36, 12, []KanbanColumn{col})
	for i, line := range strings.Split(ansi.Strip(kanbanRender(m)), "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("line %d exceeds width %d: width=%d, content=%q", i, m.width, got, line)
		}
	}
}
