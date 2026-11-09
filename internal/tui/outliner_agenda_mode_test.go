package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

// Agenda is All's head: every lane holding work already decided on or asking
// for attention — Pinned, Today, Doing, Review, Next — and none of the lanes of
// undecided work, which it counts beneath its rows instead.
func TestAgendaDrawsAllsCommittedLanesAndCountsTheRest(t *testing.T) {
	pinned := primaryRowsTask("pinned", models.StateBacklog)
	now := time.Now()
	pinned.Pinned, pinned.PinnedAt = true, &now
	today := primaryRowsTask("today", models.StateBacklog)
	doing := primaryRowsTask("doing", models.StateDoing)
	review := primaryRowsTask("review", models.StateWaiting)
	review.Review = true
	next := primaryRowsTask("next", models.StateBacklog)
	inbox := primaryRowsTask("inbox", models.StateInbox)
	backlog := primaryRowsTask("backlog", models.StateBacklog)
	waiting := primaryRowsTask("waiting", models.StateWaiting)
	project := primaryRowsProject("alpha", pinned, today, doing, review, next, inbox, backlog, waiting)
	outlinerView, runtime := primaryRowsOutliner(project)
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: today.ID}})
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: next.ID}})

	outlinerView.SelectOutlinerMode(chrome.ModeAgenda)
	var sections, tasks []string
	for _, row := range outlinerView.Rows {
		if row.IsSectionHeader {
			sections = append(sections, row.SectionName)
		}
		if row.Task != nil {
			tasks = append(tasks, row.Task.ID)
		}
	}
	if want := []string{"Pinned", "Today", "Doing", "Review", "Next"}; !slices.Equal(sections, want) {
		t.Fatalf("Agenda sections = %v, want %v", sections, want)
	}
	if want := []string{"pinned", "today", "doing", "review", "next"}; !slices.Equal(tasks, want) {
		t.Fatalf("Agenda tasks = %v, want %v", tasks, want)
	}
	outlinerView.SetSize(120, 40)
	line := ansi.Strip(outlinerView.View().Content)
	if !strings.Contains(line, "Not on the agenda: Inbox 1 · Backlog 1 · Waiting 1") {
		t.Fatalf("Agenda counted no left-out lanes:\n%s", line)
	}

	outlinerView.SelectOutlinerMode(chrome.ModeAll)
	if screen := ansi.Strip(outlinerView.View().Content); strings.Contains(screen, "Not on the agenda") {
		t.Fatalf("All drew Agenda's left-out line:\n%s", screen)
	}
}
