// Package mobileparity_test holds the desktop and the phone to one durable
// outcome. Each scenario runs twice over two private copies of the
// representative workspace under the same clock and identity sequence: once the
// way the desktop does it — the composer's capture reading, the inline editor's
// quick-edit grammar, the resume prompt, the workflow chord's planning and
// lifecycle calls — and once through the mobile bridge's JSON protocol. The two
// workspaces must then hold byte-identical task Markdown and planning state, and
// a refusal must refuse on both sides and change nothing on either.
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

func TestDesktopAndMobileCaptureAndEditAlike(t *testing.T) {
	t.Run("shorthand_capture", parityShorthandCapture)
	t.Run("shorthand_preview_matches_the_capture", parityShorthandPreviewMatchesTheCapture)
	t.Run("shorthand_refusal_names_the_line_and_commits_nothing", parityShorthandRefusalNamesTheLineAndCommitsNothing)
	t.Run("structured_capture", parityStructuredCapture)
	t.Run("daily_edit", parityDailyEdit)
	t.Run("deferral_routes_to_backlog", parityDeferralRoutesToBacklog)
	t.Run("today_next_exclusivity", parityTodayNextExclusivity)
	t.Run("completion_clears_planning", parityCompletionClearsPlanning)
	t.Run("lifecycle_refusals", parityLifecycleRefusals)
}

func parityShorthandCapture(t *testing.T) {
	p := newPair(t)
	text := "Plan the offsite !1 #ops ##work @2026-10-20 *pin =review $next\n\n" +
		"Book the venue %atlas-launch #ops $today\n" +
		"Renew passport @@2026-10-09 ;bring old photo;"
	results, err := p.desktop.capture(text)
	if err != nil || len(results) != 3 {
		t.Fatalf("desktop capture = %v, %v", results, err)
	}
	var captured struct {
		Tasks []struct {
			Title string `json:"title"`
			State string `json:"state"`
		} `json:"tasks"`
	}
	if err := p.mobile.call("command.capture", map[string]any{"text": text}, &captured); err != nil {
		t.Fatalf("mobile capture: %v", err)
	}
	if len(captured.Tasks) != 3 || captured.Tasks[0].Title != "Plan the offsite" || captured.Tasks[2].State != "backlog" {
		t.Fatalf("mobile captured %+v", captured.Tasks)
	}
	p.requireSameWorkspace()
}

