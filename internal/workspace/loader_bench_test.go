package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const (
	representativeProjectCount    = 36
	representativeTasksPerProject = 100
)

type representativeWorkspaceFixture struct {
	root    string
	taskIDs []string
}

func newRepresentativeWorkspaceFixture(tb testing.TB) representativeWorkspaceFixture {
	tb.Helper()
	fixture := representativeWorkspaceFixture{root: tb.TempDir()}
	for projectIndex := range representativeProjectCount {
		slug := fmt.Sprintf("project-%02d", projectIndex)
		path := filepath.Join(fixture.root, "projects", slug)
		if err := os.MkdirAll(path, 0o755); err != nil {
			tb.Fatalf("create project directory: %v", err)
		}
		tasks := make([]*models.Task, 0, representativeTasksPerProject)
		for taskIndex := range representativeTasksPerProject {
			id := fmt.Sprintf("%s-task-%03d", slug, taskIndex)
			taskType := models.TaskTypeTask
			if taskIndex%10 == 0 {
				taskType = models.TaskTypeStory
			}
			task, err := models.NewTask(id, taskIndex+1, taskType, fmt.Sprintf("Representative task %d", taskIndex))
			if err != nil {
				tb.Fatalf("create task: %v", err)
			}
			if taskIndex%10 != 0 {
				parentID := fmt.Sprintf("%s-task-%03d", slug, taskIndex-taskIndex%10)
				task.ParentID = &parentID
				task.DependsOn = []string{parentID}
			}
			tasks = append(tasks, task)
			fixture.taskIDs = append(fixture.taskIDs, id)
		}
		content := storage.WriteTasksMarkdown(slug, tasks)
		if err := os.WriteFile(filepath.Join(path, "tasks.md"), []byte(content), 0o600); err != nil {
			tb.Fatalf("write project: %v", err)
		}
	}
	return fixture
}

func (f representativeWorkspaceFixture) projects() []*models.Project {
	projects := make([]*models.Project, 0, representativeProjectCount)
	for projectIndex := range representativeProjectCount {
		slug := fmt.Sprintf("project-%02d", projectIndex)
		projects = append(projects, &models.Project{
			ID:   slug,
			Slug: slug,
			Name: slug,
			Path: filepath.Join(f.root, "projects", slug),
		})
	}
	return projects
}

func BenchmarkWorkspaceStartup36Projects(b *testing.B) {
	fixture := newRepresentativeWorkspaceFixture(b)
	taskCount := representativeProjectCount * representativeTasksPerProject
	b.ReportMetric(representativeProjectCount, "projects")
	b.ReportMetric(float64(taskCount), "tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		loader := NewLoader()
		snapshot := loader.Build(fixture.projects())
		if len(snapshot.Tasks()) != taskCount || loader.ParseCount() != representativeProjectCount {
			b.Fatalf("startup loaded %d tasks with %d parses", len(snapshot.Tasks()), loader.ParseCount())
		}
	}
}

func BenchmarkWorkspaceRefresh36ProjectsUnchanged(b *testing.B) {
	fixture := newRepresentativeWorkspaceFixture(b)
	loader := NewLoader()
	loader.Build(fixture.projects())
	taskCount := representativeProjectCount * representativeTasksPerProject
	b.ReportMetric(representativeProjectCount, "projects")
	b.ReportMetric(float64(taskCount), "tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		snapshot := loader.Build(fixture.projects())
		if len(snapshot.Tasks()) != taskCount {
			b.Fatalf("refresh loaded %d tasks, want %d", len(snapshot.Tasks()), taskCount)
		}
	}

	if loader.ParseCount() != representativeProjectCount {
		b.Fatalf("unchanged refresh reparsed projects: %d", loader.ParseCount()-representativeProjectCount)
	}
}

func BenchmarkWorkspaceGlobalQueries36Projects(b *testing.B) {
	fixture := newRepresentativeWorkspaceFixture(b)
	snapshot := NewLoader().Build(fixture.projects())
	b.ReportMetric(representativeProjectCount, "projects")
	b.ReportMetric(float64(len(fixture.taskIDs)), "tasks")
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		found := 0
		for _, taskID := range fixture.taskIDs {
			if _, status := snapshot.LookupTaskID(taskID); status == TaskLookupUnique {
				found++
			}
			_ = snapshot.QualifiedRef(taskID)
			_ = snapshot.ReverseDependencies(taskID)
		}
		if found != len(fixture.taskIDs) {
			b.Fatalf("global lookup found %d tasks, want %d", found, len(fixture.taskIDs))
		}
	}
}
