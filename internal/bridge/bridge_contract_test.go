package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/storage"
)

// TestBridgeRefusesMalformedAndMisaddressedRequests holds the envelope rules:
// strict decoding, version gating, the op table, and workspace addressing all
// answer with the contract's codes, never an empty string or a panic.
func TestBridgeRefusesMalformedAndMisaddressedRequests(t *testing.T) {
	h := newHarness(t, representativeFixture)

	t.Run("malformed_json", func(t *testing.T) {
		h.rejects(h.raw(`{"bridge_version":1,`), CodeInvalidRequest)
	})
	t.Run("unknown_envelope_field", func(t *testing.T) {
		reply := h.raw(`{"bridge_version":1,"request_id":"r1","op":"workspace.status","surprise":true}`)
		h.rejects(reply, CodeInvalidRequest)
		if reply.RequestID != "r1" {
			t.Fatalf("a refused request still echoes its id, got %q", reply.RequestID)
		}
	})
	t.Run("missing_version", func(t *testing.T) {
		h.rejects(h.raw(`{"request_id":"r","op":"workspace.status"}`), CodeInvalidRequest)
	})
	t.Run("newer_version", func(t *testing.T) {
		h.rejects(h.call("workspace.status", nil, func(e map[string]any) { e["bridge_version"] = Version + 1 }), CodeUnsupportedVersion)
	})
	t.Run("missing_request_id", func(t *testing.T) {
		h.rejects(h.raw(`{"bridge_version":1,"op":"workspace.status"}`), CodeInvalidRequest)
	})
	t.Run("unknown_op", func(t *testing.T) {
		refusal := h.refused(h.call("task.explode", nil), CodeUnknownOp)
		if refusal.Details["reserved"] != nil {
			t.Fatalf("an op outside the contract is not reserved: %+v", refusal)
		}
	})
	t.Run("reserved_op", func(t *testing.T) {
		refusal := h.refused(h.call("command.redo", nil), CodeUnknownOp)
		if refusal.Details["reserved"] != true {
			t.Fatalf("a contract op this build lacks says so: %+v", refusal)
		}
	})
	t.Run("not_open", func(t *testing.T) {
		h.rejects(h.call("workspace.status", nil, func(e map[string]any) { e["workspace"] = "anything" }), CodeNotOpen)
	})
	t.Run("open_requires_identity", func(t *testing.T) {
		h.rejects(h.call("workspace.open", map[string]any{"data_dir": h.dataDir}), CodeInvalidRequest)
		h.rejects(h.call("workspace.open", map[string]any{
			"data_dir": h.dataDir, "runtime_dir": h.runtimeDir, "device_id": "not a token",
		}), CodeInvalidRequest)
		h.rejects(h.call("workspace.open", map[string]any{
			"data_dir": "relative/workspace", "runtime_dir": h.runtimeDir, "device_id": "d1",
		}), CodeInvalidRequest)
	})
	t.Run("open_missing_workspace", func(t *testing.T) {
		h.rejects(h.call("workspace.open", map[string]any{
			"data_dir": filepath.Join(t.TempDir(), "absent"), "runtime_dir": h.runtimeDir, "device_id": "d1",
		}), CodeNotFound)
	})

	h.open(false)
	t.Run("workspace_mismatch", func(t *testing.T) {
		h.rejects(h.call("workspace.status", nil, func(e map[string]any) { e["workspace"] = "someone-else" }), CodeWorkspaceMismatch)
	})
	t.Run("workspace_required", func(t *testing.T) {
		h.rejects(h.call("workspace.status", nil, func(e map[string]any) { delete(e, "workspace") }), CodeInvalidRequest)
	})
	t.Run("unknown_payload_field", func(t *testing.T) {
		h.rejects(h.call("query.task", map[string]any{"task": map[string]any{"project_id": "p", "task_id": "t", "short_id": 1}}), CodeInvalidRequest)
	})
	t.Run("expect_on_query", func(t *testing.T) {
		h.rejects(h.call("query.projects", nil, withExpect(Expect{Domains: map[string]string{"planning": "x"}})), CodeInvalidRequest)
	})
	t.Run("bare_task_id", func(t *testing.T) {
		h.rejects(h.call("query.task", map[string]any{"task": map[string]any{"task_id": "0a7e5d10-2c3b-4f6a-8e9d-000000000501"}}), CodeInvalidRequest)
	})
	t.Run("unresolved_task", func(t *testing.T) {
		unfiled := h.unfiled()
		h.rejects(h.call("query.task", map[string]any{"task": locator{ProjectID: unfiled.ID, TaskID: "00000000-0000-4000-8000-999999999999"}}), CodeNotFound)
	})
	t.Run("invalid_state", func(t *testing.T) {
		unfiled := h.unfiled()
		h.rejects(h.call("command.transition", map[string]any{
			"task": locator{ProjectID: unfiled.ID, TaskID: "0a7e5d10-2c3b-4f6a-8e9d-000000000501"}, "state": "ready",
		}), CodeValidation)
	})
	t.Run("invalid_query", func(t *testing.T) {
		h.rejects(h.call("query.tasks", map[string]any{"query": "!9"}), CodeValidation)
	})
	t.Run("closed", func(t *testing.T) {
		h.ok(h.call("workspace.close", nil), nil)
		h.rejects(h.call("workspace.status", nil), CodeNotOpen)
	})
}

