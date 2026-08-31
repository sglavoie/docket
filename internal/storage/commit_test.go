package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/transaction"
)

// seedTaskAttachment creates the on-disk attachments directory for a task under a
// project and writes a single file with the given bytes, returning the file's path.
func seedTaskAttachment(t *testing.T, project *models.Project, task *models.Task, name string, data []byte) string {
	t.Helper()
	dir := GetTaskAttachmentsDir(project, task)
	if err := os.MkdirAll(dir, DirPermissions); err != nil {
		t.Fatalf("mkdir attachments: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write attachment: %v", err)
	}
	return path
}

// newCommitTestProject builds a minimal single-task project rooted under a
// canonical <data>/projects/<slug> layout inside dataDir.
func newCommitTestProject(t *testing.T, dataDir, slug, title string) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	p.Slug = slug
	p.Path = filepath.Join(dataDir, "projects", slug)
	task, err := models.NewGenericTask(uuid.New().String(), 1, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	p.Tasks = []*models.Task{task}
	p.NextShortID = 2
	p.BuildTaskTree()
	return p
}

func newCommitTestService(t *testing.T) (*CommitService, string) {
	t.Helper()
	dataDir := t.TempDir()
	svc := NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})
	return svc, dataDir
}

// onDiskRevision parses a project's committed tasks.md and returns its canonical
// revision.
func onDiskRevision(t *testing.T, project *models.Project) string {
	t.Helper()
	content, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read tasks file: %v", err)
	}
	doc, err := ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("parse committed doc: %v", err)
	}
	return doc.Revision
}

// TestCommitService_WritesProjectAndArtifact confirms a changeset commits the
// project markdown and an associated configuration artifact atomically, and
// reports the new revision.
func TestCommitResultTransactionIDAndWritesProjectAndArtifact(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "write me")

	artifactPath := filepath.Join(dataDir, ".tt", "planning.json")
	result, err := svc.Commit(ChangeSet{
		Project:   project,
		Artifacts: []ArtifactChange{{Path: artifactPath, Data: []byte(`{"queue":[]}`)}},
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if result.Revision == "" {
		t.Fatal("expected a non-empty revision")
	}
	if result.TransactionID == "" {
		t.Fatal("expected a non-empty transaction ID")
	}

	// The project markdown was written and parses back to the committed task.
	loaded, err := models.NewProject(project.ID, project.Slug, "")
	if err != nil {
		t.Fatal(err)
	}
	loaded.Path = project.Path
	if loadErr := LoadProject(loaded); loadErr != nil {
		t.Fatalf("LoadProject: %v", loadErr)
	}
	if len(loaded.Tasks) != 1 || loaded.Tasks[0].Title != "write me" {
		t.Fatalf("loaded tasks = %+v, want single 'write me'", loaded.Tasks)
	}

	// The configuration artifact was committed in the same transaction.
	got, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(got) != `{"queue":[]}` {
		t.Fatalf("artifact content = %q", got)
	}

	// The journal is clean: no leftover manifests after a successful commit.
	entries, _ := os.ReadDir(filepath.Join(dataDir, ".tt", "transactions"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			t.Fatalf("unexpected leftover manifest: %s", e.Name())
		}
	}
}

// TestCommitService_PreservesLosslessBaseAndReturnsReloadRevision proves the
// transactional user-mutation path writes the schema-v5 envelope from the exact
// loaded base and reports the revision an immediate reload observes. It repeats
// the mutation within one persisted timestamp minute to guard the historical
// in-memory/reload revision divergence.
func TestCommitService_PreservesLosslessBaseAndReturnsReloadRevision(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "lossless", "before")
	project.Tasks[0].CreatedAt = time.Date(2026, 7, 13, 10, 15, 5, 0, time.Local)
	project.Tasks[0].UpdatedAt = time.Date(2026, 7, 13, 10, 15, 5, 0, time.Local)

	seed := ProjectDocumentFromBase(project, nil)
	if err := os.MkdirAll(project.Path, DirPermissions); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(tasksFilePath(project.Path), []byte(mustWriteCurrentDocument(t, seed)), FilePermissions); err != nil {
		t.Fatalf("seed tasks: %v", err)
	}
	base, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("load base: %v", err)
	}

	project.Tasks[0].Title = "first mutation"
	project.Tasks[0].UpdatedAt = time.Date(2026, 7, 13, 10, 15, 40, 0, time.Local)
	first, err := svc.Commit(ChangeSet{Project: project, ExpectedRevision: base.Revision, Base: base})
	if err != nil {
		t.Fatalf("first Commit: %v", err)
	}
	firstBytes, err := os.ReadFile(tasksFilePath(project.Path))
	if err != nil {
		t.Fatalf("read first commit: %v", err)
	}
	firstContent := string(firstBytes)
	if !strings.Contains(firstContent, schemaMarkerV15) {
		t.Fatalf("committed content missing schema marker:\n%s", firstContent)
	}
	if strings.Contains(firstContent, "<!-- future-feature: keep-exactly -->") {
		t.Fatalf("committed schema-7 content retained opaque legacy block:\n%s", firstContent)
	}
	if !strings.Contains(firstContent, "updated:") {
		t.Fatalf("schema-7 commit omitted required updated timestamp:\n%s", firstContent)
	}
	firstReload, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload first commit: %v", err)
	}
	if first.Revision != firstReload.Revision {
		t.Fatalf("first result revision %s != reload %s", first.Revision, firstReload.Revision)
	}
	if strings.Contains(firstContent, "<!-- sections:") {
		t.Fatalf("canonical commit retained legacy section metadata:\n%s", firstContent)
	}
	project.Tasks[0].Title = "second mutation"
	project.Tasks[0].UpdatedAt = time.Date(2026, 7, 13, 10, 15, 55, 0, time.Local)
	second, err := svc.Commit(ChangeSet{Project: project, ExpectedRevision: firstReload.Revision, Base: firstReload})
	if err != nil {
		t.Fatalf("second Commit: %v", err)
	}
	secondReload, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload second commit: %v", err)
	}
	if second.Revision != secondReload.Revision {
		t.Fatalf("second result revision %s != reload %s", second.Revision, secondReload.Revision)
	}
}

