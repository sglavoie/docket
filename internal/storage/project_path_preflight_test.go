package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/transaction"
)

func TestCommitMultiRejectsProjectPathAtEveryPositionBeforeEffects(t *testing.T) {
	for pos := 0; pos < 3; pos++ {
		t.Run(fmt.Sprintf("bad-project-at-%d", pos), func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "workspace")
			transactionsDir := filepath.Join(root, ".tt", "transactions")
			svc := NewCommitService(transactionsDir, []string{root})
			projects := make([]ChangeSet, 3)
			for i := range projects {
				project := newCommitTestProject(t, root, fmt.Sprintf("p%d", i), "task")
				if i == pos {
					project.Path = filepath.Join(base, "outside", fmt.Sprintf("p%d", i))
				}
				projects[i] = ChangeSet{Project: project}
			}

			_, err := svc.CommitMulti(projects)
			if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
				t.Fatalf("CommitMulti error = %v, want ErrTargetOutsideRoot", err)
			}
			for _, path := range []string{root, transactionsDir, filepath.Join(base, "outside")} {
				if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("rejected project path created %q: %v", path, statErr)
				}
			}
		})
	}
}

func TestCommitRejectsTraversalProjectPathBeforeEffects(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	project := newCommitTestProject(t, root, "escape", "task")
	project.Path = filepath.Join(root, "projects", "..", "..", "outside")
	transactionsDir := filepath.Join(root, ".tt", "transactions")

	_, err := NewCommitService(transactionsDir, []string{root}).Commit(ChangeSet{Project: project})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("Commit error = %v, want ErrTargetOutsideRoot", err)
	}
	if _, statErr := os.Lstat(root); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("traversal rejection created workspace effects: %v", statErr)
	}
}

func TestCommitRejectsExistingProjectSymlinkEscapesBeforeEffects(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is unreliable on Windows CI")
	}

	for _, role := range []string{"directory", "tasks", "backup", "lock"} {
		t.Run(role, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			project := newCommitTestProject(t, root, "alpha", "task")
			if err := os.MkdirAll(filepath.Dir(project.Path), DirPermissions); err != nil {
				t.Fatal(err)
			}
			sentinelPath := filepath.Join(outside, "sentinel")
			if role == "directory" {
				if err := os.Symlink(outside, project.Path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			} else {
				if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(sentinelPath, []byte("outside sentinel"), FilePermissions); err != nil {
					t.Fatal(err)
				}
				linkPath := map[string]string{
					"tasks":  tasksFilePath(project.Path),
					"backup": backupFilePath(project.Path),
					"lock":   filepath.Join(project.Path, LockFileName),
				}[role]
				if err := os.Symlink(sentinelPath, linkPath); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}

			transactionsDir := filepath.Join(root, ".tt", "transactions")
			_, err := NewCommitService(transactionsDir, []string{root}).Commit(ChangeSet{Project: project})
			if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
				t.Fatalf("Commit error = %v, want ErrTargetOutsideRoot", err)
			}
			if _, statErr := os.Lstat(transactionsDir); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("symlink rejection created transactions directory: %v", statErr)
			}
			if role != "directory" {
				got, readErr := os.ReadFile(sentinelPath)
				if readErr != nil || string(got) != "outside sentinel" {
					t.Fatalf("external target changed: data=%q err=%v", got, readErr)
				}
			}
		})
	}
}

func TestCommitMultiRejectsSymlinkAliasOfSameProject(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is unreliable on Windows CI")
	}
	root := t.TempDir()
	realPath := filepath.Join(root, "projects", "real")
	aliasPath := filepath.Join(root, "projects", "alias")
	if err := os.MkdirAll(realPath, DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPath, aliasPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	realProject := newCommitTestProject(t, root, "real", "real")
	aliasProject := newCommitTestProject(t, root, "alias", "alias")
	realProject.Path = realPath
	aliasProject.Path = aliasPath
	transactionsDir := filepath.Join(root, ".tt", "transactions")

	_, err := NewCommitService(transactionsDir, []string{root}).CommitMulti([]ChangeSet{
		{Project: realProject},
		{Project: aliasProject},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicates project path") {
		t.Fatalf("CommitMulti error = %v, want duplicate project path", err)
	}
	for _, path := range []string{transactionsDir, tasksFilePath(realPath), filepath.Join(realPath, LockFileName)} {
		if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("duplicate alias rejection created %q: %v", path, statErr)
		}
	}
}
