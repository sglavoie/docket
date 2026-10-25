package bridge

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/storage"
)

// simulatorEvidenceEnv names the evidence.json the iOS simulator suite wrote.
// scripts/verify_mobile_session.sh 3 sets it after that suite passes.
const simulatorEvidenceEnv = "DOCKET_BRIDGE_SIMULATOR_EVIDENCE"

// simulatorEvidence is what BridgeSimulatorTests.testOpenFixtureCaptureCompleteReopen
// records about the workspace it changed.
type simulatorEvidence struct {
	BridgeVersion       int    `json:"bridge_version"`
	DataDir             string `json:"data_dir"`
	Workspace           string `json:"workspace"`
	DeviceID            string `json:"device_id"`
	ProjectID           string `json:"project_id"`
	ProjectSlug         string `json:"project_slug"`
	TaskID              string `json:"task_id"`
	Title               string `json:"title"`
	CaptureTransaction  string `json:"capture_transaction"`
	CompleteTransaction string `json:"complete_transaction"`
	ReopenedState       string `json:"reopened_state"`
	ReopenedRevision    string `json:"reopened_revision"`
}

// TestBridgeSimulatorEvidence checks, through Go, the workspace the iOS
// simulator suite changed: its tasks.md must be canonical schema-19 Markdown
// holding the captured task completed, every fixture task must survive
// untouched, every other project file must be byte-identical to the fixture,
// and a fresh Go-side open must read back what the simulator reported. It only
// runs where the verifier hands it that evidence.
func TestBridgeSimulatorEvidence(t *testing.T) {
	path := os.Getenv(simulatorEvidenceEnv)
	if path == "" {
		t.Skipf("%s is unset; scripts/verify_mobile_session.sh 3 runs this after the simulator suite", simulatorEvidenceEnv)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var evidence simulatorEvidence
	if err = decodeStrict(data, &evidence); err != nil {
		t.Fatalf("evidence.json: %v", err)
	}
	switch {
	case evidence.BridgeVersion != Version:
		t.Fatalf("the simulator spoke bridge version %d, this engine %d", evidence.BridgeVersion, Version)
	case evidence.ReopenedState != "done":
		t.Fatalf("the simulator reopened the task in %q", evidence.ReopenedState)
	case evidence.CaptureTransaction == "" || evidence.CompleteTransaction == "" ||
		evidence.CaptureTransaction == evidence.CompleteTransaction:
		t.Fatalf("the simulator reported transactions %q and %q", evidence.CaptureTransaction, evidence.CompleteTransaction)
	}

	requireCompletedMarkdown(t, evidence.DataDir, evidence.ProjectSlug, evidence.TaskID, evidence.Title)

	touched := filepath.Join("projects", evidence.ProjectSlug, "tasks.md")
	t.Run("fixture_tasks_survive", func(t *testing.T) {
		before := parseDocument(t, filepath.Join(representativeFixture, touched))
		after := parseDocument(t, filepath.Join(evidence.DataDir, touched))
		if len(after.Tasks) != len(before.Tasks)+1 {
			t.Fatalf("tasks.md holds %d tasks, the fixture %d plus one capture", len(after.Tasks), len(before.Tasks))
		}
		for i, task := range before.Tasks {
			kept := after.Tasks[i]
			if kept.ID != task.ID || kept.Title != task.Title || kept.WorkflowState != task.WorkflowState || !kept.UpdatedAt.Equal(task.UpdatedAt) {
				t.Fatalf("fixture task %s changed: %+v", task.ID, kept)
			}
		}
	})

	t.Run("other_project_files_untouched", func(t *testing.T) {
		compared := 0
		root := filepath.Join(representativeFixture, "projects")
		err := filepath.WalkDir(root, func(source string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return walkErr
			}
			relative, relErr := filepath.Rel(representativeFixture, source)
			if relErr != nil || relative == touched {
				return relErr
			}
			want, readErr := os.ReadFile(source)
			if readErr != nil {
				return readErr
			}
			got, readErr := os.ReadFile(filepath.Join(evidence.DataDir, relative))
			if readErr != nil {
				return readErr
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s changed", relative)
			}
			compared++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if compared == 0 {
			t.Fatal("no fixture project files were compared")
		}
	})

	t.Run("go_reopen_agrees", func(t *testing.T) {
		h := newHarness(t, "")
		h.dataDir = evidence.DataDir
		status := h.open(false)
		if status.Workspace != evidence.Workspace {
			t.Fatalf("Go opened workspace %s, the simulator %s", status.Workspace, evidence.Workspace)
		}
		var read taskResult
		h.ok(h.call("query.task", map[string]any{"task": locator{ProjectID: evidence.ProjectID, TaskID: evidence.TaskID}}), &read)
		if read.Task.State != "done" || read.Task.Title != evidence.Title || read.Revision != evidence.ReopenedRevision {
			t.Fatalf("Go reads %+v at %s; the simulator reported done at %s", read.Task, read.Revision, evidence.ReopenedRevision)
		}
	})
}

func parseDocument(t *testing.T, path string) *storage.ProjectDocument {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseProjectDocument(string(data))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return document
}

// TestSimulatorEvidenceShapeMatchesTheSwiftRecord keeps the Go decoder and
// the Swift writer on one field set, so a renamed key fails here rather than
// as a strict-decode refusal at verification time.
func TestSimulatorEvidenceShapeMatchesTheSwiftRecord(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "ios", "TTEngine", "Tests", "TTEngineTests", "BridgeSimulatorTests.swift"))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	encoded, err := json.Marshal(simulatorEvidence{})
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for field := range fields {
		if !bytes.Contains(source, []byte(`"`+field+`":`)) {
			t.Errorf("the Swift evidence record does not write %q", field)
		}
	}
}
