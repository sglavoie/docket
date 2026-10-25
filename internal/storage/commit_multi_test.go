package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/models"
)

// TestCommitService_CommitMulti_WritesAllProjectsAndArtifacts confirms a
// multi-project changeset commits every project's markdown plus heterogeneous
// artifacts (a config store, a note, a blocker record, an archive file, and a
// derived index) atomically in one transaction, returning one result per
// changeset that all share the same transaction id.
func TestCommitResultTransactionIDSharedByCommitMulti(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha task")
	beta := newCommitTestProject(t, dataDir, "beta", "beta task")

	planning := filepath.Join(dataDir, ".tt", "planning.json")
	note := filepath.Join(dataDir, "projects", "alpha", "notes", "n1.md")
	blocker := filepath.Join(dataDir, "projects", "beta", "blockers.md")
	archive := filepath.Join(dataDir, "projects", "alpha", "archive.md")
	index := filepath.Join(dataDir, ".tt", "index.json")

	results, err := svc.CommitMulti([]ChangeSet{
		{Project: alpha, Artifacts: []ArtifactChange{
			{Path: planning, Data: []byte(`{"queue":[]}`)},
			{Path: note, Data: []byte("note body")},
			{Path: archive, Data: []byte("# Archive")},
		}},
		{Project: beta, Artifacts: []ArtifactChange{
			{Path: blocker, Data: []byte("blocked on X")},
			{Path: index, Data: []byte(`{"v":1}`)},
		}},
	})
	if err != nil {
		t.Fatalf("CommitMulti: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Revision == "" || results[1].Revision == "" {
		t.Fatalf("expected non-empty revisions, got %+v", results)
	}
	if results[0].TransactionID == "" || results[0].TransactionID != results[1].TransactionID {
		t.Fatalf("grouped commit transaction IDs = %q and %q, want one shared non-empty ID", results[0].TransactionID, results[1].TransactionID)
	}

	// Both projects' markdown parses back to the committed task.
	for slug, want := range map[string]string{"alpha": "alpha task", "beta": "beta task"} {
		loaded, err := models.NewProject(uuid.New().String(), slug, "")
		if err != nil {
			t.Fatal(err)
		}
		loaded.Path = filepath.Join(dataDir, "projects", slug)
		if err := LoadProject(loaded); err != nil {
			t.Fatalf("LoadProject %s: %v", slug, err)
		}
		if len(loaded.Tasks) != 1 || loaded.Tasks[0].Title != want {
			t.Fatalf("%s loaded tasks = %+v, want single %q", slug, loaded.Tasks, want)
		}
	}

	// Every heterogeneous artifact landed with its content.
	for path, want := range map[string]string{
		planning: `{"queue":[]}`,
		note:     "note body",
		blocker:  "blocked on X",
		archive:  "# Archive",
		index:    `{"v":1}`,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read artifact %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("artifact %s = %q, want %q", path, got, want)
		}
	}

	assertJournalClean(t, dataDir)
}

