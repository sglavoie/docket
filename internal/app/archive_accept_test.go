package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func archiveProject(t *testing.T, id string, completed int) *models.Project {
	t.Helper()
	project := newProject(id, id)
	project.Path = filepath.Join(t.TempDir(), id)
	if err := os.MkdirAll(project.Path, 0o700); err != nil {
		t.Fatalf("create project directory: %v", err)
	}
	for i := range completed {
		task := newTask(id+"-done-"+string(rune('a'+i)), i+1, "Done")
		task.Complete()
		project.Tasks = append(project.Tasks, task)
	}
	project.Tasks = append(project.Tasks, newTask(id+"-ready", completed+1, "Ready"))
	project.BuildTaskTree()
	return project
}

func TestArchiveTasksCommitsArchiveEffectsAndCallerArtifacts(t *testing.T) {
	project := archiveProject(t, "alpha", 2)
	repo := newFakeRepo(project)
	service := newTestService(repo)
	callerArtifact := storage.ArtifactChange{Path: filepath.Join(t.TempDir(), "planning.json"), Data: []byte("planning")}

	result, err := service.ArchiveTasks(project.ID, callerArtifact)
	if err != nil {
		t.Fatalf("ArchiveTasks: %v", err)
	}
	if result.ProjectID != project.ID || result.ProjectSlug != project.Slug || result.ProjectName != project.Name {
		t.Fatalf("result identity = %+v", result)
	}
	if result.ArchivedCount != 2 || result.Revision == "" {
		t.Fatalf("result = %+v, want two archived tasks and a revision", result)
	}
	if len(project.Tasks) != 1 || project.Tasks[0].ID != "alpha-ready" {
		t.Fatalf("active tasks after archive = %+v", project.Tasks)
	}

	commit, ok := repo.lastCommit()
	if !ok {
		t.Fatal("archive should commit exactly one changeset")
	}
	if commit.UndoAction != "Archive Tasks" || commit.UndoContext != "2 tasks" {
		t.Fatalf("undo metadata = %q/%q", commit.UndoAction, commit.UndoContext)
	}
	if len(commit.AttachmentOps) != 2 {
		t.Fatalf("attachment ops = %d, want 2", len(commit.AttachmentOps))
	}
	foundCallerArtifact := false
	for _, artifact := range commit.Artifacts {
		if artifact.Path == callerArtifact.Path {
			foundCallerArtifact = true
		}
	}
	if !foundCallerArtifact {
		t.Fatal("caller artifact was not folded into the archive transaction")
	}
}

