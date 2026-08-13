package app_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func taskLineDestination(t *testing.T, raw string) *app.CaptureDestination {
	t.Helper()
	destination, ok := app.ParseCaptureDestination(raw)
	if !ok {
		t.Fatalf("ParseCaptureDestination(%q) failed", raw)
	}
	return &destination
}

func TestTaskLineEditAdoptsChildrenAdditively(t *testing.T) {
	task := newTask("task", 1, "Edited")
	kept := newTask("kept", 2, "Kept child")
	kept.ParentID = &task.ID
	adopted := newTask("adopted", 3, "Adopted")
	grandchild := newTask("grandchild", 4, "Grandchild")
	grandchild.ParentID = &adopted.ID
	former := newTask("former", 5, "Former parent")
	reparented := newTask("reparented", 6, "Reparented child")
	reparented.ParentID = &former.ID
	project := newProject("project", "Project", task, kept, adopted, grandchild, former, reparented)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "Edited",
		Children: []app.TaskLocator{
			{ProjectID: project.ID, TaskID: adopted.ID},
			{ProjectID: project.ID, TaskID: reparented.ID},
			{ProjectID: project.ID, TaskID: adopted.ID},
		},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	for _, child := range []*models.Task{kept, adopted, reparented} {
		if child.ParentID == nil || *child.ParentID != task.ID {
			t.Fatalf("child %q parent = %v, want the edited task", child.ID, child.ParentID)
		}
	}
	if grandchild.ParentID == nil || *grandchild.ParentID != adopted.ID {
		t.Fatalf("adopted subtree changed: %v", grandchild.ParentID)
	}
	if len(former.Children) != 0 {
		t.Fatalf("former parent kept %d children", len(former.Children))
	}
	if !result.Changed || result.TransactionID == "" || result.MovedDescendantCount != 0 {
		t.Fatalf("result = %+v, want a committed edit that relocated no descendants", result)
	}
	if !slices.Equal(result.AffectedProjectIDs, []string{project.ID}) || len(repo.commits) != 1 || len(repo.commits[0]) != 1 {
		t.Fatalf("affected projects/commits = %v/%+v", result.AffectedProjectIDs, repo.commits)
	}
}

func TestTaskLineEditAlreadyDirectChildDoesNotTransact(t *testing.T) {
	task := newTask("task", 1, "Same")
	child := newTask("child", 2, "Child")
	child.ParentID = &task.ID
	project := newProject("project", "Project", task, child)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:    "Same",
		Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: child.ID}},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if result.Changed || result.TransactionID != "" || len(result.AffectedProjectIDs) != 0 {
		t.Fatalf("no-op result = %+v", result)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("no-op committed %d transaction(s)", len(repo.commits))
	}
}

func TestTaskLineEditAppliesParentAndChildEdgesAsOneBatch(t *testing.T) {
	parent := newTask("parent", 1, "Parent")
	task := newTask("task", 2, "Inverted")
	task.ParentID = &parent.ID
	child := newTask("child", 3, "Child")
	child.ParentID = &task.ID
	project := newProject("project", "Project", parent, task, child)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:    "Inverted",
		Parent:   app.TaskLineParentPatch{Mode: app.TaskLineParentRoot},
		Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: parent.ID}},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if task.ParentID != nil {
		t.Fatalf("edited task parent = %v, want root", task.ParentID)
	}
	if parent.ParentID == nil || *parent.ParentID != task.ID {
		t.Fatalf("inverted parent = %v, want the edited task", parent.ParentID)
	}
	if child.ParentID == nil || *child.ParentID != task.ID {
		t.Fatalf("existing child = %v, want retained", child.ParentID)
	}
	if result.MovedDescendantCount != 1 {
		t.Fatalf("MovedDescendantCount = %d, want the edited subtree only", result.MovedDescendantCount)
	}
}