// TestCommitService_CommitMulti_CrossProjectMoveIsAtomic commits a representative
// cross-project move — a task removed from the source project and added to the
// destination — through one transaction, confirming both sides land together.
func TestCommitService_CommitMulti_CrossProjectMoveIsAtomic(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "moving task")
	dst := newCommitTestProject(t, dataDir, "dst", "existing task")

	// Seed both projects at a known revision.
	seed, err := svc.CommitMulti([]ChangeSet{{Project: src}, {Project: dst}})
	if err != nil {
		t.Fatalf("seed CommitMulti: %v", err)
	}

	// Perform the move in memory: the task leaves src and joins dst.
	moved := src.Tasks[0]
	src.Tasks = nil
	src.BuildTaskTree()
	moved.ShortID = dst.GetNextShortID()
	dst.Tasks = append(dst.Tasks, moved)
	dst.NextShortID = dst.GetNextShortID() + 1
	dst.BuildTaskTree()

	if _, err := svc.CommitMulti([]ChangeSet{
		{Project: src, ExpectedRevision: seed[0].Revision},
		{Project: dst, ExpectedRevision: seed[1].Revision},
	}); err != nil {
		t.Fatalf("move CommitMulti: %v", err)
	}

	loadedSrc, _ := models.NewProject(uuid.New().String(), "src", "")
	loadedSrc.Path = src.Path
	if err := LoadProject(loadedSrc); err != nil {
		t.Fatalf("LoadProject src: %v", err)
	}
	if len(loadedSrc.Tasks) != 0 {
		t.Fatalf("src still has %d tasks after move", len(loadedSrc.Tasks))
	}

	loadedDst, _ := models.NewProject(uuid.New().String(), "dst", "")
	loadedDst.Path = dst.Path
	if err := LoadProject(loadedDst); err != nil {
		t.Fatalf("LoadProject dst: %v", err)
	}
	if len(loadedDst.Tasks) != 2 {
		t.Fatalf("dst has %d tasks after move, want 2", len(loadedDst.Tasks))
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_CommitMulti_StaleRevisionAtEveryPositionAbortsAll drives a
// stale expected revision through every position of a three-project changeset and
// confirms the whole set is aborted before any project is modified — proving the
// preflight prevents all writes regardless of which project conflicts.
func TestCommitService_CommitMulti_StaleRevisionAtEveryPositionAbortsAll(t *testing.T) {
	for pos := 0; pos < 3; pos++ {
		t.Run(fmt.Sprintf("stale-at-%d", pos), func(t *testing.T) {
			svc, dataDir := newCommitTestService(t)
			projects := []*models.Project{
				newCommitTestProject(t, dataDir, "p0", "p0 v1"),
				newCommitTestProject(t, dataDir, "p1", "p1 v1"),
				newCommitTestProject(t, dataDir, "p2", "p2 v1"),
			}

			// Seed all three and capture their revisions.
			seedSets := make([]ChangeSet, len(projects))
			for i, p := range projects {
				seedSets[i] = ChangeSet{Project: p}
			}
			seed, err := svc.CommitMulti(seedSets)
			if err != nil {
				t.Fatalf("seed CommitMulti: %v", err)
			}
			seedRevs := []string{seed[0].Revision, seed[1].Revision, seed[2].Revision}

			// A concurrent change advances the on-disk revision of the project at
			// pos, so its expected (seed) revision is now stale.
			conflictProject := projects[pos]
			extra, err := models.NewGenericTask(uuid.New().String(), 99, "out of band")
			if err != nil {
				t.Fatal(err)
			}
			conflictProject.Tasks = append(conflictProject.Tasks, extra)
			conflictProject.NextShortID = 100
			conflictProject.BuildTaskTree()
			if _, commitErr := svc.Commit(ChangeSet{Project: conflictProject, ExpectedRevision: seedRevs[pos]}); commitErr != nil {
				t.Fatalf("concurrent Commit: %v", commitErr)
			}
			advancedRev := onDiskRevision(t, conflictProject)

			// Now attempt a multi-commit that mutates all three, using the stale
			// seed revision for every project. Give each a new title so we can
			// detect any leaked write.
			for _, p := range projects {
				p.Tasks[0].Title = p.Slug + " v2"
			}
			commitSets := make([]ChangeSet, len(projects))
			for i, p := range projects {
				commitSets[i] = ChangeSet{Project: p, ExpectedRevision: seedRevs[i]}
			}
			_, err = svc.CommitMulti(commitSets)
			if !errors.Is(err, ErrStaleRevision) {
				t.Fatalf("CommitMulti err = %v, want ErrStaleRevision", err)
			}

			// No project's on-disk title advanced to v2: the whole set was aborted.
			for i, p := range projects {
				loaded, _ := models.NewProject(uuid.New().String(), p.Slug, "")
				loaded.Path = p.Path
				if err := LoadProject(loaded); err != nil {
					t.Fatalf("LoadProject %s: %v", p.Slug, err)
				}
				if loaded.Tasks[0].Title == p.Slug+" v2" {
					t.Fatalf("project %s (index %d) was written despite aborted commit", p.Slug, i)
				}
			}
			// The conflicting project kept its concurrently-advanced revision.
			if got := onDiskRevision(t, conflictProject); got != advancedRev {
				t.Fatalf("conflict project revision changed to %s after aborted commit", got)
			}
			assertJournalClean(t, dataDir)
		})
	}
}

// TestCommitService_CommitMulti_BadTargetAtEveryPositionAbortsAll injects an
// artifact whose target resolves outside the approved workspace roots at every
// changeset position and confirms Prepare rejects it before any project is
// written — a failed target preflight prevents all writes wherever it occurs.
func TestCommitService_CommitMulti_BadTargetAtEveryPositionAbortsAll(t *testing.T) {
	for pos := 0; pos < 3; pos++ {
		t.Run(fmt.Sprintf("bad-target-at-%d", pos), func(t *testing.T) {
			svc, dataDir := newCommitTestService(t)
			projects := []*models.Project{
				newCommitTestProject(t, dataDir, "q0", "q0"),
				newCommitTestProject(t, dataDir, "q1", "q1"),
				newCommitTestProject(t, dataDir, "q2", "q2"),
			}

			// A target well outside the workspace root fails validation in Prepare.
			outside := filepath.Join(t.TempDir(), "escaped.json")
			commitSets := make([]ChangeSet, len(projects))
			for i, p := range projects {
				cs := ChangeSet{Project: p}
				if i == pos {
					cs.Artifacts = []ArtifactChange{{Path: outside, Data: []byte("nope")}}
				}
				commitSets[i] = cs
			}

			if _, err := svc.CommitMulti(commitSets); err == nil {
				t.Fatalf("CommitMulti with out-of-root target at %d unexpectedly succeeded", pos)
			}

			// No project was written and the offending artifact never landed.
			for _, p := range projects {
				if commitProjectExists(p) {
					t.Fatalf("project %s written despite rejected commit", p.Slug)
				}
			}
			if _, statErr := os.Stat(outside); !os.IsNotExist(statErr) {
				t.Fatalf("out-of-root artifact was written: stat err = %v", statErr)
			}
			assertJournalClean(t, dataDir)
		})
	}
}

// TestCommitService_CommitMulti_RejectsDuplicateProject confirms a changeset set
// that names the same project twice is rejected before any write, since two
// writes to one tasks.md in a single transaction would silently race.
func TestCommitService_CommitMulti_RejectsDuplicateProject(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	p := newCommitTestProject(t, dataDir, "dup", "dup")

	_, err := svc.CommitMulti([]ChangeSet{{Project: p}, {Project: p}})
	if err == nil {
		t.Fatal("expected duplicate-project commit to be rejected")
	}
	if commitProjectExists(p) {
		t.Fatal("project written despite rejected duplicate commit")
	}
}
