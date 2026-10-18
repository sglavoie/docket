package mobileparity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/bridge"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

var representativeFixture = filepath.Join("..", "compat", "testdata", "workspaces", "representative")

const (
	unfiledProject = "system-unfiled-09e0f0ff-568e-4abd-89c7-75d0445b8e3f"
	callTheBank    = "0a7e5d10-2c3b-4f6a-8e9d-000000000501"
)

type fixedClock struct{ now time.Time }

func (c *fixedClock) Now() time.Time { return c.now }

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

// clockInstant is a local-time morning, so a due date read in local time and
// the Today route measured against this clock agree on the day.
func clockInstant() time.Time {
	return time.Date(2026, time.October, 6, 9, 0, 0, 0, time.Local)
}

// desktop is the desktop half: the headless runtime the TUI is built on, driven
// through the TUI's own parsers and request builders.
type desktop struct {
	t       *testing.T
	runtime *engine.Runtime
	dataDir string
}

func newDesktop(t *testing.T, root string) *desktop {
	t.Helper()
	dataDir := filepath.Join(root, "desktop")
	copyTree(t, representativeFixture, dataDir)
	runtime, err := engine.Open(engine.Config{
		DataDir:         dataDir,
		Machine:         engine.ExplicitMachine(filepath.Join(root, "desktop-home"), "desktop"),
		Clock:           &fixedClock{now: clockInstant()},
		IDs:             &sequentialIDs{},
		RefuseMigration: true,
	})
	if err != nil {
		t.Fatalf("open desktop: %v", err)
	}
	if _, err := runtime.ReconcilePlanning(); err != nil {
		t.Fatalf("reconcile desktop planning: %v", err)
	}
	return &desktop{t: t, runtime: runtime, dataDir: dataDir}
}

// capture reads text exactly as the global `a` composer does and commits it as
// one transaction.
func (d *desktop) capture(text string) ([]app.TaskResult, error) {
	d.t.Helper()
	if err := d.runtime.Refresh(); err != nil {
		d.t.Fatal(err)
	}
	state := d.runtime.State()
	snapshot, err := d.runtime.Snapshot()
	if err != nil {
		d.t.Fatal(err)
	}
	preset := taskform.TaskComposerPreset{
		Kind:               taskform.ComposerGlobal,
		Project:            snapshot.ProjectByID(state.SystemUnfiledProject().ID),
		AllProjects:        snapshot.Projects(),
		AllowProjectSwitch: true,
		Shared:             taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}
	reqs, problems := taskform.ParseCaptureText(preset, state, text)
	if len(problems) > 0 {
		return nil, fmt.Errorf("line %d: %s", problems[0].Line, problems[0].Message)
	}
	return d.runtime.Service().CaptureMany(reqs)
}

// lookup resolves one task in a fresh snapshot.
func (d *desktop) lookup(target app.TaskLocator) app.TaskRef {
	d.t.Helper()
	if err := d.runtime.Refresh(); err != nil {
		d.t.Fatal(err)
	}
	snapshot, err := d.runtime.Snapshot()
	if err != nil {
		d.t.Fatal(err)
	}
	ref, ok := snapshot.LookupLocator(target)
	if !ok {
		d.t.Fatalf("desktop task %v does not resolve", target)
	}
	return ref
}

// quickEdit submits one inline-editor line the way the Outliner's `e` does.
func (d *desktop) quickEdit(target app.TaskLocator, line string) error {
	d.t.Helper()
	ref := d.lookup(target)
	snapshot, err := d.runtime.Snapshot()
	if err != nil {
		return err
	}
	result, err := grammar.ParseQuickEdit(line, ref.Task, ref.Project, snapshot.Projects())
	if err != nil {
		return err
	}
	_, err = d.runtime.Service().EditTaskLine(outliner.TaskLineEditRequest(taskref.For(ref.Task, ref.Project), result))
	return err
}

// describe is the Ctrl+E external-editor round trip: the title it returns
// unchanged and the body it rewrote, in one line edit.
func (d *desktop) describe(target app.TaskLocator, description string) error {
	ref := d.lookup(target)
	_, err := d.runtime.Service().EditTaskLine(app.TaskLineEditRequest{
		Source: target, Title: ref.Task.Title, Description: &description,
		DestinationProjectID: target.ProjectID,
	})
	return err
}

// mobile is the phone half: the bridge over its own copy, spoken to in JSON.
type mobile struct {
	t         *testing.T
	bridge    *bridge.Bridge
	dataDir   string
	workspace string
	requests  int
}

type wireError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func (e *wireError) Error() string { return e.Code + ": " + e.Message }

