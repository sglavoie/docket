package tui

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/listsurface"
)

// projectHubPreviewPane returns the right pane of the Hub's rendered frame, so a
// test reads what the preview shows rather than the whole two-pane composition.
func projectHubPreviewPane(t *testing.T, hub *ProjectHubModel) []string {
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

func projectHubPreviewText(t *testing.T, hub *ProjectHubModel) string {
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

func TestProjectHubPreviewListsEveryTaskInTreeOrder(t *testing.T) {
	app := projectHubDiskApp(t)
	tree := projectHubDiskProject(app, "tree", "Tree")
	writeProjectHubTaskTree(t, tree)
	addTestAppProject(app, tree)

	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:project:tree")
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
	assertListSurfaceWidth(t, hub.View().Content, 120)
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
	focusProjectHubItem(t, hub, "project-hub:project:corrupt")
	if pane := projectHubPreviewText(t, hub); !strings.Contains(pane, "Tasks not loaded.") {
		t.Fatalf("unreadable project preview does not say so:\n%s", pane)
	}

	focusProjectHubItem(t, hub, "project-hub:project:hollow")
	pane := projectHubPreviewText(t, hub)
	if !strings.Contains(pane, "No tasks.") {
		t.Fatalf("empty project preview does not say so:\n%s", pane)
	}
	if strings.Contains(pane, "Tasks not loaded.") {
		t.Fatalf("empty project preview reads as unread:\n%s", pane)
	}
}

func TestProjectHubPreviewPagesWithCtrlFAndResetsOnAnotherProject(t *testing.T) {
	app := projectHubDiskApp(t)
	long := projectHubDiskProject(app, "long", "Long")
	// Several panes' worth, so a page down lands mid-list rather than straight on
	// the tail clamp.
	writeProjectHubTasks(t, long, 120)
	addTestAppProject(app, long)

	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:project:long")
	pane := projectHubPreviewText(t, hub)
	if !strings.Contains(pane, "↓ ") {
		t.Fatalf("a 120-task project fits the pane, so paging cannot be exercised:\n%s", pane)
	}
	page := hub.preview.pageSize
	if page <= 0 {
		t.Fatalf("render settled on page size %d, want the pane's row capacity", page)
	}

	hub.Update(syntheticCtrlKey('f'))
	pane = projectHubPreviewText(t, hub)
	if hub.preview.offset != page {
		t.Fatalf("Ctrl+F moved to offset %d, want a full page of %d", hub.preview.offset, page)
	}
	if !strings.Contains(pane, "↑ ") {
		t.Fatalf("paged pane carries no upper continuation:\n%s", pane)
	}

	hub.Update(syntheticCtrlKey('b'))
	projectHubPreviewText(t, hub)
	if hub.preview.offset != 0 {
		t.Fatalf("Ctrl+B left offset %d, want the top", hub.preview.offset)
	}

	// The tail clamps: pressing past the end stops on the last full pane rather
	// than scrolling into blank rows.
	for range 20 {
		hub.Update(syntheticCtrlKey('f'))
		projectHubPreviewText(t, hub)
	}
	pane = projectHubPreviewText(t, hub)
	if strings.Contains(pane, "↓ ") {
		t.Fatalf("paging past the tail left a lower continuation:\n%s", pane)
	}
	if !strings.Contains(pane, "Task 120") {
		t.Fatalf("tail of the pane omits the last task:\n%s", pane)
	}

	// Another project starts at its own top rather than inheriting the offset.
	focusProjectHubItem(t, hub, "project-hub:project:alpha")
	projectHubPreviewText(t, hub)
	if hub.preview.offset != 0 {
		t.Fatalf("moving to another project kept offset %d, want 0", hub.preview.offset)
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
		id    listsurface.ListItemID
		tasks []string
	}{
		{id: "project-hub:project:parked", tasks: []string{"Task 1", "Task 2"}},
		{id: "project-hub:archived:archived", tasks: []string{"Task 1", "Task 2", "Task 3"}},
	} {
		focusProjectHubItem(t, hub, project.id)
		pane := projectHubPreviewText(t, hub)
		for _, want := range project.tasks {
			if !strings.Contains(pane, want) {
				t.Fatalf("%s preview omitted %q:\n%s", project.id, want, pane)
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

	rows := projectPreviewTaskRows("proj", []*models.Task{root, orphan})
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want one per task", len(rows))
	}
	for index, want := range []string{"Root", "Orphan"} {
		if rows[index].task.Title != want {
			t.Fatalf("row %d = %q, want %q", index, rows[index].task.Title, want)
		}
		if rows[index].ProjectID != "proj" {
			t.Fatalf("row %d carries project %q, want %q", index, rows[index].ProjectID, "proj")
		}
	}
}
