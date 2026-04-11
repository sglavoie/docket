package models

import (
	"fmt"
	"testing"
)

func TestProjectHasCircularDependency(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	task1 := newDependencyTestTask(t, "1", 1, "Task 1")
	task2 := newDependencyTestTask(t, "2", 2, "Task 2")
	task3 := newDependencyTestTask(t, "3", 3, "Task 3")

	project.Tasks = []*Task{task1, task2, task3}

	if project.HasCircularDependency(task1.ID, task2.ID) {
		t.Error("should not detect circular dependency when no deps exist")
	}

	_ = task1.AddDependency(task2.ID)

	if !project.HasCircularDependency(task2.ID, task1.ID) {
		t.Error("should detect circular dependency: task2 -> task1 -> task2")
	}

	if project.HasCircularDependency(task2.ID, task3.ID) {
		t.Error("should not detect circular dependency for task2 -> task3")
	}

	_ = task2.AddDependency(task3.ID)
	if !project.HasCircularDependency(task3.ID, task1.ID) {
		t.Error("should detect transitive circular dependency: task3 -> task1 -> task2 -> task3")
	}
}

func TestProjectHasCircularDependencyDeepChain(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	tasks := make([]*Task, 5)
	for i := range 5 {
		tasks[i] = newDependencyTestTask(t, fmt.Sprintf("%d", i), i+1, fmt.Sprintf("Task %d", i))
	}
	project.Tasks = tasks

	_ = tasks[0].AddDependency(tasks[1].ID)
	_ = tasks[1].AddDependency(tasks[2].ID)
	_ = tasks[2].AddDependency(tasks[3].ID)
	_ = tasks[3].AddDependency(tasks[4].ID)

	if !project.HasCircularDependency(tasks[4].ID, tasks[0].ID) {
		t.Error("should detect 5-node circular dependency")
	}

	if !project.HasCircularDependency(tasks[4].ID, tasks[2].ID) {
		t.Error("should detect cycle 4 -> 2 -> 3 -> 4")
	}

	if !project.HasCircularDependency(tasks[4].ID, tasks[3].ID) {
		t.Error("should detect cycle 4 -> 3 -> 4")
	}

	if project.HasCircularDependency(tasks[1].ID, tasks[4].ID) {
		t.Error("should not detect cycle for 1 -> 4")
	}
}

func TestProjectHasCircularDependencyBranching(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	tasks := make([]*Task, 4)
	for i := range 4 {
		tasks[i] = newDependencyTestTask(t, fmt.Sprintf("%d", i), i+1, fmt.Sprintf("Task %d", i))
	}
	project.Tasks = tasks

	_ = tasks[0].AddDependency(tasks[1].ID)
	_ = tasks[0].AddDependency(tasks[2].ID)
	_ = tasks[1].AddDependency(tasks[3].ID)
	_ = tasks[2].AddDependency(tasks[3].ID)

	if !project.HasCircularDependency(tasks[3].ID, tasks[0].ID) {
		t.Error("should detect cycle in branching graph")
	}

	if !project.HasCircularDependency(tasks[3].ID, tasks[1].ID) {
		t.Error("should detect cycle 3 -> 1 -> 3")
	}

	if project.HasCircularDependency(tasks[1].ID, tasks[2].ID) {
		t.Error("should not detect cycle for 1 -> 2")
	}
}

func TestProjectHasCircularDependencyWithNonExistentTask(t *testing.T) {
	project := newDependencyTestProject(t, "test")

	task1 := newDependencyTestTask(t, "1", 1, "Task 1")
	task2 := newDependencyTestTask(t, "2", 2, "Task 2")

	_ = task1.AddDependency(task2.ID)
	task2.DependsOn = append(task2.DependsOn, "non-existent-id")

	project.Tasks = []*Task{task1, task2}

	if !project.HasCircularDependency(task2.ID, task1.ID) {
		t.Error("should detect cycle task2 -> task1 -> task2 despite non-existent dep in chain")
	}

	_ = project.HasCircularDependency("non-existent-id", task1.ID)

	if project.HasCircularDependency(task1.ID, "non-existent-id") {
		t.Error("should not detect cycle when target dependency does not exist")
	}
}