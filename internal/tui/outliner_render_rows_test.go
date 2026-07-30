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
