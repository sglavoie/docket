package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskDetailShowsParkedInTaskCard(t *testing.T) {
	project := testutil.NewTestProject("Launch")
	task := testutil.NewTestTask("Finish launch checklist", models.TaskTypeTask)
	parkedAt := time.Now().Add(-2 * time.Hour)
	task.ParkedNote = "blocked on API review"
	task.ParkedAt = &parkedAt
	task.Description = "Checklist description body"
	project.Tasks = []*models.Task{task}

	m := &TaskDetailModel{task: task, project: project}

	want := "⏸ Parked 2 hours ago — blocked on API review"

	// The parked line must live at the end of the Task (header) card, not as a
	// standalone banner between the Task and Description cards.
	header := ansi.Strip(m.renderHeaderSection(90))
	if !strings.Contains(header, want) {
		t.Fatalf("parked line missing from Task card %q\n\n%s", want, header)
	}

	sections := m.overviewSections(90)
	headerIdx, descIdx := -1, -1
	for i, section := range sections {
		plain := ansi.Strip(section)
		if strings.Contains(plain, want) {
			if strings.Contains(plain, task.Title) {
				headerIdx = i
			} else {
				t.Fatalf("parked line appears in a non-header section:\n%s", plain)
			}
		}
		if strings.Contains(plain, "Checklist description body") {
			descIdx = i
		}
	}
	if headerIdx == -1 {
		t.Fatal("parked line should be present in the Task card section")
	}
	if descIdx == -1 {
		t.Fatal("description section should be present")
	}
	if headerIdx != 0 {
		t.Fatalf("Task card should be the first section, got index %d", headerIdx)
	}
}