func TestTaskLineEditRejectsChildAssignmentsWithoutMutation(t *testing.T) {
	t.Run("adopting the retained parent", func(t *testing.T) {
		parent := newTask("parent", 1, "Parent")
		task := newTask("task", 2, "Edited")
		task.ParentID = &parent.ID
		project := newProject("project", "Project", parent, task)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: parent.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want cycle ErrInvalidInput", err)
		}
		if parent.ParentID != nil || task.ParentID == nil || *task.ParentID != parent.ID || len(repo.commits) != 0 {
			t.Fatalf("rejected cycle mutated hierarchy: parent=%v task=%v commits=%d", parent.ParentID, task.ParentID, len(repo.commits))
		}
	})

	t.Run("adopting itself", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want self-parent ErrInvalidInput", err)
		}
		if task.ParentID != nil || len(repo.commits) != 0 {
			t.Fatalf("rejected self-adoption mutated state: %v/%d", task.ParentID, len(repo.commits))
		}
	})

	t.Run("adoption past the depth cap", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		tasks := []*models.Task{task}
		var head, previous *models.Task
		for level := 0; level <= models.MaxNestingDepth; level++ {
			node := newTask(fmt.Sprintf("chain-%d", level), level+2, fmt.Sprintf("Chain %d", level))
			if previous == nil {
				head = node
			} else {
				node.ParentID = &previous.ID
			}
			tasks = append(tasks, node)
			previous = node
		}
		project := newProject("project", "Project", tasks...)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: head.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want depth ErrInvalidInput", err)
		}
		if head.ParentID != nil || len(repo.commits) != 0 {
			t.Fatalf("rejected depth overflow mutated state: %v/%d", head.ParentID, len(repo.commits))
		}
	})
}

func TestTaskLineEditRejectsUnresolvableChildLocators(t *testing.T) {
	t.Run("stale task id", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: "deleted"}},
		})
		if !errors.Is(err, app.ErrTaskNotFound) {
			t.Fatalf("error = %v, want ErrTaskNotFound", err)
		}
		if len(repo.commits) != 0 {
			t.Fatalf("stale child committed %d transaction(s)", len(repo.commits))
		}
	})

	t.Run("unqualified locator", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("child outside the effective project", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		stranded := newTask("stranded", 2, "Stranded")
		source := newProject("source", "Source", task, stranded)
		destination := newProject("destination", "Destination")
		repo := newFakeRepo(source, destination)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: task.ID},
			Title:                "Edited",
			DestinationProjectID: destination.ID,
			Children:             []app.TaskLocator{{ProjectID: source.ID, TaskID: stranded.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want effective-project ErrInvalidInput", err)
		}
		if source.GetTaskByID(task.ID) != task || len(repo.commits) != 0 {
			t.Fatalf("rejected cross-project adoption moved the subtree: %+v/%d", source.Tasks, len(repo.commits))
		}
	})
}

func TestTaskLineEditTerminalDestinationJudgesAdoptedChildren(t *testing.T) {
	for destination, terminal := range map[string]models.WorkflowState{
		"done":     models.StateDone,
		"canceled": models.StateCanceled,
	} {
		t.Run("open adopted child blocks "+destination, func(t *testing.T) {
			task := newTask("task", 1, "Edited")
			open := newTask("open", 2, "Open")
			project := newProject("project", "Project", task, open)
			repo := newFakeRepo(project)

			_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
				Source:      app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
				Title:       "Edited",
				Destination: taskLineDestination(t, destination),
				Children:    []app.TaskLocator{{ProjectID: project.ID, TaskID: open.ID}},
			})
			var blocked *app.ChildrenBlockedError
			if !errors.As(err, &blocked) {
				t.Fatalf("error = %v, want a ChildrenBlockedError from the adopted child", err)
			}
			if task.IsTerminalState() || open.ParentID != nil || len(repo.commits) != 0 {
				t.Fatalf("blocked edit left state behind: task=%s child=%v commits=%d", task.State(), open.ParentID, len(repo.commits))
			}
		})

		// A destination naming the state the task already holds is still an
		// explicit terminal claim, so it must be refused instead of committing and
		// being reopened to Doing by ReconcileParentCompletion.
		t.Run("open adopted child blocks a re-applied "+destination, func(t *testing.T) {
			task := newTask("task", 1, "Edited")
			task.ApplyWorkflowState(terminal)
			open := newTask("open", 2, "Open")
			project := newProject("project", "Project", task, open)
			repo := newFakeRepo(project)

			_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
				Source:      app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
				Title:       "Edited",
				Destination: taskLineDestination(t, destination),
				Children:    []app.TaskLocator{{ProjectID: project.ID, TaskID: open.ID}},
			})
			var blocked *app.ChildrenBlockedError
			if !errors.As(err, &blocked) {
				t.Fatalf("error = %v, want a ChildrenBlockedError from the adopted child", err)
			}
			if task.State() != terminal || open.ParentID != nil || len(repo.commits) != 0 {
				t.Fatalf("blocked edit left state behind: task=%s child=%v commits=%d", task.State(), open.ParentID, len(repo.commits))
			}
		})
	}

	t.Run("terminal adopted child allows the terminal move", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		finished := newTask("finished", 2, "Finished")
		finished.ApplyWorkflowState(models.StateDone)
		project := newProject("project", "Project", task, finished)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:      app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:       "Edited",
			Destination: taskLineDestination(t, "done"),
			Children:    []app.TaskLocator{{ProjectID: project.ID, TaskID: finished.ID}},
		})
		if err != nil {
			t.Fatalf("EditTaskLine: %v", err)
		}
		if task.State() != models.StateDone || finished.ParentID == nil || *finished.ParentID != task.ID {
			t.Fatalf("terminal adoption = state %s child %v", task.State(), finished.ParentID)
		}
	})

	t.Run("adoption by an already terminal task reconciles", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		task.ApplyWorkflowState(models.StateDone)
		open := newTask("open", 2, "Open")
		project := newProject("project", "Project", task, open)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: open.ID}},
		})
		if err != nil {
			t.Fatalf("EditTaskLine: %v", err)
		}
		if task.State() != models.StateDoing {
			t.Fatalf("state = %s, want the reconciliation hook to reopen the new parent", task.State())
		}
	})
}