// TestBridgeCapturesCompletesAndReopens drives the whole session-3 scenario
// through the protocol: open the representative fixture, capture a task,
// complete it, close, reopen, and read both the bridge's answer and the
// Markdown the engine wrote.
func TestBridgeCapturesCompletesAndReopens(t *testing.T) {
	h := newHarness(t, representativeFixture)
	status := h.open(false)
	if status.Schemas.Catalog != 21 || status.Schemas.TaskMarkdown != storage.CurrentTaskSchemaVersion || status.Schemas.Bridge != Version {
		t.Fatalf("status reports schemas %+v", status.Schemas)
	}
	unfiled := h.unfiled()
	if status.Revisions.Projects[unfiled.ID] == "" || status.Revisions.Domains["catalog"] == "" {
		t.Fatalf("status names no revisions to precondition on: %+v", status.Revisions)
	}

	h.clock.advance(time.Minute)
	var captured mutationResult
	h.ok(h.call("command.capture", map[string]any{"title": "Ship the bridge #notatag", "priority": 2}), &captured)
	if captured.Task.Title != "Ship the bridge #notatag" || captured.Task.State != "inbox" || captured.Task.ProjectID != unfiled.ID {
		t.Fatalf("captured %+v", captured.Task)
	}
	if captured.Task.TaskID != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("capture did not mint through the injected generator: %s", captured.Task.TaskID)
	}
	if captured.Committed.TransactionID == "" || captured.Committed.Projects[unfiled.ID] == "" {
		t.Fatalf("capture reports no committed effect: %+v", captured.Committed)
	}
	target := locator{ProjectID: captured.Task.ProjectID, TaskID: captured.Task.TaskID}

	t.Run("stale_expectation_commits_nothing", func(t *testing.T) {
		refusal := h.refused(h.call("command.transition", map[string]any{"task": target, "state": "done"},
			withExpect(Expect{Projects: map[string]string{unfiled.ID: status.Revisions.Projects[unfiled.ID]}})), CodeStaleRevision)
		if refusal.Details["current_revision"] != captured.Committed.Projects[unfiled.ID] {
			t.Fatalf("stale refusal names the current revision: %+v", refusal)
		}
		var read taskResult
		h.ok(h.call("query.task", map[string]any{"task": target}), &read)
		if read.Task.State != "inbox" {
			t.Fatalf("a refused transition changed the task: %+v", read.Task)
		}
	})

	h.clock.advance(time.Minute)
	completeRequest := map[string]any{"task": target, "state": "done"}
	expect := withExpect(Expect{Projects: map[string]string{unfiled.ID: captured.Committed.Projects[unfiled.ID]}})
	var completed mutationResult
	h.ok(h.call("command.transition", completeRequest, withRequestID("complete-1"), expect), &completed)
	if completed.Task.State != "done" || completed.Task.CompletedAt == nil {
		t.Fatalf("completed %+v", completed.Task)
	}

	t.Run("replay_returns_the_first_result", func(t *testing.T) {
		var replayed mutationResult
		h.ok(h.call("command.transition", completeRequest, withRequestID("complete-1"), expect), &replayed)
		if replayed.Committed.TransactionID != completed.Committed.TransactionID {
			t.Fatalf("replay committed again: %s then %s", completed.Committed.TransactionID, replayed.Committed.TransactionID)
		}
		h.rejects(h.call("command.transition", map[string]any{"task": target, "state": "canceled"}, withRequestID("complete-1")), CodeInvalidRequest)
	})

	h.ok(h.call("workspace.close", nil), nil)
	reopened := h.open(false)
	if reopened.Workspace != status.Workspace {
		t.Fatalf("reopen changed the workspace identity: %s then %s", status.Workspace, reopened.Workspace)
	}
	var read taskResult
	h.ok(h.call("query.task", map[string]any{"task": target}), &read)
	if read.Task.State != "done" || read.Task.Title != "Ship the bridge #notatag" || read.Revision != completed.Committed.Projects[unfiled.ID] {
		t.Fatalf("reopened task %+v at %s", read.Task, read.Revision)
	}

	t.Run("queries", func(t *testing.T) {
		var inbox tasksResult
		h.ok(h.call("query.tasks", map[string]any{"mode": "inbox", "scope": unfiled.ID}), &inbox)
		for _, item := range inbox.Items {
			if item.Task.TaskID == target.TaskID {
				t.Fatalf("a completed task stayed in the inbox: %+v", item)
			}
		}
		var completedOnly tasksResult
		h.ok(h.call("query.tasks", map[string]any{"mode": "all", "query": "?completed"}), &completedOnly)
		found := false
		for _, item := range completedOnly.Items {
			found = found || item.Task.TaskID == target.TaskID
		}
		if !found || completedOnly.Query != "?completed" {
			t.Fatalf("?completed did not answer the completed task: %+v", completedOnly)
		}
	})

	requireCompletedMarkdown(t, h.dataDir, "unfiled", target.TaskID, "Ship the bridge #notatag")
}

