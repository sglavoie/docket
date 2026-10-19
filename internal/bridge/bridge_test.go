package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// representativeFixture is session 1's committed compatibility workspace.
var representativeFixture = filepath.Join("..", "compat", "testdata", "workspaces", "representative")

type fixedClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type sequentialIDs struct {
	mu   sync.Mutex
	next int
}

func (g *sequentialIDs) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", g.next)
}

// harness is one bridge over a private copy of a workspace, with a process
// $HOME the bridge must never create.
type harness struct {
	t          *testing.T
	bridge     *Bridge
	clock      *fixedClock
	dataDir    string
	runtimeDir string
	workspace  string
	requests   int
}

func newHarness(t *testing.T, fixture string) *harness {
	t.Helper()
	root := t.TempDir()
	processHome := filepath.Join(root, "process-home")
	t.Setenv("HOME", processHome)
	t.Cleanup(func() {
		if _, err := os.Stat(processHome); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("the bridge wrote beneath the process $HOME (stat err %v)", err)
		}
	})
	dataDir := filepath.Join(root, "workspace")
	if fixture != "" {
		copyTree(t, fixture, dataDir)
	}
	clock := &fixedClock{now: time.Date(2026, time.October, 6, 9, 0, 0, 0, time.UTC)}
	return &harness{
		t:          t,
		bridge:     New(Options{Clock: clock, IDs: &sequentialIDs{}}),
		clock:      clock,
		dataDir:    dataDir,
		runtimeDir: filepath.Join(root, "device"),
	}
}

func copyTree(t *testing.T, source, target string) {
	t.Helper()
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// response is a decoded reply with its result left raw for the caller.
type response struct {
	BridgeVersion int             `json:"bridge_version"`
	RequestID     string          `json:"request_id"`
	OK            bool            `json:"ok"`
	Result        json.RawMessage `json:"result"`
	Error         *Error          `json:"error"`
}

func (h *harness) raw(request string) response {
	h.t.Helper()
	reply := h.bridge.Call(request)
	if reply == "" {
		h.t.Fatalf("Call(%s) returned an empty string", request)
	}
	var decoded response
	if err := json.Unmarshal([]byte(reply), &decoded); err != nil {
		h.t.Fatalf("response is not JSON: %v\n%s", err, reply)
	}
	if decoded.OK == (decoded.Error != nil) || decoded.OK != (len(decoded.Result) > 0) {
		h.t.Fatalf("response must carry exactly one of result and error: %s", reply)
	}
	return decoded
}

// call sends one op with a fresh request id, or the given one.
func (h *harness) call(op string, payload any, opts ...func(map[string]any)) response {
	h.t.Helper()
	h.requests++
	envelope := map[string]any{
		"bridge_version": Version,
		"request_id":     fmt.Sprintf("req-%d", h.requests),
		"op":             op,
	}
	if op != "workspace.open" && op != "workspace.join" && h.workspace != "" {
		envelope["workspace"] = h.workspace
	}
	if payload != nil {
		envelope["payload"] = payload
	}
	for _, opt := range opts {
		opt(envelope)
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		h.t.Fatal(err)
	}
	reply := h.raw(string(data))
	if reply.RequestID != envelope["request_id"] {
		h.t.Fatalf("response echoed request_id %q, want %q", reply.RequestID, envelope["request_id"])
	}
	return reply
}

func withRequestID(id string) func(map[string]any) {
	return func(envelope map[string]any) { envelope["request_id"] = id }
}

func withExpect(expect Expect) func(map[string]any) {
	return func(envelope map[string]any) { envelope["expect"] = expect }
}

func (h *harness) ok(reply response, into any) {
	h.t.Helper()
	if !reply.OK {
		h.t.Fatalf("request failed: %+v", reply.Error)
	}
	if into != nil {
		if err := json.Unmarshal(reply.Result, into); err != nil {
			h.t.Fatalf("decode result: %v\n%s", err, reply.Result)
		}
	}
}

// rejects is refused for a caller that needs only the code checked.
func (h *harness) rejects(reply response, code string) {
	h.t.Helper()
	_ = h.refused(reply, code)
}

func (h *harness) refused(reply response, code string) *Error {
	h.t.Helper()
	if reply.OK || reply.Error == nil {
		h.t.Fatalf("request succeeded, want %s: %s", code, reply.Result)
	}
	if reply.Error.Code != code {
		h.t.Fatalf("error code %q (%s), want %q", reply.Error.Code, reply.Error.Message, code)
	}
	if reply.Error.Retryable != retryableCodes[code] {
		h.t.Fatalf("%s retryable = %v, want %v", code, reply.Error.Retryable, retryableCodes[code])
	}
	return reply.Error
}

func (h *harness) open(create bool) statusResult {
	h.t.Helper()
	var status statusResult
	h.ok(h.call("workspace.open", map[string]any{
		"data_dir": h.dataDir, "runtime_dir": h.runtimeDir, "device_id": "bridge-test-device", "create_if_missing": create,
	}), &status)
	h.workspace = status.Workspace
	return status
}

func (h *harness) unfiled() projectView {
	h.t.Helper()
	var projects projectsResult
	h.ok(h.call("query.projects", nil), &projects)
	for _, project := range projects.Active {
		if project.SystemUnfiled {
			return project
		}
	}
	h.t.Fatalf("no system Unfiled project among %+v", projects.Active)
	return projectView{}
}
