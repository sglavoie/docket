package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/projectops"
)

// The pane's own scroll arithmetic moved into internal/tui/projecthub with the
// preview. What stays here is what the pane draws through the Outliner's row
// painter, which the Hub is handed rather than reaching for.

// projectHubPreviewPane returns the right pane of the Hub's rendered frame, so a
// test reads what the preview shows rather than the whole two-pane composition.
func projectHubPreviewPane(t *testing.T, hub *projecthub.ProjectHubModel) []string {
	t.Helper()
	var pane []string
	for _, line := range strings.Split(hub.View().Content, "\n") {
		stripped := ansi.Strip(line)
		index := strings.Index(stripped, " │ ")
		if index < 0 {
			continue
		}
		pane = append(pane, strings.TrimRight(stripped[index+len(" │ "):], " "))
	}
	if len(pane) == 0 {
		t.Fatalf("Hub frame carries no preview pane:\n%s", hub.View().Content)
	}
	return pane
}

func projectHubPreviewText(t *testing.T, hub *projecthub.ProjectHubModel) string {
	t.Helper()
	return strings.Join(projectHubPreviewPane(t, hub), "\n")
}

// writeProjectHubTaskTree writes a parent holding two children beside a lone
// sibling, so the pane has a shape a flat list would lose.
func writeProjectHubTaskTree(t *testing.T, project *models.Project) {
	t.Helper()
	titles := []string{"Root one", "Child one", "Child two", "Root two"}
	for index, title := range titles {
		task, err := models.NewGenericTask(fmt.Sprintf("%s-%d", project.ID, index+1), index+1, title)
		if err != nil {
			t.Fatalf("NewGenericTask: %v", err)
		}
		task.Project = project
		project.Tasks = append(project.Tasks, task)
	}
	parentID := project.Tasks[0].ID
	project.Tasks[1].ParentID = &parentID
	project.Tasks[2].ParentID = &parentID
	project.NextShortID = len(titles) + 1
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%q): %v", project.ID, err)
	}
}

func TestProjectHubPreviewListsOpenTasksInTreeOrder(t *testing.T) {
	app := projectHubDiskApp(t)
	tree := projectHubDiskProject(app, "tree", "Tree")
	writeProjectHubTaskTree(t, tree)
	addTestAppProject(app, tree)

	hub := openProjectHub(t, app)
	focusProjectHubRow(t, hub, "Tree")
	pane := projectHubPreviewText(t, hub)

	// Header first: name, then the one meta line, and no labeled fields at all.
	if !strings.HasPrefix(pane, "Tree\nActive · 4 open · 4 tasks · slug tree\n") {
		t.Fatalf("preview header is not the two-row block:\n%s", pane)
	}
	for _, gone := range []string{"Lifecycle:", "Tasks:", "Open:", "Pinned:", "Slug:"} {
		if strings.Contains(pane, gone) {
			t.Errorf("preview still carries the labeled field %q:\n%s", gone, pane)
		}
	}

	// Then one row per task, in tree order, children under their parent.
	order := []string{"Root one", "Child one", "Child two", "Root two"}
	previous := -1
	for _, title := range order {
		index := strings.Index(pane, title)
		if index < 0 {
			t.Fatalf("preview omitted task %q:\n%s", title, pane)
		}
		if index < previous {
			t.Fatalf("preview lists %q out of tree order:\n%s", title, pane)
		}
		previous = index
	}
	for _, want := range []string{"│   ├── ", "│   └── ", "└── "} {
		if !strings.Contains(pane, want) {
			t.Errorf("preview dropped the %q connector:\n%s", want, pane)
		}
	}
	assertProjectHubFitsWidth(t, hub.View().Content, 120)
}

func TestProjectHubPreviewNamesAnUnreadProjectRatherThanShowingItEmpty(t *testing.T) {
	app := projectHubDiskApp(t)
	corrupt := projectHubDiskProject(app, "corrupt", "Corrupt")
	if err := os.MkdirAll(corrupt.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", corrupt.Path, err)
	}
	tasksPath := storage.TasksFilePath(corrupt.Path)
	if err := os.WriteFile(tasksPath, []byte("this is not a task document\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", tasksPath, err)
	}
	addTestAppProject(app, corrupt)
	empty := projectHubDiskProject(app, "hollow", "Hollow")
	if err := os.MkdirAll(empty.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", empty.Path, err)
	}
	addTestAppProject(app, empty)

	hub := openProjectHub(t, app)
	focusProjectHubRow(t, hub, "Corrupt")
	if pane := projectHubPreviewText(t, hub); !strings.Contains(pane, "Tasks not loaded.") {
		t.Fatalf("unreadable project preview does not say so:\n%s", pane)
	}

	focusProjectHubRow(t, hub, "Hollow")
	pane := projectHubPreviewText(t, hub)
	if !strings.Contains(pane, "No tasks.") {
		t.Fatalf("empty project preview does not say so:\n%s", pane)
	}
	if strings.Contains(pane, "Tasks not loaded.") {
		t.Fatalf("empty project preview reads as unread:\n%s", pane)
	}
}