func parityShorthandPreviewMatchesTheCapture(t *testing.T) {
	p := newPair(t)
	var preview struct {
		Lines []struct {
			Title       string   `json:"title"`
			ProjectName string   `json:"project_name"`
			Plan        string   `json:"plan"`
			State       string   `json:"state"`
			Priority    int      `json:"priority"`
			Tags        []string `json:"tags"`
		} `json:"lines"`
		Errors []struct {
			Line    int    `json:"line"`
			Column  int    `json:"column"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := p.mobile.call("query.capture_preview", map[string]any{"text": "Ship it !2 #ops $next\nFile taxes %atlas-launch"}, &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Errors) != 0 || len(preview.Lines) != 2 {
		t.Fatalf("preview = %+v", preview)
	}
	first, second := preview.Lines[0], preview.Lines[1]
	if first.Title != "Ship it" || first.Plan != "next" || first.State != "backlog" || first.Priority != 2 || len(first.Tags) != 1 {
		t.Fatalf("first preview line = %+v", first)
	}
	if second.ProjectName != "Atlas Launch" || second.State != "inbox" || second.Plan != "" {
		t.Fatalf("second preview line = %+v", second)
	}
	before := fingerprint(t, p.mobile.dataDir)
	if err := p.mobile.call("query.capture_preview", map[string]any{"text": "Fine\nBroken @notadate"}, &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Errors) != 1 || preview.Errors[0].Line != 2 || preview.Errors[0].Column != 7 || len(preview.Lines) != 0 {
		t.Fatalf("refused preview = %+v", preview)
	}
	if fingerprint(t, p.mobile.dataDir) != before {
		t.Fatal("a preview wrote to the workspace")
	}
}

func parityShorthandRefusalNamesTheLineAndCommitsNothing(t *testing.T) {
	p := newPair(t)
	text := "Fine line\nBad line $ready"
	desktopBefore, mobileBefore := fingerprint(t, p.desktop.dataDir), fingerprint(t, p.mobile.dataDir)
	_, desktopErr := p.desktop.capture(text)
	err := p.mobile.call("command.capture", map[string]any{"text": text}, nil)
	var refusal *wireError
	if desktopErr == nil || !errors.As(err, &refusal) || refusal.Code != "validation" {
		t.Fatalf("desktop %v, mobile %v: both must refuse", desktopErr, err)
	}
	lines, _ := refusal.Details["lines"].([]any)
	if len(lines) != 1 || lines[0].(map[string]any)["line"].(float64) != 2 ||
		!strings.Contains(desktopErr.Error(), lines[0].(map[string]any)["message"].(string)) {
		t.Fatalf("mobile line errors %v do not name the desktop's refusal %v", refusal.Details, desktopErr)
	}
	if fingerprint(t, p.desktop.dataDir) != desktopBefore || fingerprint(t, p.mobile.dataDir) != mobileBefore {
		t.Fatal("a refused capture wrote to a workspace")
	}
}

func parityStructuredCapture(t *testing.T) {
	p := newPair(t)
	results, err := p.desktop.capture("Call the plumber !2 #home ##phone @2026-10-21 *pin =review $backlog")
	if err != nil {
		t.Fatal(err)
	}
	created := app.TaskLocator{ProjectID: results[0].ProjectID, TaskID: results[0].TaskID}
	if err := p.desktop.describe(created, "Ask about the boiler."); err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.runtime.Service().SetResumeNote(created, "left a voicemail"); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.capture", map[string]any{
		"title": "Call the plumber", "priority": 2, "tags": []string{"home"}, "context": "phone",
		"due": "2026-10-21", "pinned": true, "review": true, "destination": "backlog",
		"description": "Ask about the boiler.", "resume_note": "left a voicemail",
	}, nil); err != nil {
		t.Fatalf("mobile structured capture: %v", err)
	}
	p.requireSameWorkspace()
}

func parityDailyEdit(t *testing.T) {
	p := newPair(t)
	target := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	if err := p.desktop.quickEdit(target, "Call the bank about the mortgage !1 #money ##phone @2026-10-12 *pin =review"); err != nil {
		t.Fatalf("desktop quick edit: %v", err)
	}
	if err := p.desktop.describe(target, "Ask for the fixed rate."); err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.runtime.Service().SetResumeNote(target, "have the account number ready"); err != nil {
		t.Fatal(err)
	}
	var edited struct {
		Changed bool `json:"changed"`
	}
	if err := p.mobile.call("command.edit", map[string]any{
		"task": wire(target), "title": "Call the bank about the mortgage", "priority": 1,
		"tags": []string{"money"}, "context": "phone", "due": "2026-10-12", "pinned": true, "review": true,
		"description": "Ask for the fixed rate.", "resume_note": "have the account number ready",
	}, &edited); err != nil || !edited.Changed {
		t.Fatalf("mobile edit = changed %v, %v", edited.Changed, err)
	}
	p.requireSameWorkspace()

	// Clearing goes through the same use case on both sides.
	if err := p.desktop.quickEdit(target, "Call the bank about the mortgage ##0 @0 -#money"); err != nil {
		t.Fatalf("desktop clearing edit: %v", err)
	}
	if err := p.mobile.call("command.edit", map[string]any{
		"task": wire(target), "context": "", "due": "", "tags": []string{},
	}, nil); err != nil {
		t.Fatalf("mobile clearing edit: %v", err)
	}
	p.requireSameWorkspace()

	// An edit naming only what is already stored commits nothing.
	before := fingerprint(t, p.mobile.dataDir)
	if err := p.mobile.call("command.edit", map[string]any{"task": wire(target), "priority": 1}, &edited); err != nil || edited.Changed {
		t.Fatalf("no-op edit = changed %v, %v", edited.Changed, err)
	}
	if fingerprint(t, p.mobile.dataDir) != before {
		t.Fatal("a no-op edit wrote to the workspace")
	}
}

func parityDeferralRoutesToBacklog(t *testing.T) {
	p := newPair(t)
	target := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	if err := p.desktop.quickEdit(target, "Call the bank @@2026-10-09"); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.edit", map[string]any{"task": wire(target), "defer": "2026-10-09"}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
	if ref := p.desktop.lookup(target); ref.Task.State() != models.StateBacklog || ref.Task.DeferredUntil == nil {
		t.Fatalf("deferred task = %s until %v", ref.Task.State(), ref.Task.DeferredUntil)
	}
}

func parityTodayNextExclusivity(t *testing.T) {
	p := newPair(t)
	target := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	service := p.desktop.runtime.Service()
	for _, action := range []app.PlanningAction{app.PlanningAddToday, app.PlanningAddNext, app.PlanningAddToday} {
		if _, err := service.ApplyPlanning(app.PlanningRequest{Locator: target, Action: action}); err != nil {
			t.Fatalf("desktop %s: %v", action, err)
		}
		if err := p.mobile.call("command.plan", map[string]any{"task": wire(target), "action": string(action)}, nil); err != nil {
			t.Fatalf("mobile %s: %v", action, err)
		}
		p.requireSameWorkspace()
		today := planning(t, p.desktop.runtime, models.QueryModeToday)
		next := planning(t, p.desktop.runtime, models.QueryModeNext)
		inToday, inNext := contains(today, "Call the bank"), contains(next, "Call the bank")
		if inToday == inNext || inToday != (action == app.PlanningAddToday) {
			t.Fatalf("after %s: today %v next %v", action, today, next)
		}
	}
	// Today is a membership, not a state: Doing keeps its state there, while
	// Next sends a task back to Backlog.
	if _, err := service.Transition(app.StateRequest{Locator: target, Target: models.StateDoing}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.transition", map[string]any{"task": wire(target), "state": "doing"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ApplyPlanning(app.PlanningRequest{Locator: target, Action: app.PlanningRemoveToday}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.plan", map[string]any{"task": wire(target), "action": "remove_today"}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
}

func parityCompletionClearsPlanning(t *testing.T) {
	p := newPair(t)
	target := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	service := p.desktop.runtime.Service()
	if _, err := service.ApplyPlanning(app.PlanningRequest{Locator: target, Action: app.PlanningAddToday}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.plan", map[string]any{"task": wire(target), "action": "add_today"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Transition(app.StateRequest{Locator: target, Target: models.StateDone}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.transition", map[string]any{"task": wire(target), "state": "done"}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
	if today := planning(t, p.desktop.runtime, models.QueryModeToday); contains(today, "Call the bank") {
		t.Fatalf("a completed task stayed in Today: %v", today)
	}
	ref := p.desktop.lookup(target)
	if ref.Task.State() != models.StateDone || ref.Task.CompletedAt == nil {
		t.Fatalf("completed task = %s at %v", ref.Task.State(), ref.Task.CompletedAt)
	}
	// Reopening goes back through the one lifecycle primitive on both sides.
	if _, err := service.Transition(app.StateRequest{Locator: target, Target: models.ReopenTargetState}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.transition", map[string]any{"task": wire(target), "state": string(models.ReopenTargetState)}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
}

func parityLifecycleRefusals(t *testing.T) {
	p := newPair(t)
	results, err := p.desktop.capture("Move house")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.capture("Pack the books >Move"); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.capture", map[string]any{"text": "Move house"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.capture", map[string]any{"text": "Pack the books >Move"}, nil); err != nil {
		t.Fatal(err)
	}
	p.requireSameWorkspace()
	parent := app.TaskLocator{ProjectID: results[0].ProjectID, TaskID: results[0].TaskID}
	bank := app.TaskLocator{ProjectID: unfiledProject, TaskID: callTheBank}
	service := p.desktop.runtime.Service()
	// A parent cannot finish over open work.
	_, desktopErr := service.Transition(app.StateRequest{Locator: parent, Target: models.StateDone})
	refusal := p.refusal("command.transition", map[string]any{"task": wire(parent), "state": "done"})
	var blocked *app.ChildrenBlockedError
	if !errors.As(desktopErr, &blocked) || refusal.Details["kind"] != "incomplete_children" ||
		!strings.Contains(refusal.Message, blocked.Error()) {
		t.Fatalf("desktop %v, mobile %+v", desktopErr, refusal)
	}
	children, _ := refusal.Details["children"].([]any)
	if len(children) != 1 || children[0].(map[string]any)["title"] != "Pack the books" {
		t.Fatalf("mobile names blocking children %v", refusal.Details["children"])
	}
	if p.desktop.lookup(parent).Task.IsTerminalState() {
		t.Fatal("a refused completion finished the parent")
	}
	p.requireSameWorkspace()

	// A finished task must be reopened before it moves anywhere else.
	if _, err := service.Transition(app.StateRequest{Locator: bank, Target: models.StateCanceled}); err != nil {
		t.Fatal(err)
	}
	if err := p.mobile.call("command.transition", map[string]any{"task": wire(bank), "state": "canceled"}, nil); err != nil {
		t.Fatal(err)
	}
	desktopBefore, mobileBefore := fingerprint(t, p.desktop.dataDir), fingerprint(t, p.mobile.dataDir)
	_, desktopErr = service.Transition(app.StateRequest{Locator: bank, Target: models.StateDoing})
	refusal = p.refusal("command.transition", map[string]any{"task": wire(bank), "state": "doing"})
	var transition *app.TransitionError
	if !errors.As(desktopErr, &transition) || refusal.Details["kind"] != "invalid_transition" ||
		refusal.Details["from"] != "canceled" || refusal.Details["to"] != "doing" || refusal.Details["reason"] != transition.Reason {
		t.Fatalf("desktop %v, mobile %+v", desktopErr, refusal)
	}

	// A finished task joins no plan.
	_, desktopErr = service.ApplyPlanning(app.PlanningRequest{Locator: bank, Action: app.PlanningAddNext})
	refusal = p.refusal("command.plan", map[string]any{"task": wire(bank), "action": "add_next"})
	var invalid *app.InvalidInputError
	if !errors.As(desktopErr, &invalid) || refusal.Details["kind"] != "invalid_input" ||
		refusal.Details["field"] != invalid.Field || refusal.Details["reason"] != invalid.Reason {
		t.Fatalf("desktop %v, mobile %+v", desktopErr, refusal)
	}

	// A field the shared gate refuses is refused on both sides.
	tooLong := strings.Repeat("x", 241)
	_, desktopErr = service.SetResumeNote(parent, tooLong)
	refusal = p.refusal("command.edit", map[string]any{"task": wire(parent), "resume_note": tooLong})
	if !errors.As(desktopErr, &invalid) || refusal.Details["field"] != "resume note" || refusal.Details["reason"] != invalid.Reason {
		t.Fatalf("desktop %v, mobile %+v", desktopErr, refusal)
	}
	if fingerprint(t, p.desktop.dataDir) != desktopBefore || fingerprint(t, p.mobile.dataDir) != mobileBefore {
		t.Fatal("a refusal wrote to a workspace")
	}
	p.requireSameWorkspace()
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
