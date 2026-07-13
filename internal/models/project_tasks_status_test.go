package models

import "testing"

func TestProjectGetTasksByState_AllStates(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	var tasks []*Task
	states := AllWorkflowStates()
	for i, state := range states {
		task, taskErr := NewTask(string(state), i+1, TaskTypeTask, "Task in "+string(state))
		if taskErr != nil {
			t.Fatal(taskErr)
		}
		task.ApplyWorkflowState(state)
		tasks = append(tasks, task)
	}
	project.Tasks = tasks

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			got := project.GetTasksByState(state)
			if len(got) != 1 {
				t.Fatalf("GetTasksByState(%s) returned %d tasks, want 1", state, len(got))
			}
			if got[0].State() != state {
				t.Errorf("returned task has state %s, want %s", got[0].State(), state)
			}
		})
	}
}

func TestProjectGetTasksByState_MultipleMatches(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task1, err := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("task-2", 2, TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task3, err := NewTask("task-3", 3, TaskTypeTask, "Task 3")
	if err != nil {
		t.Fatal(err)
	}
	task3.Complete()

	project.Tasks = []*Task{task1, task2, task3}

	readyTasks := project.GetTasksByState(StateReady)
	if len(readyTasks) != 2 {
		t.Errorf("expected 2 ready tasks, got %d", len(readyTasks))
	}
}

func TestProjectGetTasksByState_DerivesFromLegacyFields(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	// A task in the waiting state (the canonical successor of the legacy
	// blocked status) is bucketed under StateWaiting.
	task, err := NewTask("task-1", 1, TaskTypeTask, "Blocked task")
	if err != nil {
		t.Fatal(err)
	}
	task.ApplyWorkflowState(StateWaiting)
	project.Tasks = []*Task{task}

	waiting := project.GetTasksByState(StateWaiting)
	if len(waiting) != 1 {
		t.Errorf("expected legacy blocked task in waiting state, got %d tasks", len(waiting))
	}
}
