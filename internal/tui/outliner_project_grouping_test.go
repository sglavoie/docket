package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
)

// projectHeaderRowIDs returns the project headers actually drawn, in render
// order. Every cross-project group opens with one, expanded or folded.
func projectHeaderRowIDs(m *outliner.OutlinerModel) []string {
	var ids []string
	for _, row := range m.Rows {
		if row.IsProjectHeader {
			ids = append(ids, row.ProjectID)
		}
	}
	return ids
}

func newGroupingFixture(t *testing.T, mode chrome.OutlinerModeID) groupingFixture {
	t.Helper()
	var alphaOne, alphaTwo, betaOne *models.Task
	switch mode {
	case chrome.ModeInbox:
		alphaOne = primaryRowsTask("alpha-one", models.StateInbox)
		alphaTwo = primaryRowsTask("alpha-two", models.StateInbox)
		betaOne = primaryRowsTask("beta-one", models.StateInbox)
	case chrome.ModeBacklog:
		alphaOne = primaryRowsTask("alpha-one", models.StateBacklog)
		alphaTwo = primaryRowsTask("alpha-two", models.StateBacklog)
		betaOne = primaryRowsTask("beta-one", models.StateBacklog)
	case chrome.ModeDue:
		alphaOne = groupingDueTask("alpha-one", -3)
		alphaTwo = groupingDueTask("alpha-two", -2)
		betaOne = groupingDueTask("beta-one", -1)
	case chrome.ModeRecent:
		alphaOne = recentTask("alpha-one", 3)
		alphaTwo = recentTask("alpha-two", 4)
		betaOne = recentTask("beta-one", 5)
	default:
		t.Fatalf("mode %q already grouped before this contract; no fixture defined", mode)
		return groupingFixture{}
	}
	return groupingFixture{
		alpha: primaryRowsProject("alpha", alphaOne, alphaTwo),
		beta:  primaryRowsProject("beta", betaOne),
		tasks: []string{alphaOne.ID, alphaTwo.ID, betaOne.ID},
	}
}

// groupingFixture seeds two projects, alpha with two rows and beta with one, all
// visible in the named mode and ordered alpha-one, alpha-two, beta-one.
type groupingFixture struct {
	alpha *models.Project
	beta  *models.Project
	tasks []string
}

// groupingDueTask seeds an open task due dayOffset days from now.
func groupingDueTask(id string, dayOffset int) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	due := time.Now().AddDate(0, 0, dayOffset)
	task.DueDate = &due
	return task
}
