package projecthub

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/util"
)

// A project row names its missed deadlines beside its open count, by the one
// definition the Outliner's project headers count with: a past deadline on open
// work, so a finished task with an old deadline is not overdue.
func TestProjectHubRowCountsOverdueTasks(t *testing.T) {
	past := util.StartOfDay(time.Now().AddDate(0, 0, -3))
	future := util.StartOfDay(time.Now().AddDate(0, 0, 5))
	alpha := scopeTestProject("alpha", "Alpha", "1", "2", "3", "4")
	alpha.Tasks[0].DueDate = &past
	alpha.Tasks[1].DueDate = &past
	alpha.Tasks[2].DueDate = &past
	alpha.Tasks[2].WorkflowState = models.StateDone
	alpha.Tasks[3].DueDate = &future
	fixture := newProjectHubFixture(t, alpha, scopeTestProject("beta", "Beta", "1"))

	hub := fixture.open()
	row := projectHubItem(t, hub, "project-hub:project:alpha")
	if got, want := row.SecondaryText, "3 open · 2 overdue · 4 tasks"; got != want {
		t.Fatalf("overdue row secondary = %q, want %q", got, want)
	}
	if row.SecondaryAccent != "2 overdue" || row.SecondaryAccentStyle == nil {
		t.Fatalf("overdue run is not painted: accent=%q style=%v", row.SecondaryAccent, row.SecondaryAccentStyle)
	}
	if got := projectHubPreviewSummary(t, hub, "project-hub:project:alpha"); !strings.Contains(got, "3 open · 2 overdue · 4 tasks") {
		t.Fatalf("preview meta %q disagrees with the row's tally", got)
	}

	quiet := projectHubItem(t, hub, "project-hub:project:beta")
	if quiet.SecondaryText != "1 open · 1 task" || quiet.SecondaryAccent != "" {
		t.Fatalf("a project with nothing overdue reads %q (accent %q)", quiet.SecondaryText, quiet.SecondaryAccent)
	}
}

// The pinned Hub has no Parked group, so a pinned parked project keeps the chip
// that is then the only place its parking is said.
func TestPinnedProjectHubKeepsTheParkedChip(t *testing.T) {
	parked := scopeTestProject("parked", "Parked", "1")
	parked.Pinned = true
	parked.Tags = []string{"parked"}
	hub := newProjectHubFixture(t, parked).open(route.ProjectHubModePinned)
	row := projectHubItem(t, hub, "project-hub:project:parked")
	if row.CategoryID != projectHubCategoryActive || !strings.Contains(row.SecondaryText, "#parked") {
		t.Fatalf("pinned parked row = category %q, secondary %q", row.CategoryID, row.SecondaryText)
	}
}
