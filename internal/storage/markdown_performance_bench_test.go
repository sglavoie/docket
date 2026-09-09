package storage

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func BenchmarkWriteProjectMarkdownLargeHierarchy(b *testing.B) {
	const taskCount = 5000
	tasks := make([]*models.Task, 0, taskCount)
	for i := range taskCount {
		id := fmt.Sprintf("task-%05d", i)
		task, err := models.NewGenericTask(id, i+1, fmt.Sprintf("Large hierarchy task %d", i))
		if err != nil {
			b.Fatalf("create task: %v", err)
		}
		if i%50 != 0 {
			parentID := fmt.Sprintf("task-%05d", i-i%50)
			task.ParentID = &parentID
		}
		tasks = append(tasks, task)
	}
	b.ReportMetric(taskCount, "tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		output, err := WriteProjectMarkdown(&models.Project{Name: "large-hierarchy", Tasks: tasks})
		if err != nil {
			b.Fatal(err)
		}
		if len(output) == 0 {
			b.Fatal("serializer produced empty output")
		}
	}
}
