package app_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestTaskLineEditAppliesSameProjectPatchInOneTransaction(t *testing.T) {
	task := newTask("task", 1, "Before")
	task.Priority = models.PriorityLow
	task.Tags = []string{"keep", "remove"}
	task.ApplyWorkflowState(models.StateWaiting)
	snooze := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	task.SnoozedUntil = &snooze
	child := newTask("child", 2, "Child")
	child.ParentID = &task.ID
	parent := newTask("parent", 3, "New parent")
	project := newProject("project", "Project", task, child, parent)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	priority := models.PriorityHigh
	due := time.Date(2026, 8, 14, 17, 30, 0, 0, time.UTC)
	destination := app.CaptureDestinationNext
	var planning app.PlanningReconcileRequest
	expected := ""
	result, err := svc.EditTaskLine(app.TaskLineEditRequest{
		Source:       app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:        "  After  ",
		Priority:     &priority,
		DueSet:       true,
		Due:          &due,
		Destination:  &destination,
		TagsToAdd:    []string{"added"},
		TagsToRemove: []string{"remove"},
		Parent: app.TaskLineParentPatch{
			Mode:   app.TaskLineParentSet,
			Parent: app.TaskLocator{ProjectID: project.ID, TaskID: parent.ID},
		},
		PlanningArtifacts: func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
			planning = req
			return []storage.ArtifactChange{{Path: "planning.json", Data: []byte("next"), ExpectedRevision: &expected}}, nil
		},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	assertSameProjectTaskLineEdit(t, repo, project, task, child, parent, planning, result)
}

func assertSameProjectTaskLineEdit(
	t *testing.T,
	repo *fakeRepo,
	project *models.Project,
	task, child, parent *models.Task,
	planning app.PlanningReconcileRequest,
	result app.TaskLineEditResult,
) {
	t.Helper()
	if repo.snapshots != 1 {
		t.Fatalf("snapshots = %d, want exactly one", repo.snapshots)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 1 {
		t.Fatalf("commits = %+v, want one transaction with one changeset", repo.commits)
	}
	change := repo.commits[0][0]
	if change.UndoAction != "Edit Task Line" || len(change.Artifacts) != 1 {
		t.Fatalf("changeset = %+v, want edit undo metadata and planning artifact", change)
	}
	if task.Title != "After" || task.Priority != models.PriorityHigh {
		t.Fatalf("content patch not applied: title=%q priority=%d", task.Title, task.Priority)
	}
	if task.DueDate == nil || task.DueDate.Hour() != 0 || task.DueDate.Day() != 14 {
		t.Fatalf("due date = %v, want normalized 2026-08-14", task.DueDate)
	}
	if !slices.Equal(task.Tags, []string{"keep", "added"}) {
		t.Fatalf("tags = %v, want [keep added]", task.Tags)
	}
	if task.ParentID == nil || *task.ParentID != parent.ID {
		t.Fatalf("parent = %v, want %q", task.ParentID, parent.ID)
	}
	if child.ParentID == nil || *child.ParentID != task.ID {
		t.Fatalf("descendant hierarchy changed: child parent = %v", child.ParentID)
	}
	if task.State() != models.StateBacklog || task.SnoozedUntil != nil {
		t.Fatalf("Next destination state/snooze = %s/%v", task.State(), task.SnoozedUntil)
	}
	if len(planning.Targets) != 1 || planning.Targets[0].Next != app.PlanningNextAdd || planning.Targets[0].Today != app.PlanningTodayRemove {
		t.Fatalf("planning request = %+v, want canonical Next directives", planning)
	}
	if !result.Changed || result.Task != (app.TaskLocator{ProjectID: project.ID, TaskID: task.ID}) || result.TransactionID == "" {
		t.Fatalf("result identity/transaction = %+v", result)
	}
	if result.MovedDescendantCount != 1 || !slices.Equal(result.AffectedProjectIDs, []string{project.ID}) {
		t.Fatalf("result move/affected projects = %+v", result)
	}
}

func TestTaskLineEditSemanticNoOpDoesNotTransact(t *testing.T) {
	task := newTask("task", 1, "Same")
	task.Tags = []string{"keep"}
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "Same",
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if result.Changed || result.TransactionID != "" || len(result.AffectedProjectIDs) != 0 {
		t.Fatalf("no-op result = %+v", result)
	}
	if result.Task.TaskID != task.ID {
		t.Fatalf("no-op lost final identity: %+v", result)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("no-op committed %d transaction(s)", len(repo.commits))
	}
}

func TestTaskLineEditRejectsValidationAndHierarchyWithoutMutation(t *testing.T) {
	t.Run("conflicting tags", func(t *testing.T) {
		task := newTask("task", 1, "Before")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:       app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:        "After",
			TagsToAdd:    []string{"same"},
			TagsToRemove: []string{"same"},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want ErrInvalidInput", err)
		}
		if task.Title != "Before" || len(repo.commits) != 0 {
			t.Fatalf("rejected patch mutated title or committed: %q/%d", task.Title, len(repo.commits))
		}
	})

	t.Run("descendant parent", func(t *testing.T) {
		task := newTask("task", 1, "Before")
		child := newTask("child", 2, "Child")
		child.ParentID = &task.ID
		project := newProject("project", "Project", task, child)
		repo := newFakeRepo(project)
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:  "After",
			Parent: app.TaskLineParentPatch{
				Mode:   app.TaskLineParentSet,
				Parent: app.TaskLocator{ProjectID: project.ID, TaskID: child.ID},
			},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want hierarchy ErrInvalidInput", err)
		}
		if task.Title != "Before" || task.ParentID != nil || child.ParentID == nil || *child.ParentID != task.ID {
			t.Fatalf("hierarchy failure did not restore full state: task=%+v child=%+v", task, child)
		}
		if len(repo.commits) != 0 {
			t.Fatalf("hierarchy failure committed %d transaction(s)", len(repo.commits))
		}
	})

	t.Run("cross project parent belongs to source", func(t *testing.T) {
		task := newTask("task", 1, "Before")
		wrongParent := newTask("wrong-parent", 2, "Wrong parent")
		source := newProject("source", "Source", task, wrongParent)
		destination := newProject("destination", "Destination")
		repo := newFakeRepo(source, destination)
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: task.ID},
			Title:                "After",
			DestinationProjectID: destination.ID,
			Parent: app.TaskLineParentPatch{
				Mode:   app.TaskLineParentSet,
				Parent: app.TaskLocator{ProjectID: source.ID, TaskID: wrongParent.ID},
			},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want cross-project parent ErrInvalidInput", err)
		}
		if task.Title != "Before" || len(repo.commits) != 0 {
			t.Fatalf("invalid parent mutated state or committed: %q/%d", task.Title, len(repo.commits))
		}
	})
}

