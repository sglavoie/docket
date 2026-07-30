package models

import (
	"fmt"
	"testing"
)

func BenchmarkBuildTaskTreeLargeProject(b *testing.B) {
	const taskCount = 5000
	project := &Project{ID: "large", Name: "Large", Tasks: make([]*Task, 0, taskCount)}
	for i := range taskCount {
		id := fmt.Sprintf("task-%05d", i)
		task, err := NewGenericTask(id, i+1, fmt.Sprintf("Large hierarchy task %d", i))
		if err != nil {
			b.Fatalf("create task: %v", err)
		}
		if i%50 != 0 {
			parentID := fmt.Sprintf("task-%05d", i-i%50)
			task.ParentID = &parentID
			task.DependsOn = []string{parentID}
		}
		project.Tasks = append(project.Tasks, task)
	}
	b.ReportMetric(taskCount, "tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		project.BuildTaskTree()
	}
}
