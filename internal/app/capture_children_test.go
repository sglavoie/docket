package app_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

// nestTask makes child a stored child of parent without rebuilding the tree, so
// a fixture can describe a subtree before newProject builds it.
func nestTask(child, parent *models.Task) *models.Task {
	parentID := parent.ID
	child.ParentID = &parentID
	return child
}

func TestService_Capture_AdoptsMultipleChildrenAtomically(t *testing.T) {
	first := newTask("t-1", 1, "First")
	second := newTask("t-2", 2, "Second")
	bystander := newTask("t-3", 3, "Bystander")
	project := newProject("proj-1", "Proj", first, second, bystander)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Umbrella",
		ChildIDs:  []string{"t-1", "t-2"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want 1", len(repo.commits))
	}
	for _, child := range []*models.Task{first, second} {
		if child.ParentID == nil || *child.ParentID != res.TaskID {
			t.Errorf("%s parent = %v, want %s", child.Title, child.ParentID, res.TaskID)
		}
	}
	if bystander.ParentID != nil {
		t.Errorf("unmentioned task was reparented to %v", bystander.ParentID)
	}
	// Adoption re-nests tasks without resequencing them: every pre-existing task
	// keeps its slice position and the captured task is appended after them.
	titles := projectTaskTitles(project)
	want := []string{"First", "Second", "Bystander", "Umbrella"}
	if strings.Join(titles, ",") != strings.Join(want, ",") {
		t.Errorf("task order = %v, want %v", titles, want)
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Add Task" {
		t.Errorf("undo action = %q, want %q", cs.UndoAction, "Add Task")
	}
}

func TestService_Capture_RepeatedChildIDsCollapse(t *testing.T) {
	child := newTask("t-1", 1, "Child")
	project := newProject("proj-1", "Proj", child)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Umbrella",
		ChildIDs:  []string{"t-1", "t-1", "t-1"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != res.TaskID {
		t.Fatalf("child parent = %v, want %s", child.ParentID, res.TaskID)
	}
	created := project.GetTaskByID(res.TaskID)
	if created == nil || len(created.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(created.Children))
	}
}

func TestService_Capture_AdoptedChildKeepsItsSubtree(t *testing.T) {
	parent := newTask("t-1", 1, "Parent")
	kid := nestTask(newTask("t-2", 2, "Kid"), parent)
	grandkid := nestTask(newTask("t-3", 3, "Grandkid"), kid)
	project := newProject("proj-1", "Proj", parent, kid, grandkid)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Umbrella",
		ChildIDs:  []string{"t-1"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if parent.ParentID == nil || *parent.ParentID != res.TaskID {
		t.Fatalf("adopted parent = %v, want %s", parent.ParentID, res.TaskID)
	}
	if kid.ParentID == nil || *kid.ParentID != parent.ID {
		t.Errorf("kid parent = %v, want %s", kid.ParentID, parent.ID)
	}
	if grandkid.ParentID == nil || *grandkid.ParentID != kid.ID {
		t.Errorf("grandkid parent = %v, want %s", grandkid.ParentID, kid.ID)
	}
	if kid.Level != 2 || grandkid.Level != 3 {
		t.Errorf("levels = kid %d grandkid %d, want 2 and 3", kid.Level, grandkid.Level)
	}
}

func TestService_Capture_AdoptedChildLeavesItsPreviousParent(t *testing.T) {
	oldParent := newTask("t-1", 1, "Old parent")
	child := nestTask(newTask("t-2", 2, "Child"), oldParent)
	project := newProject("proj-1", "Proj", oldParent, child)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "New home", ChildIDs: []string{"t-2"}})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != res.TaskID {
		t.Fatalf("child parent = %v, want %s", child.ParentID, res.TaskID)
	}
	if len(oldParent.Children) != 0 {
		t.Errorf("old parent still holds %d children", len(oldParent.Children))
	}
}

// TestService_Capture_ChildrenAloneKeepInboxDefault pins the destination rule:
// only ParentID selects the child Backlog default, so a root that merely adopts
// existing tasks still lands in Inbox.
func TestService_Capture_ChildrenAloneKeepInboxDefault(t *testing.T) {
	child := newTask("t-1", 1, "Child")
	project := newProject("proj-1", "Proj", child)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Umbrella", ChildIDs: []string{"t-1"}})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	created := project.GetTaskByID(res.TaskID)
	if created == nil || created.State() != models.StateInbox {
		t.Fatalf("state = %+v, want inbox", created)
	}
}

