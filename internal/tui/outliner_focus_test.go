package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerCoreFocusesMapToPerspectives(t *testing.T) {
	tests := []struct {
		focus       OutlinerFocus
		perspective int
	}{
		{OutlinerActive, PerspectiveActive},
		{OutlinerAll, PerspectiveAll},
		{OutlinerInbox, PerspectiveInbox},
		{OutlinerNext, PerspectiveNext},
		{OutlinerStandby, PerspectiveStandby},
		{OutlinerWaiting, PerspectiveWaiting},
		{OutlinerReview, PerspectiveReview},
		{OutlinerSomeday, PerspectiveSomeday},
		{OutlinerCompleted, PerspectiveCompleted},
		{OutlinerSnoozed, PerspectiveSnoozed},
	}
	for _, test := range tests {
		got, ok := test.focus.perspectiveID()
		if !ok || got != test.perspective {
			t.Errorf("%s perspective = %d/%v, want %d", test.focus, got, ok, test.perspective)
		}
	}
}

func TestOutlinerSmartFocusPredicates(t *testing.T) {
	now := time.Now()
	blocker := &models.Task{ID: "blocker", WorkflowState: models.StateReady}
	task := &models.Task{
		ID:            "task",
		WorkflowState: models.StateReady,
		Priority:      2,
		Pinned:        true,
		ParkedAt:      &now,
		DependsOn:     []string{blocker.ID},
	}
	due := now.AddDate(0, 0, 3)
	task.DueDate = &due
	project := &models.Project{ID: "project", Tasks: []*models.Task{blocker, task}}
	project.BuildTaskTree()
	runtime := newTestConfig()
	runtime.Machine().SetRecentTaskVisits([]config.RecentTaskVisit{{ProjectID: project.ID, TaskID: task.ID}})
	outliner := NewOutlinerModel(project, nil, runtime)

	for _, focus := range []OutlinerFocus{
		OutlinerDueSoon,
		OutlinerHighPriority,
		OutlinerBlocked,
		OutlinerParked,
		OutlinerPinned,
		OutlinerRecent,
	} {
		if !outliner.matchesSmartFocus(task, project, focus, now) {
			t.Errorf("task did not match %s", focus)
		}
	}

	task.WorkflowState = models.StateDone
	for _, focus := range []OutlinerFocus{OutlinerDueSoon, OutlinerHighPriority, OutlinerBlocked, OutlinerParked, OutlinerPinned} {
		if outliner.matchesSmartFocus(task, project, focus, now) {
			t.Errorf("terminal task unexpectedly matched %s", focus)
		}
	}
}

func TestOutlinerFocusOrderingPreservesHierarchy(t *testing.T) {
	child := &models.Task{ID: "child"}
	parent := &models.Task{ID: "parent", Children: []*models.Task{child}}
	first := &models.Task{ID: "first"}
	unranked := &models.Task{ID: "unranked"}

	ordered := sortTaskTreeByRank(
		[]*models.Task{unranked, parent, first},
		map[string]int{first.ID: 0, child.ID: 1},
	)
	if len(ordered) != 3 || ordered[0].ID != first.ID || ordered[1].ID != parent.ID || ordered[2].ID != unranked.ID {
		t.Fatalf("ordered roots = %v, want first, parent, unranked", taskIDs(ordered))
	}
	if len(ordered[1].Children) != 1 || ordered[1].Children[0].ID != child.ID {
		t.Fatalf("hierarchy was not preserved: %+v", ordered[1].Children)
	}
}

func TestOutlinerOpenOptionsReplaceCrossPriorityLens(t *testing.T) {
	m := minimalOutliner()
	m.crossPriorityMode = true
	m.ApplyOpenOptions(OutlinerOpenOptions{
		Focus: OutlinerHighPriority, SetFocus: true,
		Sort: SortByScore, SetSort: true,
	})
	if m.crossPriorityMode {
		t.Fatal("opening an explicit focus left the cross-project priority lens active")
	}
	if m.smartFocus == nil || *m.smartFocus != OutlinerHighPriority || m.sort.Field != SortByScore {
		t.Fatalf("open options = focus %v sort %v", m.smartFocus, m.sort.Field)
	}
}

