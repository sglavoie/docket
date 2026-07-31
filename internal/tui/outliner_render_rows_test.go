package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectTaskRowsOmitProjectBadge(t *testing.T) {
	alpha := primaryRowsProject("alpha", primaryRowsTask("alpha-task", models.StateReady))
	beta := primaryRowsProject("beta", primaryRowsTask("beta-task", models.StateReady))
	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(ModeAll)

	headerLabels := make(map[string]bool)
	for index, row := range outliner.rows {
		if row.IsProjectHeader {
			headerLabels[row.ProjectID] = strings.Contains(
				ansi.Strip(outliner.renderOutlinerProjectHeaderRow(row, false, 120, index)),
				"["+row.ProjectID+"]",
			)
			continue
		}
		if !row.isTaskRow() || row.task == nil {
			continue
		}

		line := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, index, nil))
		if strings.Contains(line, "["+row.ProjectID+"]") {
			t.Errorf("task row %q repeated project badge in %q", row.task.ID, line)
		}
	}

	for _, projectID := range []string{alpha.ID, beta.ID} {
		if !headerLabels[projectID] {
			t.Errorf("project header %q did not retain its label", projectID)
		}
	}
}

func TestOutlinerTaskRowShowsCompactCommentAndAttachmentCounts(t *testing.T) {
	task := &models.Task{
		ID:            "task",
		Title:         "Review compact metadata",
		WorkflowState: models.StateReady,
		Comments:      []*models.Comment{{Text: "One"}, {Text: "Two"}},
		Attachments:   []models.Attachment{{Filename: "design.pdf"}},
	}
	project := renderFixtureProject(t, "project", "project", "Project", []*models.Task{task})
	outliner := NewOutlinerModel(project, nil, newTestConfig())
	row := outlinerRow{task: task, ProjectID: project.ID}

	plain := ansi.Strip(outliner.renderOutlinerTaskRow(row, false, 120, 120, 0, nil))
	for _, want := range []string{"▪ 2", "⊕ 1"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("task row missing %q: %q", want, plain)
		}
	}
}