func TestTaskLineEditMovesCrossProjectSubtreeInOneTransaction(t *testing.T) {
	root := newTask("root", 1, "Before")
	root.Tags = []string{"root-tag"}
	child := newTask("child", 2, "Child")
	child.ParentID = &root.ID
	child.Priority = models.PriorityLow
	child.Tags = []string{"child-tag"}
	source := newProject("source", "Source", root, child)
	parent := newTask("parent", 8, "Destination parent")
	destination := newProject("destination", "Destination", parent)
	repo := newFakeRepo(source, destination)
	priority := models.PriorityHigh
	planningDestination := app.CaptureDestinationNext
	var planning app.PlanningReconcileRequest
	expected := ""

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
		Title:                "After",
		Priority:             &priority,
		TagsToAdd:            []string{"edited"},
		Destination:          &planningDestination,
		DestinationProjectID: destination.ID,
		Parent: app.TaskLineParentPatch{
			Mode:   app.TaskLineParentSet,
			Parent: app.TaskLocator{ProjectID: destination.ID, TaskID: parent.ID},
		},
		PlanningArtifacts: func(req app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
			planning = req
			return []storage.ArtifactChange{{Path: "planning.json", Data: []byte("moved"), ExpectedRevision: &expected}}, nil
		},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	assertCrossProjectTaskLineMove(t, repo, source, destination, root, child, parent)
	assertCrossProjectTaskLinePlanning(t, planning, source.ID, destination.ID)
	assertCrossProjectTaskLineResult(t, result, root, source.ID, destination.ID)
}

