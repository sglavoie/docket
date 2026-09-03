package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// TestMoveToProjectListsEveryAvailableProjectButTheCurrentOne pins the picker's
// population: AvailableProjects rather than the base task workspace, so a parked
// project stays a destination, and never the project the task already lives in.
func TestMoveToProjectListsEveryAvailableProjectButTheCurrentOne(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureParked)
	_, project := taskContentFixtureTask(t, m, "Ship the delivery checklist")

	primary := make(map[string]ListItem)
	for _, item := range operationProjectListSource(m.runtime.AvailableProjects(), project.ID).Items {
		primary[item.PrimaryText] = item
	}

	parked := taskContentFixtureProject(t, m, parkedFixtureParkedName)
	row, ok := primary[parked.Name]
	if !ok {
		t.Fatalf("Move to Project omits the parked project %q", parked.Name)
	}
	if !row.Enabled() {
		t.Errorf("parked destination is disabled: %s", row.DisabledReason)
	}
	if row.Data != parked.ID {
		t.Errorf("row payload = %v, want the project ID %q", row.Data, parked.ID)
	}
	if _, offered := primary[project.Name]; offered {
		t.Errorf("Move to Project offers %q, the project the task already lives in", project.Name)
	}
	if _, offered := primary[config.SystemUnfiledName]; !offered {
		t.Error("Move to Project omits the system Inbox as a destination")
	}
}

// TestMoveToProjectReassignsShortIDsAcrossTheWholeSubtree is the review
// checkpoint for the move: the destination mints its own project-local ShortIDs
// for every relocated task rather than carrying the source numbering into a
// collision, and the subtree keeps its shape under the destination's root.
func TestMoveToProjectReassignsShortIDsAcrossTheWholeSubtree(t *testing.T) {
	m := taskContentFixtureApp(t, FixtureMultiProject)
	source, sourceProject := taskContentFixtureTask(t, m, "Alpha migration kickoff")
	destination := taskContentFixtureProject(t, m, "Client Beta")

	child := newFixtureTask(sourceProject, "Alpha migration cutover")
	child.ParentID = &source.ID
	sourceProject.BuildTaskTree()
	if err := storage.SaveProject(sourceProject); err != nil {
		t.Fatalf("seed the child task: %v", err)
	}

	resident := taskShortIDsByTitle(destination)["Beta rollout checklist"]
	if resident == 0 {
		t.Fatal("the destination fixture task has no ShortID to collide with")
	}

	cmd := m.moveTaskToProjectCmd([]taskref.TaskIdentity{taskref.For(source, sourceProject)}, destination.ID, nil)
	if cmd == nil {
		t.Fatal("the move produced no command")
	}
	msg, ok := cmd().(coordinator.TaskLineEditedMsg)
	if !ok {
		t.Fatalf("the move produced %T, want an edit result", cmd())
	}
	if !msg.Result.Changed {
		t.Fatal("a cross-project move reported no change")
	}
	if msg.Result.MovedDescendantCount != 1 {
		t.Errorf("moved descendant count = %d, want 1", msg.Result.MovedDescendantCount)
	}

	m.reloadProjectByID(sourceProject.ID)
	m.reloadProjectByID(destination.ID)
	reloadedSource := m.projectByID(sourceProject.ID)
	reloadedDestination := m.projectByID(destination.ID)

	if len(reloadedSource.Tasks) != 0 {
		t.Errorf("the source project kept %d task(s) after the move", len(reloadedSource.Tasks))
	}

	shortIDs := taskShortIDsByTitle(reloadedDestination)
	if len(shortIDs) != 3 {
		t.Fatalf("destination task ShortIDs = %v, want three tasks", shortIDs)
	}
	seen := make(map[int]string, len(shortIDs))
	for title, shortID := range shortIDs {
		if shortID <= 0 {
			t.Errorf("task %q landed with ShortID %d", title, shortID)
		}
		if other, clash := seen[shortID]; clash {
			t.Errorf("tasks %q and %q share ShortID %d", other, title, shortID)
		}
		seen[shortID] = title
	}
	if got := shortIDs["Beta rollout checklist"]; got != resident {
		t.Errorf("the resident task's ShortID = %d, want its original %d", got, resident)
	}
	highest := max(shortIDs["Alpha migration kickoff"], shortIDs["Alpha migration cutover"])
	if reloadedDestination.NextShortID <= highest {
		t.Errorf(
			"destination NextShortID = %d, want it past the highest moved ShortID %d",
			reloadedDestination.NextShortID,
			highest,
		)
	}

	movedRoot := reloadedDestination.GetTaskByID(source.ID)
	movedChild := reloadedDestination.GetTaskByID(child.ID)
	if movedRoot == nil || movedChild == nil {
		t.Fatal("the destination is missing part of the moved subtree")
	}
	if movedRoot.ParentID != nil {
		t.Errorf("moved root parent = %v, want the destination root", movedRoot.ParentID)
	}
	if movedChild.ParentID == nil || *movedChild.ParentID != movedRoot.ID {
		t.Errorf("moved child parent = %v, want the moved root", movedChild.ParentID)
	}
}

func taskShortIDsByTitle(project *models.Project) map[string]int {
	shortIDs := make(map[string]int, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil {
			shortIDs[task.Title] = task.ShortID
		}
	}
	return shortIDs
}
