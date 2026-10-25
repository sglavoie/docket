package tui

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// pinSpec seeds one pinned task: its title doubles as the assertion label and
// pinnedAt fixes its rank in the PinnedAt-descending Pinned order.
type pinSpec struct {
	title    string
	pinnedAt time.Time
	parent   string
}

// pinnedOrderProject seeds one on-disk project whose tasks are all pinned, so a
// J/K reorder test can inspect the committed pin order.
func pinnedOrderProject(t *testing.T, id, slug string, specs ...pinSpec) (*models.Project, map[string]*models.Task) {
	t.Helper()
	project, err := models.NewProject(id, slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = slug
	byTitle := make(map[string]*models.Task, len(specs))
	tasks := make([]*models.Task, len(specs))
	for i, spec := range specs {
		task := seedTask(t, i+1, spec.title, models.StateBacklog)
		pinnedAt := spec.pinnedAt
		task.Pinned, task.PinnedAt = true, &pinnedAt
		if spec.parent != "" {
			parent, ok := byTitle[spec.parent]
			if !ok {
				t.Fatalf("pinSpec %q names an unseeded parent %q", spec.title, spec.parent)
			}
			task.ParentID = &parent.ID
		}
		tasks[i] = task
		byTitle[spec.title] = task
	}
	project.Tasks = tasks
	project.NextShortID = len(specs) + 1
	project.BuildTaskTree()
	return project, byTitle
}

// diskPinOrder reloads the project and returns its task titles newest pin
// first, which is exactly the order a restart would render Pinned in.
func diskPinOrder(t *testing.T, project *models.Project) []string {
	t.Helper()
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project: %v", err)
	}
	pinned := make([]*models.Task, 0, len(fresh.Tasks))
	for _, task := range fresh.Tasks {
		if task.Pinned {
			pinned = append(pinned, task)
		}
	}
	sort.SliceStable(pinned, func(i, j int) bool {
		return pinned[i].PinnedAt.After(*pinned[j].PinnedAt)
	})
	titles := make([]string, 0, len(pinned))
	for _, task := range pinned {
		titles = append(titles, task.Title)
	}
	return titles
}

// visibleTaskTitles returns the rendered task rows' titles in render order.
func visibleTaskTitles(m *outliner.OutlinerModel) []string {
	var titles []string
	for _, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil {
			titles = append(titles, row.Task.Title)
		}
	}
	return titles
}