func TestService_Capture_RejectsBlankAndUnknownChildIDs(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		childIDs []string
		wantErr  error
	}{
		{name: "blank", childIDs: []string{"   "}, wantErr: app.ErrInvalidInput},
		{name: "empty", childIDs: []string{""}, wantErr: app.ErrInvalidInput},
		{name: "unknown", childIDs: []string{"gone"}, wantErr: app.ErrTaskNotFound},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			existing := newTask("t-1", 1, "Existing")
			project := newProject("proj-1", "Proj", existing)
			repo := newFakeRepo(project)
			svc := newTestService(repo)
			wantNextShortID := project.NextShortID

			_, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Umbrella", ChildIDs: testCase.childIDs})
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("error = %v, want %v", err, testCase.wantErr)
			}
			if len(repo.commits) != 0 || len(project.Tasks) != 1 || project.NextShortID != wantNextShortID {
				t.Fatalf("rejected capture mutated state: commits=%d tasks=%d next=%d want next=%d",
					len(repo.commits), len(project.Tasks), project.NextShortID, wantNextShortID)
			}
		})
	}
}

// TestService_Capture_RejectsParentThatIsAlsoAChild covers the one cycle the
// grammar can express directly: nesting under a task while adopting it.
func TestService_Capture_RejectsParentThatIsAlsoAChild(t *testing.T) {
	anchor := newTask("t-1", 1, "Anchor")
	project := newProject("proj-1", "Proj", anchor)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	parentID := anchor.ID
	_, err := svc.Capture(app.CaptureRequest{
		ProjectID: project.ID,
		Title:     "Umbrella",
		ParentID:  &parentID,
		ChildIDs:  []string{anchor.ID},
	})
	if !errors.Is(err, app.ErrInvalidInput) || !strings.Contains(err.Error(), "circular") {
		t.Fatalf("error = %v, want circular ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 || len(project.Tasks) != 1 || anchor.ParentID != nil {
		t.Fatalf("rejected capture mutated state: commits=%d tasks=%d anchor parent=%v",
			len(repo.commits), len(project.Tasks), anchor.ParentID)
	}
}

// TestService_Capture_ChildDepthIsMeasuredOnTheFinalGraph proves the parent edge
// and the child edges are validated together: the same adoption is legal or not
// depending on the height of the adopted subtree beneath the new task's parent.
func TestService_Capture_ChildDepthIsMeasuredOnTheFinalGraph(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		grandchild bool
		wantErr    bool
	}{
		{name: "leaf child fits at the cap", grandchild: false},
		{name: "child subtree overflows the cap", grandchild: true, wantErr: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			chain := make([]*models.Task, 0, models.MaxNestingDepth)
			var previous *models.Task
			for level := 0; level < models.MaxNestingDepth-1; level++ {
				link := newTask(fmt.Sprintf("chain-%d", level), level+1, fmt.Sprintf("chain %d", level))
				if previous != nil {
					nestTask(link, previous)
				}
				chain = append(chain, link)
				previous = link
			}
			adoptee := newTask("adoptee", 90, "Adoptee")
			tasks := append(append([]*models.Task{}, chain...), adoptee)
			if testCase.grandchild {
				tasks = append(tasks, nestTask(newTask("adoptee-kid", 91, "Adoptee kid"), adoptee))
			}
			project := newProject("proj-1", "Proj", tasks...)
			repo := newFakeRepo(project)
			svc := newTestService(repo)

			parentID := previous.ID
			res, err := svc.Capture(app.CaptureRequest{
				ProjectID: project.ID,
				Title:     "Umbrella",
				ParentID:  &parentID,
				ChildIDs:  []string{adoptee.ID},
			})
			if testCase.wantErr {
				if !errors.Is(err, app.ErrInvalidInput) || !strings.Contains(err.Error(), "maximum nesting depth") {
					t.Fatalf("error = %v, want depth ErrInvalidInput", err)
				}
				if len(repo.commits) != 0 || adoptee.ParentID != nil {
					t.Fatalf("rejected capture mutated state: commits=%d adoptee parent=%v", len(repo.commits), adoptee.ParentID)
				}
				return
			}
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}
			if adoptee.ParentID == nil || *adoptee.ParentID != res.TaskID {
				t.Fatalf("adoptee parent = %v, want %s", adoptee.ParentID, res.TaskID)
			}
			if adoptee.Level != models.MaxNestingDepth {
				t.Errorf("adoptee level = %d, want %d", adoptee.Level, models.MaxNestingDepth)
			}
		})
	}
}