// TestCommitService_StaleRevisionConflict confirms a changeset whose expected
// revision no longer matches the on-disk revision is aborted with a
// stale-revision conflict and does not overwrite the concurrent change.
func TestCommitService_StaleRevisionConflict(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "original")

	if _, err := svc.Commit(ChangeSet{Project: project}); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}
	staleRevision := onDiskRevision(t, project)

	// A concurrent change advances the on-disk revision.
	extra, err := models.NewGenericTask(uuid.New().String(), 2, "added out of band")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, extra)
	project.NextShortID = 3
	project.BuildTaskTree()
	if _, commitErr := svc.Commit(ChangeSet{Project: project}); commitErr != nil {
		t.Fatalf("concurrent Commit: %v", commitErr)
	}
	currentRevision := onDiskRevision(t, project)
	if currentRevision == staleRevision {
		t.Fatal("expected the concurrent commit to change the on-disk revision")
	}

	// Committing against the now-stale revision must conflict before writing.
	_, err = svc.Commit(ChangeSet{Project: project, ExpectedRevision: staleRevision})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("Commit err = %v, want ErrStaleRevision", err)
	}
	var conflict *RevisionConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *RevisionConflictError, got %T", err)
	}
	if conflict.Expected != staleRevision || conflict.Actual != currentRevision {
		t.Fatalf("conflict revisions = (%s, %s), want (%s, %s)", conflict.Expected, conflict.Actual, staleRevision, currentRevision)
	}

	// The on-disk file is untouched by the rejected commit.
	if got := onDiskRevision(t, project); got != currentRevision {
		t.Fatalf("on-disk revision changed to %s after rejected commit", got)
	}
}

// TestCommitService_ExpectedRevisionMatchCommits confirms that supplying the
// correct current revision lets the commit proceed.
func TestCommitService_ExpectedRevisionMatchCommits(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "v1")

	if _, err := svc.Commit(ChangeSet{Project: project}); err != nil {
		t.Fatalf("initial Commit: %v", err)
	}
	current := onDiskRevision(t, project)

	extra, err := models.NewGenericTask(uuid.New().String(), 2, "v2 task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, extra)
	project.NextShortID = 3
	project.BuildTaskTree()

	if _, err := svc.Commit(ChangeSet{Project: project, ExpectedRevision: current}); err != nil {
		t.Fatalf("Commit with matching revision: %v", err)
	}
}

