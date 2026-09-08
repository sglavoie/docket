package tui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

const (
	// resumeBarrierNoteA and resumeBarrierNoteB are the two notes the race
	// commits. They have to be distinguishable in one tasks.md, because "machine
	// A's note survived" is checked by reading the file both machines rewrote.
	resumeBarrierNoteA = "picking the migration plan back up"
	resumeBarrierNoteB = "waiting on legal to answer the vendor"
)

// TestCrossMachineResumeBarrierFreezesMutationSnapshots is the in-suite proof
// that the cross-machine resume-note case is the lost-update race it claims to
// be rather than two sequential writes.
//
// The distinction is the whole point. SetResumeNote loads its own snapshot, so a
// barrier placed around the call releases two processes that have resolved a
// task and nothing more: one of them may then read, commit and exit before the
// other has read the document it is about to rewrite, and every assertion the
// gate makes still passes. Holding both processes at
// app.ResumeNoteCommitBarrier — after each snapshot, before either commit —
// makes the overlap observable, and the base revision each side publishes from
// inside that seam is the evidence: two equal revisions, both equal to the
// document as it stood before either process started, mean neither had written
// when the other read.
//
// It forks the real thing. The two mutating parties are this package's own test
// binary re-entered as TestCrossMachineHelperSetsResumeNote, which is the role
// scripts/cross_machine_gate.sh forks, so the coordination proven here is the
// coordination the gate depends on. internal/tui's TestMain gives each process
// its own $HOME, which is exactly what makes them two machines.
func TestCrossMachineResumeBarrierFreezesMutationSnapshots(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("locate this test binary: %v", err)
	}

	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("newUITestWorkspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	project, err := seedCrossMachineWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	openCrossMachineWorkspaceOnce(t, workspace.DataDir)

	// The revision the document stands at before either process starts. Both
	// helpers must report it from inside the barrier: a helper reporting anything
	// else read a document the other one had already rewritten.
	baseRevision := crossMachineProjectRevision(t, project)

	barrierDir := t.TempDir()
	proceed := filepath.Join(barrierDir, "go")
	helpers := []*resumeNoteHelper{
		startResumeNoteHelper(t, binary, workspace.DataDir, barrierDir, proceed, "A", helperRaceTitleA, resumeBarrierNoteA),
		startResumeNoteHelper(t, binary, workspace.DataDir, barrierDir, proceed, "B", helperRaceTitleB, resumeBarrierNoteB),
	}

	for _, helper := range helpers {
		helper.awaitBarrier(t)
	}
	for _, helper := range helpers {
		if helper.revision != baseRevision {
			t.Fatalf("machine %s reached the mutation boundary at revision %q, want the pre-race %q",
				helper.name, helper.revision, baseRevision)
		}
		// A result line is recorded only after SetResumeNote returns, so an
		// outcome that exists while both processes are held would mean one of them
		// had already finished the mutation the other is only now beginning.
		if _, statErr := os.Stat(helper.result); statErr == nil {
			t.Fatalf("machine %s recorded an outcome before the barrier opened", helper.name)
		}
	}
	// The other half of "frozen": the snapshots are taken and the document is
	// still untouched. Equal published revisions alone would also be produced by
	// a barrier that fired after both commits, since each side reports the
	// revision it captured rather than the one on disk. Reading the file while
	// both machines are held is what separates the two.
	if held := crossMachineProjectRevision(t, project); held != baseRevision {
		t.Fatalf("the document moved to %q while both machines were held; the barrier does not precede the commit", held)
	}

	if err = os.WriteFile(proceed, []byte("go\n"), storage.FilePermissions); err != nil {
		t.Fatalf("release the barrier: %v", err)
	}
	for _, helper := range helpers {
		helper.awaitExit(t)
	}

	document := storage.TasksFilePath(project.Path)
	raw, err := os.ReadFile(document)
	if err != nil {
		t.Fatalf("read %s: %v", document, err)
	}
	committed := 0
	for _, helper := range helpers {
		outcome := helper.outcome(t)
		switch {
		case strings.HasPrefix(outcome, "lost"):
			t.Fatalf("machine %s reported a commit that was silently replaced: %s", helper.name, outcome)
		case outcome == "committed":
			committed++
			// Read after both processes exited, so this is the claim that a
			// reported commit survived the other machine finishing rather than
			// merely that it was on disk when its own process looked.
			if !strings.Contains(string(raw), helper.note) {
				t.Fatalf("machine %s committed %q, which is not in %s:\n%s", helper.name, helper.note, document, raw)
			}
		case outcome == "expired", strings.HasPrefix(outcome, "conflicted:"):
			// Admissible: one machine may lose the race outright, and the loser's
			// edit is then refused or durably recorded rather than dropped.
		default:
			t.Fatalf("machine %s reported %q", helper.name, outcome)
		}
	}
	if committed == 0 {
		t.Fatal("neither machine committed a resume note")
	}
	if after := crossMachineProjectRevision(t, project); after == baseRevision {
		t.Fatalf("the document still stands at %q after a reported commit", after)
	}
}

