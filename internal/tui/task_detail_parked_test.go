package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskDetailShowsParkedBanner(t *testing.T) {
	project := testutil.NewTestProject("Launch")
	task := testutil.NewTestTask("Finish launch checklist", models.TaskTypeTask)
	parkedAt := time.Now().Add(-2 * time.Hour)
	task.ParkedNote = "blocked on API review"
	task.ParkedAt = &parkedAt
	project.Tasks = []*models.Task{task}

	m := &TaskDetailModel{task: task, project: project}
	plain := ansi.Strip(strings.Join(m.overviewSections(90), "\n"))

	want := "⏸ Parked 2 hours ago — blocked on API review"
	if !strings.Contains(plain, want) {
		t.Fatalf("parked banner missing %q\n\n%s", want, plain)
	}
}