// TestCommitService_BlockedByQuarantine confirms a quarantined transaction blocks
// the commit and leaves the project unwritten.
func TestCommitService_BlockedByQuarantine(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "should not write")

	// Plant a corrupt manifest so startup recovery quarantines it.
	txnDir := filepath.Join(dataDir, ".tt", "transactions")
	if err := os.MkdirAll(txnDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(txnDir, "txn-corrupt.json"), []byte("{ not valid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := svc.Commit(ChangeSet{Project: project})
	if !errors.Is(err, ErrCommitBlocked) {
		t.Fatalf("Commit err = %v, want ErrCommitBlocked", err)
	}
	if _, statErr := os.Stat(tasksFilePath(project.Path)); !os.IsNotExist(statErr) {
		t.Fatalf("project written despite blocked commit: stat err = %v", statErr)
	}
}

// commitProjectExists reports whether a project's tasks.md is present on disk.
func commitProjectExists(project *models.Project) bool {
	_, err := os.Stat(tasksFilePath(project.Path))
	return err == nil
}

// assertJournalClean fails if any manifest is left in the transactions directory,
// which would mean a commit did not reclaim its transaction.
func assertJournalClean(t *testing.T, dataDir string) {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(dataDir, ".tt", "transactions"))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			t.Fatalf("unexpected leftover manifest: %s", e.Name())
		}
	}
}

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