func TestProjectHubPreviewListsTasksOutsideTheBaseTaskWorkspace(t *testing.T) {
	app := projectHubDiskApp(t)
	parked := projectHubDiskProject(app, "parked", "Parked")
	parked.Tags = []string{"parked"}
	writeProjectHubTasks(t, parked, 2)
	addTestAppProject(app, parked)
	archived := projectHubDiskProject(app, "archived", "Archived")
	archived.Archived = true
	writeProjectHubTasks(t, archived, 3)
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	// Both reach the Hub as catalog records holding no tasks of their own, so the
	// pane only fills if the tally retained the tree the count pass read.
	for _, project := range []struct {
		name  string
		tasks []string
	}{
		{name: "Parked", tasks: []string{"Task 1", "Task 2"}},
		{name: "Archived", tasks: []string{"Task 1", "Task 2", "Task 3"}},
	} {
		focusProjectHubRow(t, hub, project.name)
		pane := projectHubPreviewText(t, hub)
		for _, want := range project.tasks {
			if !strings.Contains(pane, want) {
				t.Fatalf("%s preview omitted %q:\n%s", project.name, want, pane)
			}
		}
	}
}

func TestProjectPreviewTaskRowsListsAnOrphanExactlyOnce(t *testing.T) {
	t.Parallel()

	root := &models.Task{ID: "root", ShortID: 1, Title: "Root", WorkflowState: models.StateBacklog}
	orphan := &models.Task{ID: "orphan", ShortID: 2, Title: "Orphan", WorkflowState: models.StateBacklog}
	// A parent reference no task in the project answers: reading roots as
	// "reports no parent" would drop this row entirely.
	missing := "gone"
	orphan.ParentID = &missing

	rows, settled := projectops.PreviewTaskRows("proj", []*models.Task{root, orphan})
	if !settled.Empty() {
		t.Fatalf("settled = %+v, want none for two open tasks", settled)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want one per task", len(rows))
	}
	for index, want := range []string{"Root", "Orphan"} {
		if rows[index].Task.Title != want {
			t.Fatalf("row %d = %q, want %q", index, rows[index].Task.Title, want)
		}
		if rows[index].ProjectID != "proj" {
			t.Fatalf("row %d carries project %q, want %q", index, rows[index].ProjectID, "proj")
		}
	}
}

func TestProjectPreviewTaskRowsFoldsTerminalTasksIntoOneSummary(t *testing.T) {
	t.Parallel()

	parent := &models.Task{ID: "parent", ShortID: 1, Title: "Parent", WorkflowState: models.StateDoing}
	open := &models.Task{ID: "open", ShortID: 2, Title: "Open child", WorkflowState: models.StateBacklog}
	finished := &models.Task{ID: "finished", ShortID: 3, Title: "Finished child", WorkflowState: models.StateDone}
	dropped := &models.Task{ID: "dropped", ShortID: 4, Title: "Dropped", WorkflowState: models.StateCanceled}
	shipped := &models.Task{ID: "shipped", ShortID: 5, Title: "Shipped", WorkflowState: models.StateDone}
	parentID := parent.ID
	open.ParentID = &parentID
	finished.ParentID = &parentID
	// The finished child comes last, so the open one must still close the
	// sibling run: connectors are measured over the open siblings alone.
	parent.Children = []*models.Task{open, finished}

	rows, settled := projectops.PreviewTaskRows("proj", []*models.Task{parent, open, finished, dropped, shipped})
	if got, want := settled, (projectops.PreviewSettledTally{Done: 2, Canceled: 1}); got != want {
		t.Fatalf("settled = %+v, want %+v", got, want)
	}
	if got, want := settled.Label(), "✓ 2 done · 1 canceled"; got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}
	if got, want := (projectops.PreviewSettledTally{Done: 3}).Label(), "✓ 3 done"; got != want {
		t.Fatalf("done-only label = %q, want %q", got, want)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want the two open tasks", len(rows))
	}
	if rows[0].Task.Title != "Parent" || rows[1].Task.Title != "Open child" {
		t.Fatalf("rows = %q, %q; want Parent then Open child", rows[0].Task.Title, rows[1].Task.Title)
	}
	if got, want := rows[1].Prefix, outliner.BuildTreePrefix([]bool{true}, true); got != want {
		t.Fatalf("open child prefix = %q, want the last-sibling connector %q", got, want)
	}
}

func TestProjectHubPreviewSummarizesSettledWorkInOneRow(t *testing.T) {
	app := projectHubDiskApp(t)
	tree := projectHubDiskProject(app, "tree", "Tree")
	writeProjectHubTaskTree(t, tree)
	tree.Tasks[3].WorkflowState = models.StateDone
	tree.Tasks[3].StateChangedAt = tree.Tasks[3].UpdatedAt
	if err := storage.SaveProject(tree); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	addTestAppProject(app, tree)

	hub := openProjectHub(t, app)
	focusProjectHubRow(t, hub, "Tree")
	pane := projectHubPreviewText(t, hub)
	if strings.Contains(pane, "Root two") {
		t.Fatalf("preview still lists the done task:\n%s", pane)
	}
	if !strings.Contains(pane, "✓ 1 done") {
		t.Fatalf("preview has no settled summary row:\n%s", pane)
	}
	assertProjectHubFitsWidth(t, hub.View().Content, 120)
}

// assertProjectHubFitsWidth is the Hub's own copy of the overflow assertion the
// list-surface render tests make. It stayed behind when those tests followed
// their subject into listsurface: this is the only shell caller, so per the
// extraction recipe it moves into its consumer rather than crossing the
// boundary as exported test API.
func assertProjectHubFitsWidth(t *testing.T, rendered string, width int) {
	t.Helper()
	for index, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line %d width = %d, want <= %d: %q", index, got, width, line)
		}
	}
}