func assertCrossProjectTaskLineMove(
	t *testing.T,
	repo *fakeRepo,
	source, destination *models.Project,
	root, child, parent *models.Task,
) {
	t.Helper()
	if repo.snapshots != 1 {
		t.Fatalf("snapshots = %d, want exactly one", repo.snapshots)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commits = %+v, want one transaction with source and destination", repo.commits)
	}
	if source.GetTaskByID(root.ID) != nil || source.GetTaskByID(child.ID) != nil {
		t.Fatalf("source retained moved subtree: %+v", source.Tasks)
	}
	if destination.GetTaskByID(root.ID) != root || destination.GetTaskByID(child.ID) != child {
		t.Fatalf("destination lost stable task identities: %+v", destination.Tasks)
	}
	if root.ParentID == nil || *root.ParentID != parent.ID || child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("moved hierarchy = root parent %v child parent %v", root.ParentID, child.ParentID)
	}
	if root.ShortID != 9 || child.ShortID != 10 {
		t.Fatalf("fresh destination short IDs = root %d child %d, want 9/10", root.ShortID, child.ShortID)
	}
	if root.Title != "After" || root.Priority != models.PriorityHigh || !slices.Equal(root.Tags, []string{"root-tag", "edited"}) {
		t.Fatalf("root patches not applied: %+v", root)
	}
	if child.Title != "Child" || child.Priority != models.PriorityLow || !slices.Equal(child.Tags, []string{"child-tag"}) {
		t.Fatalf("descendant metadata changed: %+v", child)
	}
	sourceChange := repo.commits[0][0]
	if sourceChange.Project.ID != source.ID || len(sourceChange.AttachmentOps) != 2 || len(sourceChange.Artifacts) != 1 {
		t.Fatalf("source changeset side effects = %+v", sourceChange)
	}
}

func assertCrossProjectTaskLinePlanning(t *testing.T, planning app.PlanningReconcileRequest, sourceID, destinationID string) {
	t.Helper()
	if len(planning.Moves) != 2 || len(planning.Targets) != 1 || planning.Targets[0].ProjectID != destinationID {
		t.Fatalf("planning request = %+v, want two requalifications and one destination-root patch", planning)
	}
	for _, move := range planning.Moves {
		if move.FromProjectID != sourceID || move.ToProjectID != destinationID {
			t.Fatalf("planning move = %+v, want source to destination", move)
		}
	}
}

func assertCrossProjectTaskLineResult(
	t *testing.T,
	result app.TaskLineEditResult,
	root *models.Task,
	sourceID, destinationID string,
) {
	t.Helper()
	if !result.Changed || result.Task != (app.TaskLocator{ProjectID: destinationID, TaskID: root.ID}) || result.ShortID != root.ShortID {
		t.Fatalf("result final identity = %+v", result)
	}
	if result.MovedDescendantCount != 1 || !slices.Equal(result.AffectedProjectIDs, []string{sourceID, destinationID}) || result.TransactionID == "" {
		t.Fatalf("result transaction metadata = %+v", result)
	}
}

func TestTaskLineEditRollsBackPlanningPersistenceAndCommitFailures(t *testing.T) {
	t.Run("planning artifact failure", func(t *testing.T) {
		task := newTask("task", 1, "Before")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)
		destination := app.CaptureDestinationToday
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:      app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:       "After",
			Destination: &destination,
			PlanningArtifacts: func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
				return nil, errors.New("planning unavailable")
			},
		})
		if err == nil {
			t.Fatal("expected planning failure")
		}
		if task.Title != "Before" || len(repo.commits) != 0 {
			t.Fatalf("planning failure left partial edit: title=%q commits=%d", task.Title, len(repo.commits))
		}
	})

	t.Run("repository commit failure", func(t *testing.T) {
		task := newTask("task", 1, "Before")
		parent := newTask("parent", 2, "Parent")
		project := newProject("project", "Project", task, parent)
		repo := newFakeRepo(project)
		repo.commitErr = errors.New("disk full")
		priority := models.PriorityHigh
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "After",
			Priority: &priority,
			Parent: app.TaskLineParentPatch{
				Mode:   app.TaskLineParentSet,
				Parent: app.TaskLocator{ProjectID: project.ID, TaskID: parent.ID},
			},
		})
		if err == nil {
			t.Fatal("expected commit failure")
		}
		if task.Title != "Before" || task.Priority != models.PriorityUnset || task.ParentID != nil {
			t.Fatalf("commit failure left partial task: %+v", task)
		}
		if len(repo.commits) != 0 {
			t.Fatalf("failed repository recorded %d commit(s)", len(repo.commits))
		}
	})
}