// requireCompletedMarkdown reads the project's tasks.md through the live
// parser and holds it to what the bridge reported: canonical bytes at the
// current schema, holding the task completed under its literal title.
func requireCompletedMarkdown(t *testing.T, dataDir, slug, taskID, title string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dataDir, "projects", slug, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := storage.ParseProjectDocument(string(data))
	if err != nil {
		t.Fatalf("tasks.md does not parse: %v", err)
	}
	rendered, err := document.WriteCurrent()
	if err != nil || rendered != string(data) {
		t.Fatalf("tasks.md is not the canonical rendering (err %v):\n%s", err, data)
	}
	if !strings.Contains(string(data), fmt.Sprintf("<!-- tt-schema: %d -->", storage.CurrentTaskSchemaVersion)) {
		t.Fatalf("tasks.md lacks the current schema marker:\n%s", data)
	}
	for _, task := range document.Tasks {
		if task.ID != taskID {
			continue
		}
		if task.Title != title || string(task.WorkflowState) != "done" || task.CompletedAt == nil {
			t.Fatalf("tasks.md holds %q in %s (completed %v)", task.Title, task.WorkflowState, task.CompletedAt)
		}
		return
	}
	t.Fatalf("tasks.md does not hold task %s:\n%s", taskID, data)
}

// TestBridgeOpenRefusesToMigrate holds workspace.open to the contract: a
// workspace older than the engine is refused before anything is written, and
// one newer is refused as too new.
func TestBridgeOpenRefusesToMigrate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		schema int
		code   string
	}{
		{"older", 20, CodeMigrationRequired},
		{"newer", 99, CodeSchemaTooNew},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, representativeFixture)
			catalogPath := filepath.Join(h.dataDir, ".tt", "workspace.json")
			rewriteCatalogSchema(t, catalogPath, tc.schema)
			before, err := os.ReadFile(catalogPath)
			if err != nil {
				t.Fatal(err)
			}
			h.rejects(h.call("workspace.open", map[string]any{
				"data_dir": h.dataDir, "runtime_dir": h.runtimeDir, "device_id": "d1",
			}), tc.code)
			after, err := os.ReadFile(catalogPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("a refused open rewrote the catalog")
			}
			if _, statErr := os.Stat(h.runtimeDir); !errors.Is(statErr, fs.ErrNotExist) {
				t.Fatalf("a refused open created machine-local state (stat err %v)", statErr)
			}
		})
	}
}

func rewriteCatalogSchema(t *testing.T, path string, schema int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err = json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	envelope["data"].(map[string]any)["schema_version"] = schema
	if data, err = json.MarshalIndent(envelope, "", "  "); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestBridgeCreatesAFreshWorkspace opens an empty directory with
// create_if_missing, which is how the app starts before any sync.
func TestBridgeCreatesAFreshWorkspace(t *testing.T) {
	h := newHarness(t, "")
	status := h.open(true)
	if status.Workspace == "" || status.Schemas.Catalog != 21 {
		t.Fatalf("fresh status %+v", status)
	}
	unfiled := h.unfiled()
	var captured mutationResult
	h.ok(h.call("command.capture", map[string]any{"project_id": unfiled.ID, "title": "First", "destination": "today"}), &captured)
	if captured.Committed.Domains["planning"] == "" {
		t.Fatalf("a Today capture reports the planning revision it moved: %+v", captured.Committed)
	}
	var today tasksResult
	h.ok(h.call("query.tasks", map[string]any{"mode": "today"}), &today)
	if len(today.Items) != 1 || !today.Items[0].Today || today.Items[0].Task.TaskID != captured.Task.TaskID {
		t.Fatalf("today %+v", today.Items)
	}
}