func newMobile(t *testing.T, root string) *mobile {
	t.Helper()
	dataDir := filepath.Join(root, "mobile")
	copyTree(t, representativeFixture, dataDir)
	m := &mobile{
		t:       t,
		bridge:  bridge.New(bridge.Options{Clock: &fixedClock{now: clockInstant()}, IDs: &sequentialIDs{}}),
		dataDir: dataDir,
	}
	var status struct {
		Workspace string `json:"workspace"`
	}
	if err := m.call("workspace.open", map[string]any{
		"data_dir": dataDir, "runtime_dir": filepath.Join(root, "phone-runtime"), "device_id": "phone",
	}, &status); err != nil {
		t.Fatalf("open mobile: %v", err)
	}
	m.workspace = status.Workspace
	return m
}

func (m *mobile) call(op string, payload any, into any) error {
	m.t.Helper()
	m.requests++
	request := map[string]any{"bridge_version": bridge.Version, "request_id": fmt.Sprintf("r%d", m.requests), "op": op, "payload": payload}
	if m.workspace != "" {
		request["workspace"] = m.workspace
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		m.t.Fatal(err)
	}
	var reply struct {
		OK     bool            `json:"ok"`
		Result json.RawMessage `json:"result"`
		Error  *wireError      `json:"error"`
	}
	if err := json.Unmarshal([]byte(m.bridge.Call(string(encoded))), &reply); err != nil {
		m.t.Fatal(err)
	}
	if !reply.OK {
		return reply.Error
	}
	if into != nil {
		if err := json.Unmarshal(reply.Result, into); err != nil {
			m.t.Fatal(err)
		}
	}
	return nil
}

type wireLocator struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
}

func wire(target app.TaskLocator) wireLocator {
	return wireLocator{ProjectID: target.ProjectID, TaskID: target.TaskID}
}

// pair runs one scenario on both halves.
type pair struct {
	t       *testing.T
	desktop *desktop
	mobile  *mobile
}

func newPair(t *testing.T) *pair {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "process-home"))
	return &pair{t: t, desktop: newDesktop(t, root), mobile: newMobile(t, root)}
}

// requireSameWorkspace compares every project's task Markdown and the planning
// artifact byte for byte.
func (p *pair) requireSameWorkspace() {
	p.t.Helper()
	p.requireSameFiles(func(data []byte) []byte { return data })
}

// requireSameFiles compares the same files after normalize.
func (p *pair) requireSameFiles(normalize func([]byte) []byte) {
	p.t.Helper()
	files := []string{filepath.Join(".tt", "planning.json")}
	matches, err := filepath.Glob(filepath.Join(p.desktop.dataDir, "projects", "*", "tasks.md"))
	if err != nil || len(matches) == 0 {
		p.t.Fatalf("no desktop task files: %v", err)
	}
	for _, match := range matches {
		relative, _ := filepath.Rel(p.desktop.dataDir, match)
		files = append(files, relative)
	}
	for _, file := range files {
		left, leftErr := os.ReadFile(filepath.Join(p.desktop.dataDir, file))
		right, rightErr := os.ReadFile(filepath.Join(p.mobile.dataDir, file))
		if leftErr != nil || rightErr != nil {
			p.t.Fatalf("read %s: desktop %v, mobile %v", file, leftErr, rightErr)
		}
		if left, right = normalize(left), normalize(right); !bytes.Equal(left, right) {
			p.t.Fatalf("%s differs between desktop and mobile:\n--- desktop\n%s\n--- mobile\n%s", file, left, right)
		}
	}
}

// fingerprint is the durable state a refusal must leave untouched.
func fingerprint(t *testing.T, dataDir string) string {
	t.Helper()
	var b strings.Builder
	paths, _ := filepath.Glob(filepath.Join(dataDir, "projects", "*", "tasks.md"))
	paths = append(paths, filepath.Join(dataDir, ".tt", "planning.json"))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString(path)
		b.Write(data)
	}
	return b.String()
}

// planning reads one workspace's Today and Next memberships through the runtime
// query both frontends list them with.
func planning(t *testing.T, runtime *engine.Runtime, mode models.QueryMode) []string {
	t.Helper()
	query, err := models.NewQuery(mode, "", models.QuerySort{})
	if err != nil {
		t.Fatal(err)
	}
	answer, err := runtime.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, item := range answer.Items {
		titles = append(titles, item.Task.Title)
	}
	return titles
}

// refusal calls the bridge expecting a validation refusal.
func (p *pair) refusal(op string, payload any) *wireError {
	p.t.Helper()
	err := p.mobile.call(op, payload, nil)
	var refusal *wireError
	if !errors.As(err, &refusal) || refusal.Code != "validation" {
		p.t.Fatalf("%s = %v, want a validation refusal", op, err)
	}
	return refusal
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