func TestTaskLineEditCrossProjectFailuresRestoreBothProjects(t *testing.T) {
	assertRestored := func(t *testing.T, source, destination *models.Project, root, child *models.Task, commits int) {
		t.Helper()
		if source.GetTaskByID(root.ID) != root || source.GetTaskByID(child.ID) != child {
			t.Fatalf("source subtree not restored: %+v", source.Tasks)
		}
		if destination.GetTaskByID(root.ID) != nil || destination.GetTaskByID(child.ID) != nil {
			t.Fatalf("destination retained partial subtree: %+v", destination.Tasks)
		}
		if root.Title != "Before" || root.ShortID != 1 || root.ParentID != nil || child.ShortID != 2 || child.ParentID == nil || *child.ParentID != root.ID {
			t.Fatalf("task identity/content not restored: root=%+v child=%+v", root, child)
		}
		if commits != 0 {
			t.Fatalf("failed transaction recorded %d commit(s)", commits)
		}
	}

	newWorkspace := func() (*models.Task, *models.Task, *models.Project, *models.Project, *fakeRepo) {
		root := newTask("root", 1, "Before")
		child := newTask("child", 2, "Child")
		child.ParentID = &root.ID
		source := newProject("source", "Source", root, child)
		destination := newProject("destination", "Destination")
		repo := newFakeRepo(source, destination)
		return root, child, source, destination, repo
	}

	t.Run("planning state", func(t *testing.T) {
		root, child, source, destination, repo := newWorkspace()
		wantErr := errors.New("planning unavailable")
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
			Title:                "After",
			DestinationProjectID: destination.ID,
			PlanningArtifacts: func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
				return nil, wantErr
			},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		assertRestored(t, source, destination, root, child, len(repo.commits))
	})

	t.Run("repository transaction after internal move", func(t *testing.T) {
		root, child, source, destination, repo := newWorkspace()
		repo.commitErr = errors.New("transaction unavailable")
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
			Title:                "After",
			DestinationProjectID: destination.ID,
		})
		if err == nil {
			t.Fatal("expected transaction failure")
		}
		assertRestored(t, source, destination, root, child, len(repo.commits))
	})

	for _, staleProject := range []string{"source", "destination"} {
		t.Run(staleProject+" document revision", func(t *testing.T) {
			root, child, source, destination, repo := newWorkspace()
			repo.preCommit = func() {
				repo.revisions[staleProject] = "concurrent-revision"
				repo.preCommit = nil
			}
			_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
				Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
				Title:                "After",
				DestinationProjectID: destination.ID,
			})
			if !errors.Is(err, storage.ErrStaleRevision) {
				t.Fatalf("error = %v, want ErrStaleRevision", err)
			}
			assertRestored(t, source, destination, root, child, len(repo.commits))
		})
	}

	t.Run("destination depth after subtree insertion", func(t *testing.T) {
		root, child, source, destination, repo := newWorkspace()
		var parent *models.Task
		for level := 0; level <= models.MaxNestingDepth; level++ {
			node := newTask(fmt.Sprintf("parent-%d", level), level+1, fmt.Sprintf("Parent %d", level))
			if parent != nil {
				node.ParentID = &parent.ID
			}
			destination.Tasks = append(destination.Tasks, node)
			parent = node
		}
		destination.BuildTaskTree()
		repo.documents[destination.ID] = documentOf(destination, nil)
		repo.revisions[destination.ID] = repo.documents[destination.ID].Revision

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
			Title:                "After",
			DestinationProjectID: destination.ID,
			Parent: app.TaskLineParentPatch{
				Mode:   app.TaskLineParentSet,
				Parent: app.TaskLocator{ProjectID: destination.ID, TaskID: parent.ID},
			},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want depth ErrInvalidInput", err)
		}
		assertRestored(t, source, destination, root, child, len(repo.commits))
	})
}

func TestTaskLineEditRejectsStaleRevisionAndRestoresSnapshot(t *testing.T) {
	task := newTask("task", 1, "Before")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	repo.preCommit = func() {
		repo.revisions[project.ID] = "concurrent-revision"
		repo.preCommit = nil
	}

	_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "After",
	})
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("error = %v, want ErrStaleRevision", err)
	}
	if task.Title != "Before" || len(repo.commits) != 0 {
		t.Fatalf("stale edit left partial state: title=%q commits=%d", task.Title, len(repo.commits))
	}
}