// openCrossMachineWorkspaceOnce opens the seeded workspace the way a helper role
// does, before any helper runs. A first open may bootstrap system state, and
// letting two forked processes discover that at once would race over something
// other than the resume notes under test.
func openCrossMachineWorkspaceOnce(t *testing.T, dataDir string) {
	t.Helper()
	manager := config.NewManager(config.ConfigPathForDataDir(dataDir))
	if _, err := manager.LoadDomains(); err != nil {
		t.Fatalf("LoadDomains(%q): %v", dataDir, err)
	}
	if _, err := manager.LoadRuntimeState(); err != nil {
		t.Fatalf("LoadRuntimeState(%q): %v", dataDir, err)
	}
}

// crossMachineProjectRevision reads the project's canonical revision straight
// from disk through the same call the application snapshot uses, so the value
// compared against a helper's published revision is the same kind of value.
func crossMachineProjectRevision(t *testing.T, project *models.Project) string {
	t.Helper()
	document, err := storage.LoadProjectDocument(&models.Project{
		ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path,
	})
	if err != nil {
		t.Fatalf("LoadProjectDocument(%q): %v", project.Name, err)
	}
	return document.Revision
}

// resumeNoteHelper is one forked machine: the process, the barrier files it
// speaks through, and what it published.
type resumeNoteHelper struct {
	name     string
	note     string
	command  *exec.Cmd
	output   *bytes.Buffer
	ready    string
	result   string
	exit     chan error
	revision string
}

// startResumeNoteHelper forks one machine and returns before it has reached the
// barrier. Both machines are started before either is awaited, since a helper
// that had to be awaited before its partner existed could not overlap with it.
func startResumeNoteHelper(t *testing.T, binary, dataDir, barrierDir, proceed, name, title, note string) *resumeNoteHelper {
	t.Helper()
	helper := &resumeNoteHelper{
		name:   name,
		note:   note,
		output: &bytes.Buffer{},
		ready:  filepath.Join(barrierDir, "ready-"+name),
		result: filepath.Join(barrierDir, "result-"+name),
		exit:   make(chan error, 1),
	}
	helper.command = exec.Command(binary, "-test.run", "^TestCrossMachineHelperSetsResumeNote$", "-test.count=1")
	helper.command.Env = append(os.Environ(),
		helperDataDirEnv+"="+dataDir,
		helperReadyEnv+"="+helper.ready,
		helperGoEnv+"="+proceed,
		helperResultEnv+"="+helper.result,
		helperTitleEnv+"="+title,
		helperNoteEnv+"="+note,
	)
	helper.command.Stdout = helper.output
	helper.command.Stderr = helper.output
	if err := helper.command.Start(); err != nil {
		t.Fatalf("fork machine %s: %v", name, err)
	}
	t.Cleanup(func() {
		if helper.command.Process != nil {
			_ = helper.command.Process.Kill()
		}
	})
	go func() { helper.exit <- helper.command.Wait() }()
	return helper
}

// awaitBarrier blocks until this machine publishes the revision it is holding.
// A process that exited instead of publishing is a named failure rather than a
// wait that runs to its bound, because the outcome it never reached is the whole
// evidence this test is collecting.
func (h *resumeNoteHelper) awaitBarrier(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(helperBarrierTimeout)
	for time.Now().Before(deadline) {
		select {
		case err := <-h.exit:
			t.Fatalf("machine %s exited (%v) before reaching the mutation barrier:\n%s", h.name, err, h.output)
		default:
		}
		published, err := os.ReadFile(h.ready)
		if err == nil {
			h.revision = strings.TrimSpace(string(published))
			if h.revision == "" {
				t.Fatalf("machine %s published an empty base revision", h.name)
			}
			return
		}
		time.Sleep(helperPollInterval)
	}
	t.Fatalf("machine %s did not reach the mutation barrier within %s:\n%s", h.name, helperBarrierTimeout, h.output)
}

func (h *resumeNoteHelper) awaitExit(t *testing.T) {
	t.Helper()
	select {
	case err := <-h.exit:
		if err != nil {
			t.Fatalf("machine %s failed: %v\n%s", h.name, err, h.output)
		}
	case <-time.After(helperBarrierTimeout):
		t.Fatalf("machine %s never finished its mutation:\n%s", h.name, h.output)
	}
}

func (h *resumeNoteHelper) outcome(t *testing.T) string {
	t.Helper()
	recorded, err := os.ReadFile(h.result)
	if err != nil {
		t.Fatalf("machine %s recorded no outcome: %v\n%s", h.name, err, h.output)
	}
	return strings.TrimSpace(string(recorded))
}