func TestOutlinerOpenOptionsPreserveLensUnlessIntentIsExplicit(t *testing.T) {
	m := minimalOutliner()
	m.lens = OutlinerLensDueDate
	m.ApplyOpenOptions(OutlinerOpenOptions{})
	if m.lens != OutlinerLensDueDate {
		t.Fatalf("generic navigation changed lens to %v", m.lens)
	}

	m.ApplyOpenOptions(OutlinerOpenOptions{Lens: OutlinerLensTree, SetLens: true})
	if m.lens != OutlinerLensTree {
		t.Fatalf("explicit tree intent left lens at %v", m.lens)
	}
}

func TestOutlinerOpenOptionsReconcilesActiveAndTreeSorts(t *testing.T) {
	m := NewOutlinerModel(nil, nil, newTestConfig())
	m.selectOutlinerMode(ModeAll)
	m.toggleSort(SortByPriority)

	// The command shape used by "all tasks by due date" resolves to the fixed
	// Due Date target. Its SetSort is not allowed to overwrite the tree sort.
	m.ApplyOpenOptions(OutlinerOpenOptions{
		Focus: OutlinerAll, SetFocus: true,
		Sort: SortByDueDate, SetSort: true,
		Lens: OutlinerLensDueDate, SetLens: true,
	})
	if got := m.activeOutlinerMode(); got != ModeDueDateLens {
		t.Fatalf("command target = %q, want Due Date", got)
	}
	if m.sort != NewSortState() || m.treeSort.Field != SortByPriority {
		t.Fatalf("Due Date command sorts = active %+v tree %+v", m.sort, m.treeSort)
	}

	// Command-driven focus changes use catalog selection and restore the stored
	// tree ordering.
	m.ApplyOpenOptions(OutlinerOpenOptions{Focus: OutlinerInbox, SetFocus: true})
	if got := m.activeOutlinerMode(); got != ModeInbox {
		t.Fatalf("focus target = %q, want Inbox", got)
	}
	if m.sort.Field != SortByPriority || m.treeSort != m.sort {
		t.Fatalf("focus restore = active %+v tree %+v", m.sort, m.treeSort)
	}

	// Explicit sort intent on a tree target updates both copies atomically.
	m.ApplyOpenOptions(OutlinerOpenOptions{Sort: SortByScore, SetSort: true})
	want := SortState{Field: SortByScore, Ascending: true}
	if m.sort != want || m.treeSort != want {
		t.Fatalf("explicit tree sort = active %+v tree %+v, want %+v", m.sort, m.treeSort, want)
	}
}

func TestOpenAtTaskRevealsTargetHiddenByFocusAndSearch(t *testing.T) {
	active := &models.Task{ID: "active", Title: "visible active", WorkflowState: models.StateReady}
	completed := &models.Task{ID: "completed", Title: "hidden completed", WorkflowState: models.StateDone}
	project := &models.Project{ID: "project", Tasks: []*models.Task{active, completed}}
	project.BuildTaskTree()
	m := NewOutlinerModel(project, nil, newTestConfig())
	m.SetSearchQuery("visible")
	m.OpenAtTask = completed
	m.rebuildRows()

	if m.OpenAtTask != nil {
		t.Fatal("pending OpenAtTask was not consumed")
	}
	if m.activePerspective != PerspectiveCompleted || m.SearchQuery() != "" {
		t.Fatalf("reveal state = perspective %d search %q", m.activePerspective, m.SearchQuery())
	}
	if task := m.getCursorTask(); task == nil || task.ID != completed.ID {
		t.Fatalf("cursor task = %v, want completed target", task)
	}
}

func taskIDs(tasks []*models.Task) []string {
	ids := make([]string, len(tasks))
	for index, task := range tasks {
		ids[index] = task.ID
	}
	return ids
}