func TestTaskLineEditAdoptsDestinationChildrenOnCrossProjectMove(t *testing.T) {
	root := newTask("root", 1, "Before")
	descendant := newTask("descendant", 2, "Descendant")
	descendant.ParentID = &root.ID
	source := newProject("source", "Source", root, descendant)
	parent := newTask("parent", 8, "Destination parent")
	adopted := newTask("adopted", 9, "Destination child")
	destination := newProject("destination", "Destination", parent, adopted)
	repo := newFakeRepo(source, destination)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
		Title:                "After",
		DestinationProjectID: destination.ID,
		Parent: app.TaskLineParentPatch{
			Mode:   app.TaskLineParentSet,
			Parent: app.TaskLocator{ProjectID: destination.ID, TaskID: parent.ID},
		},
		Children: []app.TaskLocator{{ProjectID: destination.ID, TaskID: adopted.ID}},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if root.ParentID == nil || *root.ParentID != parent.ID {
		t.Fatalf("moved root parent = %v, want the destination parent", root.ParentID)
	}
	if descendant.ParentID == nil || *descendant.ParentID != root.ID || adopted.ParentID == nil || *adopted.ParentID != root.ID {
		t.Fatalf("final hierarchy = descendant %v adopted %v", descendant.ParentID, adopted.ParentID)
	}
	if adopted.ShortID != 9 {
		t.Fatalf("adopted short ID = %d, want the destination task left in place", adopted.ShortID)
	}
	if result.MovedDescendantCount != 1 {
		t.Fatalf("MovedDescendantCount = %d, want the moved subtree only", result.MovedDescendantCount)
	}
	if !slices.Equal(result.AffectedProjectIDs, []string{source.ID, destination.ID}) || result.TransactionID == "" {
		t.Fatalf("result transaction metadata = %+v", result)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 || len(repo.commits[0][0].AttachmentOps) != 2 {
		t.Fatalf("commits = %+v, want one transaction relocating only the moved subtree", repo.commits)
	}
}

func TestTaskLineEditAdoptsDestinationChildrenAtRootOnCrossProjectMove(t *testing.T) {
	root := newTask("root", 1, "Before")
	source := newProject("source", "Source", root)
	adopted := newTask("adopted", 5, "Destination child")
	former := newTask("former", 6, "Former parent")
	adopted.ParentID = &former.ID
	destination := newProject("destination", "Destination", former, adopted)
	repo := newFakeRepo(source, destination)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
		Title:                "After",
		DestinationProjectID: destination.ID,
		Children:             []app.TaskLocator{{ProjectID: destination.ID, TaskID: adopted.ID}},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if root.ParentID != nil {
		t.Fatalf("moved root parent = %v, want a destination root", root.ParentID)
	}
	if adopted.ParentID == nil || *adopted.ParentID != root.ID {
		t.Fatalf("adopted parent = %v, want the moved root", adopted.ParentID)
	}
	if adopted.ShortID != 5 {
		t.Fatalf("adopted short ID = %d, want the destination task left in place", adopted.ShortID)
	}
	if result.MovedDescendantCount != 0 {
		t.Fatalf("MovedDescendantCount = %d, want the leaf subtree only", result.MovedDescendantCount)
	}
	if !slices.Equal(result.AffectedProjectIDs, []string{source.ID, destination.ID}) || result.TransactionID == "" {
		t.Fatalf("result transaction metadata = %+v", result)
	}
}

func TestTaskLineEditChildAdoptionRollsBackOnCommitFailure(t *testing.T) {
	t.Run("same project", func(t *testing.T) {
		task := newTask("task", 1, "Before")
		former := newTask("former", 2, "Former parent")
		child := newTask("child", 3, "Child")
		child.ParentID = &former.ID
		project := newProject("project", "Project", task, former, child)
		repo := newFakeRepo(project)
		repo.commitErr = errors.New("disk full")

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "After",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: child.ID}},
		})
		if err == nil {
			t.Fatal("expected commit failure")
		}
		if task.Title != "Before" || child.ParentID == nil || *child.ParentID != former.ID {
			t.Fatalf("commit failure left a partial adoption: title=%q child=%v", task.Title, child.ParentID)
		}
	})

	t.Run("cross project", func(t *testing.T) {
		root := newTask("root", 1, "Before")
		source := newProject("source", "Source", root)
		adopted := newTask("adopted", 5, "Destination child")
		destination := newProject("destination", "Destination", adopted)
		repo := newFakeRepo(source, destination)
		repo.commitErr = errors.New("transaction unavailable")

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
			Title:                "After",
			DestinationProjectID: destination.ID,
			Children:             []app.TaskLocator{{ProjectID: destination.ID, TaskID: adopted.ID}},
		})
		if err == nil {
			t.Fatal("expected commit failure")
		}
		if source.GetTaskByID(root.ID) != root || destination.GetTaskByID(root.ID) != nil {
			t.Fatalf("cross-project failure left the subtree split: source=%+v destination=%+v", source.Tasks, destination.Tasks)
		}
		if adopted.ParentID != nil || adopted.ShortID != 5 || root.Title != "Before" || root.ShortID != 1 {
			t.Fatalf("cross-project failure left an adopted child: adopted=%+v root=%+v", adopted, root)
		}
	})
}

