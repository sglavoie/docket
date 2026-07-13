package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sglavoie/tt/internal/storage"
)

// Session 22 reliability gate: exercise real, separate `tt` processes mutating
// one workspace at the same time — the true production concurrency model (a CLI
// invocation racing another CLI invocation, or the TUI, which drives the same
// command/commit boundary internally). The workspace commit lock must keep
// tasks.md always well-formed no matter how the processes interleave.

// buildTTBinary compiles the tt CLI once and returns its path. The binary lives
// under the test's temp dir, so it is cleaned up automatically.
func buildTTBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "tt")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/sglavoie/tt").CombinedOutput()
	if err != nil {
		t.Fatalf("build tt binary: %v\n%s", err, out)
	}
	return bin
}

// runTT runs the built tt binary against a fully isolated workspace, returning
// combined output and any run error. HOME and TT_DATA_DIR both point at dataDir
// so config, project data, and the transaction journal (and its workspace commit
// lock) are all co-located under one temp directory that every process shares.
func runTT(t *testing.T, bin, dataDir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "TT_DATA_DIR="+dataDir, "HOME="+dataDir)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// findTasksFile locates the tasks.md a project slug was written to under the
// isolated data directory, regardless of the exact on-disk layout.
func findTasksFile(t *testing.T, dataDir, slug string) string {
	t.Helper()
	var found string
	_ = filepath.WalkDir(dataDir, func(p string, d fs.DirEntry, err error) error {
		// A walk error on one entry just skips it (short-circuits before d is used,
		// which may be nil on error); we are scanning best-effort for the slug dir.
		if err != nil || found != "" || !d.IsDir() || d.Name() != slug {
			return nil //nolint:nilerr // a per-entry walk error means skip that entry, not abort
		}
		cand := filepath.Join(p, "tasks.md")
		if _, statErr := os.Stat(cand); statErr == nil {
			found = cand
		}
		return nil
	})
	if found == "" {
		t.Fatalf("tasks.md for slug %q not found under %s", slug, dataDir)
	}
	return found
}

// mustParseTasks reads and parses a tasks.md, failing if it is missing or the
// document does not parse — i.e. if a concurrent write left it torn or corrupt.
func mustParseTasks(t *testing.T, path string) *storage.ProjectDocument {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read tasks file: %v", err)
	}
	doc, err := storage.ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("tasks.md must always parse under concurrent writes, but it is corrupt: %v\n---\n%s", err, content)
	}
	return doc
}