// TestCommitService_AttachmentCopyCommitsWithProject confirms an attachment copy
// is staged and committed in the same transaction as the project write, landing the
// ingested bytes atomically with the tasks.md that references them.
func TestCommitService_AttachmentCopyCommitsWithProject(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "has attachment")
	task := project.Tasks[0]

	srcPath := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(srcPath, []byte("pdf-bytes"), 0o600); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	if _, err := svc.Commit(ChangeSet{
		Project:       project,
		AttachmentOps: []AttachmentOp{{Kind: AttachmentCopy, Task: task, DestProject: project, SourcePath: srcPath}},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if !commitProjectExists(project) {
		t.Fatal("project tasks.md not written")
	}
	got, err := os.ReadFile(GetAttachmentPath(project, task, "report.pdf"))
	if err != nil {
		t.Fatalf("read committed attachment: %v", err)
	}
	if string(got) != "pdf-bytes" {
		t.Fatalf("attachment content = %q, want %q", got, "pdf-bytes")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentCopyDuplicateNameAbortsAll confirms copying a file
// whose name already exists in the task's directory fails, and — because attachment
// resolution runs before any bytes are staged — leaves the project unwritten.
func TestCommitService_AttachmentCopyDuplicateNameAbortsAll(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "dup name")
	task := project.Tasks[0]
	seedTaskAttachment(t, project, task, "dup.txt", []byte("existing"))

	srcPath := filepath.Join(t.TempDir(), "dup.txt")
	if err := os.WriteFile(srcPath, []byte("incoming"), 0o600); err != nil {
		t.Fatalf("seed source: %v", err)
	}

	_, err := svc.Commit(ChangeSet{
		Project:       project,
		AttachmentOps: []AttachmentOp{{Kind: AttachmentCopy, Task: task, DestProject: project, SourcePath: srcPath}},
	})
	if err == nil {
		t.Fatal("expected duplicate-name copy to be rejected")
	}
	if commitProjectExists(project) {
		t.Fatal("project written despite rejected attachment copy")
	}
	// The pre-existing file is untouched.
	got, _ := os.ReadFile(GetAttachmentPath(project, task, "dup.txt"))
	if string(got) != "existing" {
		t.Fatalf("existing attachment changed to %q", got)
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentCopyMissingSourceAbortsAll confirms a copy whose
// source file is missing fails and prevents the project write.
func TestCommitService_AttachmentCopyMissingSourceAbortsAll(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "missing source")
	task := project.Tasks[0]

	_, err := svc.Commit(ChangeSet{
		Project:       project,
		AttachmentOps: []AttachmentOp{{Kind: AttachmentCopy, Task: task, DestProject: project, SourcePath: filepath.Join(t.TempDir(), "nope.txt")}},
	})
	if err == nil {
		t.Fatal("expected missing-source copy to be rejected")
	}
	if commitProjectExists(project) {
		t.Fatal("project written despite missing attachment source")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentMoveAcrossProjects moves a task's whole attachments
// directory — including a large file — from one project to another in the same
// transaction as both projects' tasks.md writes, confirming the destination takes
// ownership of the bytes and the source directory is gone.
func TestCommitService_AttachmentMoveAcrossProjects(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "moving")
	dst := newCommitTestProject(t, dataDir, "dst", "existing")
	task := src.Tasks[0]

	large := make([]byte, 5<<20) // 5 MiB
	for i := range large {
		large[i] = byte(i % 251)
	}
	seedTaskAttachment(t, src, task, "small.txt", []byte("small"))
	seedTaskAttachment(t, src, task, "large.bin", large)

	if _, err := svc.CommitMulti([]ChangeSet{
		{Project: src, AttachmentOps: []AttachmentOp{{Kind: AttachmentMove, Task: task, SourceProject: src, DestProject: dst}}},
		{Project: dst},
	}); err != nil {
		t.Fatalf("CommitMulti: %v", err)
	}

	if _, err := os.Stat(GetTaskAttachmentsDir(src, task)); !os.IsNotExist(err) {
		t.Fatalf("source attachments dir should be gone, stat err = %v", err)
	}
	if got, err := os.ReadFile(GetAttachmentPath(dst, task, "small.txt")); err != nil || string(got) != "small" {
		t.Fatalf("moved small file = %q err = %v", got, err)
	}
	movedLarge, err := os.ReadFile(GetAttachmentPath(dst, task, "large.bin"))
	if err != nil {
		t.Fatalf("read moved large file: %v", err)
	}
	if len(movedLarge) != len(large) || movedLarge[0] != large[0] || movedLarge[len(large)-1] != large[len(large)-1] {
		t.Fatalf("moved large file corrupted: len=%d want=%d", len(movedLarge), len(large))
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentMoveMissingSourceIsNoop confirms moving a task that
// has no attachments directory is a no-op that still commits the project writes.
func TestCommitService_AttachmentMoveMissingSourceIsNoop(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "no attachments")
	dst := newCommitTestProject(t, dataDir, "dst", "dst")
	task := src.Tasks[0]

	if _, err := svc.CommitMulti([]ChangeSet{
		{Project: src, AttachmentOps: []AttachmentOp{{Kind: AttachmentMove, Task: task, SourceProject: src, DestProject: dst}}},
		{Project: dst},
	}); err != nil {
		t.Fatalf("CommitMulti: %v", err)
	}
	if !commitProjectExists(src) || !commitProjectExists(dst) {
		t.Fatal("both projects should commit even with no attachments to move")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentMoveDestExistsAbortsAll confirms a move onto an
// already-occupied destination directory fails and prevents all writes.
func TestCommitService_AttachmentMoveDestExistsAbortsAll(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "moving")
	dst := newCommitTestProject(t, dataDir, "dst", "existing")
	task := src.Tasks[0]
	seedTaskAttachment(t, src, task, "a.txt", []byte("a"))
	seedTaskAttachment(t, dst, task, "collide.txt", []byte("already here"))

	_, err := svc.CommitMulti([]ChangeSet{
		{Project: src, AttachmentOps: []AttachmentOp{{Kind: AttachmentMove, Task: task, SourceProject: src, DestProject: dst}}},
		{Project: dst},
	})
	if err == nil {
		t.Fatal("expected move onto occupied destination to be rejected")
	}
	if commitProjectExists(src) || commitProjectExists(dst) {
		t.Fatal("projects written despite rejected attachment move")
	}
	// The source attachment is left in place for a retry.
	if !AttachmentExists(src, task, "a.txt") {
		t.Fatal("source attachment should be untouched after rejected move")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentDeleteRemovesDir confirms an attachment delete removes
// every file and the task's directory in the same transaction as the project write.
func TestCommitService_AttachmentDeleteRemovesDir(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "deleting")
	task := project.Tasks[0]
	seedTaskAttachment(t, project, task, "a.txt", []byte("a"))
	seedTaskAttachment(t, project, task, "b.txt", []byte("b"))

	if _, err := svc.Commit(ChangeSet{
		Project:       project,
		AttachmentOps: []AttachmentOp{{Kind: AttachmentDelete, Task: task, SourceProject: project}},
	}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := os.Stat(GetTaskAttachmentsDir(project, task)); !os.IsNotExist(err) {
		t.Fatalf("task attachments dir should be gone, stat err = %v", err)
	}
	if !commitProjectExists(project) {
		t.Fatal("project tasks.md not written")
	}
	assertJournalClean(t, dataDir)
}

// TestCommitService_AttachmentMoveRecoversWithMetadata proves the crash-safety
// guarantee: a transaction that renames a task's attachments directory together
// with both projects' tasks.md writes, interrupted after Prepare but before Commit,
// is rolled forward by recovery so the attachment bytes and the task metadata are
// restored together — never one without the other.
func TestCommitService_AttachmentMoveRecoversWithMetadata(t *testing.T) {
	_, dataDir := newCommitTestService(t)
	src := newCommitTestProject(t, dataDir, "src", "moving")
	dst := newCommitTestProject(t, dataDir, "dst", "existing")
	task := src.Tasks[0]

	seedTaskAttachment(t, src, task, "payload.bin", []byte("attachment-bytes"))
	srcDir := GetTaskAttachmentsDir(src, task)
	destDir := GetTaskAttachmentsDir(dst, task)
	if err := os.MkdirAll(filepath.Dir(destDir), DirPermissions); err != nil {
		t.Fatalf("mkdir dest parent: %v", err)
	}

	// Perform the metadata move in memory so the staged tasks.md reflects it.
	moved := src.Tasks[0]
	src.Tasks = nil
	src.BuildTaskTree()
	moved.ShortID = dst.GetNextShortID()
	dst.Tasks = append(dst.Tasks, moved)
	dst.NextShortID = moved.ShortID + 1
	dst.BuildTaskTree()

	srcContent := mustWriteV5Project(t, src)
	dstContent := mustWriteV5Project(t, dst)

	txnDir := filepath.Join(dataDir, ".tt", "transactions")
	journal := transaction.NewJournal(txnDir, []string{dataDir})
	if _, err := journal.Prepare([]transaction.Op{
		{Kind: transaction.OpWrite, Target: tasksFilePath(src.Path), Data: []byte(srcContent), Mode: FilePermissions},
		{Kind: transaction.OpWrite, Target: tasksFilePath(dst.Path), Data: []byte(dstContent), Mode: FilePermissions},
		{Kind: transaction.OpMove, Source: srcDir, Target: destDir},
	}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	// Simulate a crash: the manifest is prepared but never committed. A fresh
	// journal (as a later process would construct) recovers it.
	report, err := transaction.NewJournal(txnDir, []string{dataDir}).Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.RolledForward) != 1 {
		t.Fatalf("RolledForward = %v, want exactly one transaction", report.RolledForward)
	}

	// Attachment bytes followed the task to the destination.
	if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
		t.Fatalf("source attachments dir should be gone, stat err = %v", err)
	}
	if got, err := os.ReadFile(GetAttachmentPath(dst, task, "payload.bin")); err != nil || string(got) != "attachment-bytes" {
		t.Fatalf("recovered attachment = %q err = %v", got, err)
	}

	// Metadata landed together with the bytes: dst owns the task, src is empty.
	loadedDst, _ := models.NewProject(uuid.New().String(), "dst", "")
	loadedDst.Path = dst.Path
	if err := LoadProject(loadedDst); err != nil {
		t.Fatalf("LoadProject dst: %v", err)
	}
	if len(loadedDst.Tasks) != 2 {
		t.Fatalf("dst has %d tasks after recovery, want 2", len(loadedDst.Tasks))
	}
	loadedSrc, _ := models.NewProject(uuid.New().String(), "src", "")
	loadedSrc.Path = src.Path
	if err := LoadProject(loadedSrc); err != nil {
		t.Fatalf("LoadProject src: %v", err)
	}
	if len(loadedSrc.Tasks) != 0 {
		t.Fatalf("src has %d tasks after recovery, want 0", len(loadedSrc.Tasks))
	}
	assertJournalClean(t, dataDir)
}