// TestTaskLineEditAdoptedChildPreservesFlatOrder pins the ordering contract on
// the inline-edit surface: a completed root adopts an open root that sits before
// an unrelated root, so the post-mutation hooks reopen the parent and would
// previously have dragged the adopted task past the bystander.
func TestTaskLineEditAdoptedChildPreservesFlatOrder(t *testing.T) {
	adopted := newTask("adopted", 1, "Adopted root")
	unrelated := newTask("unrelated", 2, "Unrelated root")
	finished := newTask("finished", 3, "Finished root")
	finished.Complete()
	project := newProject("project", "Project", adopted, unrelated, finished)
	repo := newFakeRepo(project)

	if _, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:   app.TaskLocator{ProjectID: project.ID, TaskID: finished.ID},
		Title:    "Finished root",
		Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: adopted.ID}},
	}); err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if adopted.ParentID == nil || *adopted.ParentID != finished.ID {
		t.Fatalf("adopted parent = %v, want %s", adopted.ParentID, finished.ID)
	}
	if finished.IsComplete() {
		t.Fatalf("completed root kept its terminal state above an open child")
	}
	if unrelated.ParentID != nil {
		t.Fatalf("unmentioned root was reparented to %v", unrelated.ParentID)
	}
	want := []string{"Adopted root", "Unrelated root", "Finished root"}
	if got := projectTaskTitles(project); !slices.Equal(got, want) {
		t.Errorf("task order = %v, want %v", got, want)
	}
}