func TestReliability_ConcurrentCLIMutations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess reliability test in -short mode")
	}
	bin := buildTTBinary(t)

	t.Run("concurrent adds never corrupt tasks.md", func(t *testing.T) {
		dataDir := t.TempDir()
		if out, err := runTT(t, bin, dataDir, "project", "new", "Conc"); err != nil {
			t.Fatalf("project new: %v\n%s", err, out)
		}
		tasksPath := findTasksFile(t, dataDir, "conc")

		const workers = 12
		var wg sync.WaitGroup
		type outcome struct {
			ok  bool
			msg string
		}
		outcomes := make([]outcome, workers)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				out, err := runTT(t, bin, dataDir, "task", "add", fmt.Sprintf("concurrent-task-%d", i))
				outcomes[i] = outcome{ok: err == nil, msg: fmt.Sprintf("worker %d: %v\n%s", i, err, out)}
			}(i)
		}
		wg.Wait()

		// Revision threading landed at the CLI application-service boundary
		// (sessions 27+): `task add` now commits with the loaded project's
		// ExpectedRevision and retries a bounded number of times on a stale-revision
		// conflict. Under heavy contention some adds may still exhaust their retries
		// and fail — but they fail LOUDLY with a stale-revision error rather than
		// silently clobbering a concurrent writer. The guarantee this gate now
		// enforces is stronger than the old last-write-wins one: no corruption AND no
		// lost updates — every add that reports success is present in the file.
		successes := 0
		for _, o := range outcomes {
			if o.ok {
				successes++
				continue
			}
			// A failure is only acceptable when it is the optimistic-concurrency
			// conflict surfacing (retries exhausted), not a corruption or crash.
			if !strings.Contains(o.msg, "changed on disk") && !strings.Contains(o.msg, "retry") {
				t.Errorf("concurrent add failed for a non-conflict reason: %s", o.msg)
			}
		}

		doc := mustParseTasks(t, tasksPath)
		if successes < 1 {
			t.Fatalf("expected at least one successful concurrent add, got %d", successes)
		}
		// No lost updates: the surviving task count must equal the number of adds
		// that reported success. A revision-checked commit only lands when the
		// committer observed the latest revision, so every success accumulates on
		// top of the prior ones.
		if len(doc.Tasks) != successes {
			t.Fatalf("lost update: %d adds succeeded but %d tasks survived", successes, len(doc.Tasks))
		}
		for _, task := range doc.Tasks {
			if !strings.HasPrefix(task.Title, "concurrent-task-") {
				t.Fatalf("unexpected task title %q; file was written by a foreign writer", task.Title)
			}
		}
		t.Logf("concurrent CLI adds: %d/%d succeeded and all survived (revision-checked; no lost updates, no corruption)", successes, workers)
	})

	t.Run("mixed adds, completes, and priority changes never corrupt tasks.md", func(t *testing.T) {
		dataDir := t.TempDir()
		if out, err := runTT(t, bin, dataDir, "project", "new", "Mixed"); err != nil {
			t.Fatalf("project new: %v\n%s", err, out)
		}
		// Seed tasks 1..seed so the mutating workers have stable targets.
		const seed = 6
		for i := 0; i < seed; i++ {
			if out, err := runTT(t, bin, dataDir, "task", "add", fmt.Sprintf("seed-%d", i)); err != nil {
				t.Fatalf("seed add: %v\n%s", err, out)
			}
		}
		tasksPath := findTasksFile(t, dataDir, "mixed")

		// Concurrent, heterogeneous mutations model the CLI and the TUI (which
		// drives the same commands) mutating at once: some processes add tasks,
		// others complete or re-prioritize the seeded tasks by short id.
		var wg sync.WaitGroup
		type job struct{ args []string }
		jobs := []job{}
		for i := 0; i < seed; i++ {
			jobs = append(jobs, job{[]string{"task", "add", fmt.Sprintf("added-%d", i)}})
			jobs = append(jobs, job{[]string{"task", "complete", fmt.Sprintf("%d", i+1)}})
			jobs = append(jobs, job{[]string{"task", "priority", fmt.Sprintf("%d", i+1), "1"}})
		}
		for _, j := range jobs {
			wg.Add(1)
			go func(args []string) {
				defer wg.Done()
				// A mutation may legitimately fail (e.g. a short id already
				// completed by a racing process); the invariant under test is file
				// integrity, so run errors are tolerated but the file is checked.
				_, _ = runTT(t, bin, dataDir, args...)
			}(j.args)
		}
		wg.Wait()

		doc := mustParseTasks(t, tasksPath)
		if len(doc.Tasks) < seed {
			t.Fatalf("seeded tasks must survive heterogeneous concurrent mutation; got %d, want >= %d", len(doc.Tasks), seed)
		}
		// A final serial add must still succeed and land, proving the workspace is
		// left in a consistent, writable state after the concurrent storm.
		if out, err := runTT(t, bin, dataDir, "task", "add", "final-serial"); err != nil {
			t.Fatalf("post-storm serial add failed: %v\n%s", err, out)
		}
		after := mustParseTasks(t, tasksPath)
		if len(after.Tasks) != len(doc.Tasks)+1 {
			t.Fatalf("post-storm add did not land cleanly: before=%d after=%d", len(doc.Tasks), len(after.Tasks))
		}
	})
}