// TestService_Capture_TerminalDestinationJudgesAdoptedChildren pins capture's
// half of the one terminal-adoption policy. A $done or $canceled capture that
// adopts open work is refused with the same ChildrenBlockedError the inline-edit
// surface raises, instead of landing terminal and being silently reopened by the
// post-mutation reconciliation; adopting finished work still lands terminal.
func TestService_Capture_TerminalDestinationJudgesAdoptedChildren(t *testing.T) {
	for _, destination := range []app.CaptureDestination{textgrammar.DestinationDone, textgrammar.DestinationCanceled} {
		t.Run("open adopted child blocks "+string(destination), func(t *testing.T) {
			child := newTask("t-1", 1, "Open child")
			project := newProject("proj-1", "Proj", child)
			repo := newFakeRepo(project)

			_, err := newTestService(repo).Capture(app.CaptureRequest{
				ProjectID:   project.ID,
				Title:       "Umbrella",
				Destination: destination,
				ChildIDs:    []string{"t-1"},
			})
			var blocked *app.ChildrenBlockedError
			if !errors.As(err, &blocked) {
				t.Fatalf("error = %v, want a ChildrenBlockedError from the adopted child", err)
			}
			if child.ParentID != nil || len(project.Tasks) != 1 || len(repo.commits) != 0 {
				t.Fatalf("refused capture left state behind: child parent=%v tasks=%d commits=%d",
					child.ParentID, len(project.Tasks), len(repo.commits))
			}
		})

		t.Run("finished adopted child allows "+string(destination), func(t *testing.T) {
			child := newTask("t-1", 1, "Finished child")
			child.Complete()
			project := newProject("proj-1", "Proj", child)
			repo := newFakeRepo(project)

			res, err := newTestService(repo).Capture(app.CaptureRequest{
				ProjectID:   project.ID,
				Title:       "Umbrella",
				Destination: destination,
				ChildIDs:    []string{"t-1"},
			})
			if err != nil {
				t.Fatalf("Capture: %v", err)
			}
			created := project.GetTaskByID(res.TaskID)
			if created == nil || !created.IsTerminalState() {
				t.Fatalf("captured parent = %+v, want a terminal state", created)
			}
			if child.ParentID == nil || *child.ParentID != res.TaskID {
				t.Fatalf("child parent = %v, want %s", child.ParentID, res.TaskID)
			}
		})
	}
}

// TestService_CaptureMany_TerminalDestinationBlockedByAdoptedChildNamesItsLine
// keeps the batch surface on the same policy: one refused line names itself and
// leaves every project in the batch untouched.
func TestService_CaptureMany_TerminalDestinationBlockedByAdoptedChildNamesItsLine(t *testing.T) {
	child := newTask("t-1", 1, "Open child")
	project := newProject("proj-1", "Proj", child)
	repo := newFakeRepo(project)

	_, err := newTestService(repo).CaptureMany([]app.CaptureRequest{
		{SourceLine: 1, ProjectID: project.ID, Title: "Plain"},
		{SourceLine: 2, ProjectID: project.ID, Title: "Umbrella", Destination: textgrammar.DestinationDone, ChildIDs: []string{"t-1"}},
	})
	var blocked *app.ChildrenBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("error = %v, want a ChildrenBlockedError from the adopted child", err)
	}
	if !strings.HasPrefix(err.Error(), "line 2: ") {
		t.Errorf("error = %q, want it attributed to line 2", err)
	}
	if child.ParentID != nil || len(project.Tasks) != 1 || len(repo.commits) != 0 {
		t.Fatalf("refused batch was partial: child parent=%v tasks=%d commits=%d",
			child.ParentID, len(project.Tasks), len(repo.commits))
	}
}

// TestService_Capture_AdoptionLeavesChildPlanningUntouched pins the planning
// scope: adoption changes no project or planning ownership, so the child keeps
// its Today membership and only the new task's destination is reconciled.
func TestService_Capture_AdoptionLeavesChildPlanningUntouched(t *testing.T) {
	child := newTask("t-1", 1, "Planned child")
	project := newProject("proj-1", "Proj", child)
	repo := newFakeRepo(project)
	planning := &config.PlanningState{}
	planning.AddToToday(child.ID, project.ID)
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	res, err := svc.Capture(app.CaptureRequest{
		ProjectID:   project.ID,
		Title:       "Umbrella",
		Destination: app.CaptureDestinationNext,
		ChildIDs:    []string{"t-1"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !planningHasLocator(planning.GetToday(), child.ID, project.ID) {
		t.Errorf("adopted child lost Today: %+v", planning.GetToday())
	}
	if !planningHasLocator(planning.GetNext(), res.TaskID, project.ID) {
		t.Errorf("captured task missing from Next: %+v", planning.GetNext())
	}
	if planningHasLocator(planning.GetNext(), child.ID, project.ID) {
		t.Errorf("adoption widened planning to the child: %+v", planning.GetNext())
	}
}

func TestService_Capture_CommitFailureRestoresAdoptedChildren(t *testing.T) {
	child := newTask("t-1", 1, "Child")
	project := newProject("proj-1", "Proj", child)
	repo := newFakeRepo(project)
	repo.commitErr = errors.New("commit unavailable")
	svc := newTestService(repo)
	wantNextShortID := project.NextShortID

	_, err := svc.Capture(app.CaptureRequest{ProjectID: project.ID, Title: "Umbrella", ChildIDs: []string{"t-1"}})
	if err == nil || !strings.Contains(err.Error(), "commit unavailable") {
		t.Fatalf("error = %v, want commit failure", err)
	}
	restored := project.GetTaskByID("t-1")
	if restored == nil || restored.ParentID != nil {
		t.Fatalf("child parent = %+v, want restored root", restored)
	}
	if len(project.Tasks) != 1 || project.NextShortID != wantNextShortID {
		t.Fatalf("failed commit was partial: tasks=%d next=%d want next=%d",
			len(project.Tasks), project.NextShortID, wantNextShortID)
	}
}