func TestArchiveTasksNoopAndFailures(t *testing.T) {
	t.Run("no completed tasks", func(t *testing.T) {
		project := archiveProject(t, "empty", 0)
		repo := newFakeRepo(project)
		result, err := newTestService(repo).ArchiveTasks(project.ID)
		if err != nil {
			t.Fatalf("ArchiveTasks: %v", err)
		}
		if result.ArchivedCount != 0 || len(repo.commits) != 0 {
			t.Fatalf("noop result = %+v, commits = %d", result, len(repo.commits))
		}
	})

	t.Run("snapshot error", func(t *testing.T) {
		repo := newFakeRepo()
		repo.snapErr = errors.New("snapshot failed")
		if _, err := newTestService(repo).ArchiveTasks("missing"); err == nil {
			t.Fatal("expected snapshot error")
		}
	})

	t.Run("project missing", func(t *testing.T) {
		if _, err := newTestService(newFakeRepo()).ArchiveTasks("missing"); !errors.Is(err, app.ErrProjectNotFound) {
			t.Fatalf("error = %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("archive planning error", func(t *testing.T) {
		project := archiveProject(t, "bad-path", 1)
		badPath := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(badPath, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		project.Path = badPath
		if _, err := newTestService(newFakeRepo(project)).ArchiveTasks(project.ID); err == nil {
			t.Fatal("expected archive planning error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		project := archiveProject(t, "commit-failure", 1)
		repo := newFakeRepo(project)
		repo.commitErr = errors.New("commit failed")
		if _, err := newTestService(repo).ArchiveTasks(project.ID); err == nil {
			t.Fatal("expected commit error")
		}
	})
}

func TestArchiveAllTasksCommitsOneMultiProjectTransaction(t *testing.T) {
	alpha := archiveProject(t, "alpha-all", 1)
	beta := archiveProject(t, "beta-all", 2)
	ready := archiveProject(t, "ready-all", 0)
	repo := newFakeRepo(alpha, beta, ready)
	service := newTestService(repo)
	callerArtifact := storage.ArtifactChange{Path: filepath.Join(t.TempDir(), "planning.json"), Data: []byte("planning")}

	result, err := service.ArchiveAllTasks([]string{alpha.ID, ready.ID, beta.ID}, callerArtifact)
	if err != nil {
		t.Fatalf("ArchiveAllTasks: %v", err)
	}
	if result.TotalArchived != 3 || len(result.Projects) != 2 {
		t.Fatalf("result = %+v", result)
	}
	if len(result.TouchedProjectIDs) != 2 || result.TouchedProjectIDs[0] != alpha.ID || result.TouchedProjectIDs[1] != beta.ID {
		t.Fatalf("touched projects = %v", result.TouchedProjectIDs)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commit shape = %d calls / %d changesets", len(repo.commits), len(repo.commits[0]))
	}
	if repo.commits[0][0].UndoContext != "3 tasks across 2 projects" || repo.commits[0][1].UndoContext != "3 tasks across 2 projects" {
		t.Fatalf("undo contexts = %q/%q", repo.commits[0][0].UndoContext, repo.commits[0][1].UndoContext)
	}
	if result.Projects[0].Revision == "" || result.Projects[1].Revision == "" {
		t.Fatal("archive-all results must carry revisions")
	}
	foundCallerArtifact := false
	for _, artifact := range repo.commits[0][0].Artifacts {
		if artifact.Path == callerArtifact.Path {
			foundCallerArtifact = true
		}
	}
	if !foundCallerArtifact {
		t.Fatal("caller artifact should be attached to the first changeset")
	}
}

func TestArchiveAllTasksNoopAndFailures(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		result, err := newTestService(newFakeRepo()).ArchiveAllTasks(nil)
		if err != nil || result.TotalArchived != 0 {
			t.Fatalf("result = %+v, err = %v", result, err)
		}
	})

	t.Run("snapshot error", func(t *testing.T) {
		repo := newFakeRepo()
		repo.snapErr = errors.New("snapshot failed")
		if _, err := newTestService(repo).ArchiveAllTasks([]string{"p"}); err == nil {
			t.Fatal("expected snapshot error")
		}
	})

	t.Run("missing project", func(t *testing.T) {
		if _, err := newTestService(newFakeRepo()).ArchiveAllTasks([]string{"missing"}); !errors.Is(err, app.ErrProjectNotFound) {
			t.Fatalf("error = %v, want ErrProjectNotFound", err)
		}
	})

	t.Run("nothing completed", func(t *testing.T) {
		ready := archiveProject(t, "ready", 0)
		repo := newFakeRepo(ready)
		result, err := newTestService(repo).ArchiveAllTasks([]string{ready.ID})
		if err != nil || result.TotalArchived != 0 || len(repo.commits) != 0 {
			t.Fatalf("result = %+v, err = %v, commits = %d", result, err, len(repo.commits))
		}
	})

	t.Run("planning error", func(t *testing.T) {
		project := archiveProject(t, "bad-all", 1)
		badPath := filepath.Join(t.TempDir(), "not-a-directory")
		if err := os.WriteFile(badPath, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		project.Path = badPath
		if _, err := newTestService(newFakeRepo(project)).ArchiveAllTasks([]string{project.ID}); err == nil {
			t.Fatal("expected planning error")
		}
	})

	t.Run("commit error", func(t *testing.T) {
		project := archiveProject(t, "commit-all", 1)
		repo := newFakeRepo(project)
		repo.commitErr = errors.New("commit failed")
		if _, err := newTestService(repo).ArchiveAllTasks([]string{project.ID}); err == nil {
			t.Fatal("expected commit error")
		}
	})
}