func TestOutlinerPinnedOrderJK(t *testing.T) {
	base := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)

	t.Run("J swaps two pins and the new order survives a reload", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-pins", "pins",
			pinSpec{title: "Alpha", pinnedAt: base},
			pinSpec{title: "Bravo", pinnedAt: base.Add(-time.Hour)},
			pinSpec{title: "Charlie", pinnedAt: base.Add(-2 * time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["Alpha"].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J on a pinned row produced no reorder command")
		}

		assertOrder(t, "pinned rows", visibleTaskTitles(cross), []string{"Bravo", "Alpha", "Charlie"})
		assertOrder(t, "committed pin", diskPinOrder(t, project), []string{"Bravo", "Alpha", "Charlie"})
		if task, _ := cross.GetTaskRef(); task == nil || task.Title != "Alpha" {
			t.Fatalf("cursor after reorder = %v, want Alpha", task)
		}
		if !strings.Contains(m.feedback.message, "Reorder Pinned") || !strings.Contains(m.feedback.message, "u undo") {
			t.Fatalf("status = %q, want the pinned reorder confirmation with its undo affordance", m.feedback.message)
		}
	})

	t.Run("equal pin times still move in the pressed direction", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-tie", "tie",
			pinSpec{title: "Alpha", pinnedAt: base},
			pinSpec{title: "Bravo", pinnedAt: base},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["Alpha"].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J on the tied pair produced no reorder command")
		}
		assertOrder(t, "pinned rows after J", visibleTaskTitles(cross), []string{"Bravo", "Alpha"})

		if !pressReorderKey(t, m, 'K') {
			t.Fatal("K on the tied pair produced no reorder command")
		}
		assertOrder(t, "pinned rows after K", visibleTaskTitles(cross), []string{"Alpha", "Bravo"})
		assertOrder(t, "committed pin", diskPinOrder(t, project), []string{"Alpha", "Bravo"})
	})

	t.Run("Pinned reorders while durably ordered Today mode is active", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-today", "today",
			pinSpec{title: "Alpha", pinnedAt: base},
			pinSpec{title: "Bravo", pinnedAt: base.Add(-time.Hour)},
		)
		planned := seedTask(t, 3, "Planned", models.StateBacklog)
		project.Tasks = append(project.Tasks, planned)
		project.NextShortID = 4
		project.BuildTaskTree()

		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		seedTodayOrder(t, m,
			config.ProjectTaskLocator{ProjectID: project.ID, TaskID: tasks["Alpha"].ID},
			config.ProjectTaskLocator{ProjectID: project.ID, TaskID: tasks["Bravo"].ID},
			config.ProjectTaskLocator{ProjectID: project.ID, TaskID: planned.ID},
		)
		enterCrossMode(cross, chrome.ModeToday)
		if !cross.MoveCursorToProjectTask(tasks["Alpha"].ID, project.ID) {
			t.Fatal("cursor did not land on Alpha in Today mode")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J on a pinned row in Today mode produced no reorder command")
		}
		assertOrder(t, "Today mode rows", visibleTaskTitles(cross), []string{"Bravo", "Alpha", "Planned"})
	})

	t.Run("J stops at the project group boundary", func(t *testing.T) {
		alpha, alphaTasks := pinnedOrderProject(t, "p-alpha", "alpha",
			pinSpec{title: "Alpha One", pinnedAt: base},
			pinSpec{title: "Alpha Two", pinnedAt: base.Add(-time.Hour)},
		)
		beta, _ := pinnedOrderProject(t, "p-beta", "beta",
			pinSpec{title: "Beta One", pinnedAt: base.Add(-2 * time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, alpha, beta)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(alphaTasks["Alpha Two"].ID, alpha.ID) {
			t.Fatal("cursor did not land on Alpha Two")
		}

		if pressReorderKey(t, m, 'J') {
			t.Fatal("J crossed the Pinned project group boundary")
		}
		assertOrder(t, "pinned rows", visibleTaskTitles(cross),
			[]string{"Alpha One", "Alpha Two", "Beta One"})
		assertOrder(t, "committed pin", diskPinOrder(t, alpha), []string{"Alpha One", "Alpha Two"})
	})

	t.Run("J on a nested pin swaps only its siblings", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-nest", "nest",
			pinSpec{title: "Parent", pinnedAt: base},
			pinSpec{title: "First Child", pinnedAt: base.Add(-time.Hour), parent: "Parent"},
			pinSpec{title: "Second Child", pinnedAt: base.Add(-2 * time.Hour), parent: "Parent"},
			pinSpec{title: "Other Root", pinnedAt: base.Add(-3 * time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["First Child"].ID, project.ID) {
			t.Fatal("cursor did not land on First Child")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J on a nested pin produced no reorder command")
		}
		assertOrder(t, "pinned rows", visibleTaskTitles(cross),
			[]string{"Parent", "Second Child", "First Child", "Other Root"})
	})

	t.Run("J on a root moves its whole subtree past the next root", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-subtree", "subtree",
			pinSpec{title: "Parent", pinnedAt: base},
			pinSpec{title: "Child", pinnedAt: base.Add(-time.Hour), parent: "Parent"},
			pinSpec{title: "Other Root", pinnedAt: base.Add(-2 * time.Hour)},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		if !cross.MoveCursorToProjectTask(tasks["Parent"].ID, project.ID) {
			t.Fatal("cursor did not land on Parent")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J on a pinned root produced no reorder command")
		}
		assertOrder(t, "pinned rows", visibleTaskTitles(cross),
			[]string{"Other Root", "Parent", "Child"})
	})

	t.Run("a collapsed subtree is one J target, not its hidden child", func(t *testing.T) {
		project, tasks := pinnedOrderProject(t, "p-collapsed", "collapsed",
			pinSpec{title: "Other Root", pinnedAt: base},
			pinSpec{title: "Parent", pinnedAt: base.Add(-time.Hour)},
			pinSpec{title: "Child", pinnedAt: base.Add(-2 * time.Hour), parent: "Parent"},
		)
		m, cross, _ := newPlanningOrderApp(t, project)
		cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		enterCrossMode(cross, chrome.ModeAll)
		cross.Folds.ExpandedNodes[taskref.TaskIdentity{ProjectID: project.ID, TaskID: tasks["Parent"].ID}] = false
		cross.Population.Invalidate()
		cross.CrossProjectRebuildRows()
		if got := visibleTaskTitles(cross); !slices.Equal(got, []string{"Other Root", "Parent"}) {
			t.Fatalf("collapsed pinned rows = %v, want the child hidden", got)
		}
		if !cross.MoveCursorToProjectTask(tasks["Other Root"].ID, project.ID) {
			t.Fatal("cursor did not land on Other Root")
		}

		if !pressReorderKey(t, m, 'J') {
			t.Fatal("J above a collapsed pinned subtree produced no reorder command")
		}
		assertOrder(t, "collapsed pinned rows", visibleTaskTitles(cross), []string{"Parent", "Other Root"})
		assertOrder(t, "committed pin", diskPinOrder(t, project),
			[]string{"Parent", "Other Root", "Child"})
	})
}

// TestPinnedReorderIsUndoable proves the swap rides one ledger transaction, so
// the u affordance the status advertises restores the previous pin order.
func TestPinnedReorderIsUndoable(t *testing.T) {
	base := time.Date(2026, 8, 7, 9, 0, 0, 0, time.UTC)
	project, tasks := pinnedOrderProject(t, "p-undo", "undo",
		pinSpec{title: "Alpha", pinnedAt: base},
		pinSpec{title: "Bravo", pinnedAt: base.Add(-time.Hour)},
	)
	m, cross, dataDir := newPlanningOrderApp(t, project)
	cross.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
	enterCrossMode(cross, chrome.ModeAll)
	if !cross.MoveCursorToProjectTask(tasks["Alpha"].ID, project.ID) {
		t.Fatal("cursor did not land on Alpha")
	}
	if !pressReorderKey(t, m, 'J') {
		t.Fatal("J on a pinned row produced no reorder command")
	}
	assertOrder(t, "committed pin", diskPinOrder(t, project), []string{"Bravo", "Alpha"})

	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	assertOrder(t, "pin after undo", diskPinOrder(t, project), []string{"Alpha", "Bravo"})
}
