package tui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// crossMachineRaceResult is what a finished race leaves a caller to judge: the
// document every machine rewrote, the workspace it lives in — a conflict bundle
// is durable state beside it, not something a helper's word can prove — and each
// machine's reported outcome in the order the roles were given.
type crossMachineRaceResult struct {
	merged   *models.Project
	dataDir  string
	outcomes []string
}

// runCrossMachineRace forks the named machines over one freshly seeded
// workspace, proves they are all held at the same untouched document, releases
// them together, and judges what each one reported against the merged file. It
// returns that merged project so a caller can make a claim about the whole
// document rather than about one machine's own write.
func runCrossMachineRace(t *testing.T, roles []crossMachineRole) crossMachineRaceResult {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("locate this test binary: %v", err)
	}

	workspace, project := crossMachineWorkspace(t)

	// The revision the document stands at before any process starts. Every helper
	// must report it from inside the barrier: a helper reporting anything else
	// read a document another one had already rewritten.
	baseRevision := crossMachineProjectRevision(t, project)

	barrierDir := t.TempDir()
	proceed := filepath.Join(barrierDir, "go")
	helpers := make([]*crossMachineHelper, 0, len(roles))
	for _, spec := range roles {
		helpers = append(helpers, startCrossMachineHelper(t, binary, workspace.DataDir, barrierDir, proceed, spec))
	}

	for _, helper := range helpers {
		helper.awaitBarrier(t)
	}
	for _, helper := range helpers {
		if helper.revision != baseRevision {
			t.Fatalf("machine %s reached the mutation boundary at revision %q, want the pre-race %q",
				helper.name, helper.revision, baseRevision)
		}
		// A result line is recorded only after the mutation returns, so an outcome
		// that exists while every process is held would mean one of them had
		// already finished the mutation the others are only now beginning.
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

	// Loaded after every process exited, so a confirmed commit is one that
	// survived the other machines finishing rather than one that merely was on
	// disk when its own process looked.
	merged := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err = storage.LoadProject(merged); err != nil {
		t.Fatalf("reload %s after the race: %v", project.Name, err)
	}
	committed := 0
	outcomes := make([]string, 0, len(helpers))
	for _, helper := range helpers {
		outcome := helper.outcome(t)
		outcomes = append(outcomes, outcome)
		switch {
		case strings.HasPrefix(outcome, "lost"):
			t.Fatalf("machine %s reported a commit that was silently replaced: %s", helper.name, outcome)
		case outcome == "committed", strings.HasPrefix(outcome, "retried:"):
			// A retry that landed is a commit whose first attempt was refused whole,
			// so the same claim about the merged document holds for it.
			committed++
			helper.confirm(t, merged)
		case outcome == "expired", strings.HasPrefix(outcome, "conflicted:"):
			// Admissible: a machine may lose the race outright, and the loser's edit
			// is then refused or durably recorded rather than dropped.
		default:
			t.Fatalf("machine %s reported %q", helper.name, outcome)
		}
	}
	if committed == 0 {
		t.Fatal("no machine committed its mutation")
	}
	if after := crossMachineProjectRevision(t, project); after == baseRevision {
		t.Fatalf("the document still stands at %q after a reported commit", after)
	}
	return crossMachineRaceResult{merged: merged, dataDir: workspace.DataDir, outcomes: outcomes}
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

// crossMachineHelper is one forked machine: the process, the barrier files it
// speaks through, what it published, and the claim its commit makes about the
// document both machines rewrote.
type crossMachineHelper struct {
	name     string
	command  *exec.Cmd
	output   *bytes.Buffer
	ready    string
	result   string
	exit     chan error
	revision string
	// confirm reads the merged project and fails when this machine's reported
	// commit is not in it. Each mutation makes a different claim — a note, a
	// comment, a cleared flag, a task that did not exist — so the claim travels
	// with the machine that made it rather than being re-derived by the race.
	confirm func(t *testing.T, project *models.Project)
}

// crossMachineRole names one forked machine before it exists: which helper role
// to re-enter, what extra environment that role reads, and what its commit will
// claim. One struct covers every race because the roles differ only in those
// three things.
type crossMachineRole struct {
	name    string
	role    string
	env     []string
	confirm func(t *testing.T, project *models.Project)
}

// startCrossMachineHelper forks one machine and returns before it has reached
// the barrier. Both machines are started before either is awaited, since a
// helper that had to be awaited before its partner existed could not overlap
// with it.
func startCrossMachineHelper(t *testing.T, binary, dataDir, barrierDir, proceed string, spec crossMachineRole) *crossMachineHelper {
	t.Helper()
	helper := &crossMachineHelper{
		name:    spec.name,
		output:  &bytes.Buffer{},
		ready:   filepath.Join(barrierDir, "ready-"+spec.name),
		result:  filepath.Join(barrierDir, "result-"+spec.name),
		exit:    make(chan error, 1),
		confirm: spec.confirm,
	}
	helper.command = exec.Command(binary, "-test.run", "^"+spec.role+"$", "-test.count=1")
	helper.command.Env = append(append(os.Environ(),
		helperDataDirEnv+"="+dataDir,
		helperReadyEnv+"="+helper.ready,
		helperGoEnv+"="+proceed,
		helperResultEnv+"="+helper.result,
	), spec.env...)
	helper.command.Stdout = helper.output
	helper.command.Stderr = helper.output
	if err := helper.command.Start(); err != nil {
		t.Fatalf("fork machine %s: %v", spec.name, err)
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
func (h *crossMachineHelper) awaitBarrier(t *testing.T) {
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

func (h *crossMachineHelper) awaitExit(t *testing.T) {
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

func (h *crossMachineHelper) outcome(t *testing.T) string {
	t.Helper()
	recorded, err := os.ReadFile(h.result)
	if err != nil {
		t.Fatalf("machine %s recorded no outcome: %v\n%s", h.name, err, h.output)
	}
	return strings.TrimSpace(string(recorded))
}
