package bridge

import (
	"encoding/json"
	"testing"
	"time"
)

// TestBridgeCapturesAndEditsThroughTheSharedEngine holds the capture and
// daily-edit ops to their envelope: what each one accepts, what it refuses and
// how it says so, and what it reports back. The durable outcomes themselves are
// compared with the desktop's in internal/mobileparity.
func TestBridgeCapturesAndEditsThroughTheSharedEngine(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	f := &captureFixture{h: h, unfiled: h.unfiled()}
	t.Run("grammar_capture_is_one_transaction", f.grammarCaptureIsOneTransaction)
	t.Run("grammar_refusal_names_every_line", f.grammarRefusalNamesEveryLine)
	t.Run("text_stands_alone", f.textStandsAlone)
	t.Run("structured_capture_keeps_a_literal_title", f.structuredCaptureKeepsALiteralTitle)
	t.Run("preview_reads_without_writing", f.previewReadsWithoutWriting)
	t.Run("edit_applies_named_fields_once", f.editAppliesNamedFieldsOnce)
	t.Run("plan_keeps_today_and_next_exclusive", f.planKeepsTodayAndNextExclusive)
	t.Run("lifecycle_refusals_explain_themselves", f.lifecycleRefusalsExplainThemselves)
}

// captureFixture is the one workspace the subtests share, in order: the edit
// subtest captures the task the planning and lifecycle subtests then move.
type captureFixture struct {
	h       *harness
	unfiled projectView
	target  locator
}

func (f *captureFixture) grammarCaptureIsOneTransaction(t *testing.T) {
	h := f.h
	var captured captureResult
	h.ok(h.call("command.capture", map[string]any{"text": "Water the plants #home $today\n\nBuy stamps !2"}), &captured)
	if len(captured.Tasks) != 2 || captured.Task.TaskID != captured.Tasks[0].TaskID {
		t.Fatalf("captured %+v", captured)
	}
	first, second := captured.Tasks[0], captured.Tasks[1]
	if first.Title != "Water the plants" || len(first.Tags) != 1 || first.State != "backlog" ||
		second.Title != "Buy stamps" || second.Priority != 2 || second.State != "inbox" {
		t.Fatalf("captured %+v and %+v", first, second)
	}
	if captured.Committed.TransactionID == "" || captured.Committed.Domains["planning"] == "" {
		t.Fatalf("a Today line reports no planning revision: %+v", captured.Committed)
	}
	var read taskResult
	h.ok(h.call("query.task", map[string]any{"task": locator{first.ProjectID, first.TaskID}}), &read)
	if !read.Today || read.Next {
		t.Fatalf("captured Today task reads today %v next %v", read.Today, read.Next)
	}
}

func (f *captureFixture) grammarRefusalNamesEveryLine(t *testing.T) {
	h := f.h
	before := h.revisions()
	refusal := h.refused(h.call("command.capture", map[string]any{"text": "Fine\n  Bad @notadate\nWorse $ready"}), CodeValidation)
	lines, _ := refusal.Details["lines"].([]any)
	if len(lines) != 2 {
		t.Fatalf("line errors = %v", refusal.Details)
	}
	first := lines[0].(map[string]any)
	if first["line"].(float64) != 2 || first["column"].(float64) != 6 {
		t.Fatalf("first refusal = %v, want line 2 column 6", first)
	}
	if after := h.revisions(); after != before {
		t.Fatal("a refused capture moved a revision")
	}
}

func (f *captureFixture) textStandsAlone(t *testing.T) {
	h := f.h
	h.rejects(h.call("command.capture", map[string]any{"text": "Ship", "title": "Ship"}), CodeInvalidRequest)
	h.rejects(h.call("command.capture", map[string]any{"text": "Ship", "priority": 1}), CodeInvalidRequest)
	refusal := h.refused(h.call("command.capture", map[string]any{"text": "   \n "}), CodeValidation)
	if refusal.Details["lines"] == nil {
		t.Fatalf("a blank capture names no line: %+v", refusal)
	}
}

func (f *captureFixture) structuredCaptureKeepsALiteralTitle(t *testing.T) {
	h := f.h
	var captured captureResult
	h.ok(h.call("command.capture", map[string]any{
		"project_id": f.unfiled.ID, "title": "Read #3 $next", "resume_note": "chapter two",
		"due": "2026-10-06", "defer": "", "context": "Home",
	}), &captured)
	task := captured.Task
	if task.Title != "Read #3 $next" || task.ResumeNote != "chapter two" || task.Context != "home" || task.DueDate == nil {
		t.Fatalf("structured capture = %+v", task)
	}
	var read taskResult
	h.ok(h.call("query.task", map[string]any{"task": locator{task.ProjectID, task.TaskID}}), &read)
	if !read.Today {
		t.Fatal("a deadline falling today did not route to Today")
	}
	refusal := h.refused(h.call("command.capture", map[string]any{"title": "Bad", "due": "someday-ish"}), CodeValidation)
	if refusal.Details["field"] != "due" {
		t.Fatalf("bad due refusal = %+v", refusal)
	}
	refusal = h.refused(h.call("command.capture", map[string]any{"title": "Bad", "priority": 9}), CodeValidation)
	if refusal.Details["kind"] != "invalid_input" || refusal.Details["field"] != "priority" {
		t.Fatalf("bad priority refusal = %+v", refusal)
	}
}

func (f *captureFixture) previewReadsWithoutWriting(t *testing.T) {
	h := f.h
	before := h.revisions()
	var preview capturePreviewResult
	h.ok(h.call("query.capture_preview", map[string]any{"text": "Call mum ##phone $next"}), &preview)
	if len(preview.Lines) != 1 || preview.Lines[0].Plan != "next" || preview.Lines[0].Context != "phone" || len(preview.Errors) != 0 {
		t.Fatalf("preview = %+v", preview)
	}
	h.ok(h.call("query.capture_preview", map[string]any{"text": "Call mum $nope"}), &preview)
	if len(preview.Errors) != 1 || len(preview.Lines) != 0 {
		t.Fatalf("refused preview = %+v", preview)
	}
	h.ok(h.call("query.capture_preview", map[string]any{"text": ""}), &preview)
	if len(preview.Lines) != 0 || len(preview.Errors) != 0 {
		t.Fatalf("empty preview = %+v", preview)
	}
	if after := h.revisions(); after != before {
		t.Fatal("a preview moved a revision")
	}
}

func (f *captureFixture) editAppliesNamedFieldsOnce(t *testing.T) {
	h := f.h
	var captured captureResult
	h.ok(h.call("command.capture", map[string]any{"title": "Fix the gate", "tags": []string{"yard", "diy"}}), &captured)
	f.target = locator{captured.Task.ProjectID, captured.Task.TaskID}
	var edited editResult
	request := map[string]any{"task": f.target, "tags": []string{"DIY", "urgent"}, "review": true, "resume_note": "hinge ordered"}
	h.ok(h.call("command.edit", request, withRequestID("edit-1")), &edited)
	if !edited.Changed || edited.Committed.TransactionID == "" || edited.Task.Title != "Fix the gate" ||
		!edited.Task.Review || edited.Task.ResumeNote != "hinge ordered" {
		t.Fatalf("edit = %+v", edited)
	}
	if got := edited.Task.Tags; len(got) != 2 || got[0] != "diy" || got[1] != "urgent" {
		t.Fatalf("tags = %v, want the complete new set", got)
	}
	var replayed editResult
	h.ok(h.call("command.edit", request, withRequestID("edit-1")), &replayed)
	if replayed.Committed.TransactionID != edited.Committed.TransactionID {
		t.Fatal("a replayed edit committed again")
	}
	var unchanged editResult
	h.ok(h.call("command.edit", map[string]any{"task": f.target, "review": true}), &unchanged)
	if unchanged.Changed || unchanged.Committed.TransactionID != "" {
		t.Fatalf("a no-op edit = %+v", unchanged)
	}
	h.rejects(h.call("command.edit", map[string]any{"task": f.target, "title": "  "}), CodeValidation)
	h.rejects(h.call("command.edit", map[string]any{"task": locator{f.target.ProjectID, "missing"}, "review": false}), CodeNotFound)
	h.rejects(h.call("command.edit", map[string]any{"task": f.target, "shade": "red"}), CodeInvalidRequest)
}

func (f *captureFixture) planKeepsTodayAndNextExclusive(t *testing.T) {
	h := f.h
	var read taskResult
	for _, step := range []struct {
		action      string
		today, next bool
	}{
		{"add_next", false, true},
		{"add_today", true, false},
		{"remove_today", false, false},
		{"add_next", false, true},
		{"backlog", false, false},
	} {
		h.ok(h.call("command.plan", map[string]any{"task": f.target, "action": step.action}), nil)
		h.ok(h.call("query.task", map[string]any{"task": f.target}), &read)
		if read.Today != step.today || read.Next != step.next {
			t.Fatalf("after %s: today %v next %v", step.action, read.Today, read.Next)
		}
	}
	h.rejects(h.call("command.plan", map[string]any{"task": f.target, "action": "someday"}), CodeInvalidRequest)
}

func (f *captureFixture) lifecycleRefusalsExplainThemselves(t *testing.T) {
	h := f.h
	h.ok(h.call("command.transition", map[string]any{"task": f.target, "state": "done"}), nil)
	refusal := h.refused(h.call("command.transition", map[string]any{"task": f.target, "state": "waiting"}), CodeValidation)
	if refusal.Details["kind"] != "invalid_transition" || refusal.Details["from"] != "done" || refusal.Details["to"] != "waiting" {
		t.Fatalf("terminal move = %+v", refusal)
	}
	refusal = h.refused(h.call("command.plan", map[string]any{"task": f.target, "action": "add_today"}), CodeValidation)
	if refusal.Details["kind"] != "invalid_input" || refusal.Details["field"] != "planning" {
		t.Fatalf("terminal plan = %+v", refusal)
	}
	refusal = h.refused(h.call("command.edit", map[string]any{"task": f.target, "defer": "2026-10-09"}), CodeValidation)
	if refusal.Details["kind"] == nil {
		t.Fatalf("terminal deferral = %+v", refusal)
	}
}

// revisions is every revision a refusal must leave alone.
func (h *harness) revisions() string {
	h.t.Helper()
	var status statusResult
	h.ok(h.call("workspace.status", nil), &status)
	encoded, err := json.Marshal(status.Revisions)
	if err != nil {
		h.t.Fatal(err)
	}
	return string(encoded)
}

// TestBridgeAdoptsTheDeviceTimeZone holds open to the phone's zone: a due date
// is a local calendar day and Today a local day, so the engine reads both in
// the zone the device names, and a name it cannot resolve opens nothing.
func TestBridgeAdoptsTheDeviceTimeZone(t *testing.T) {
	previous := time.Local
	t.Cleanup(func() { time.Local = previous })
	h := newHarness(t, representativeFixture)
	h.rejects(h.call("workspace.open", map[string]any{
		"data_dir": h.dataDir, "runtime_dir": h.runtimeDir, "device_id": "bridge-test-device", "time_zone": "Mars/Olympus",
	}), CodeInvalidRequest)
	if time.Local != previous {
		t.Fatal("a refused zone changed the process zone")
	}
	var status statusResult
	h.ok(h.call("workspace.open", map[string]any{
		"data_dir": h.dataDir, "runtime_dir": h.runtimeDir, "device_id": "bridge-test-device", "time_zone": "Pacific/Kiritimati",
	}), &status)
	h.workspace = status.Workspace
	if time.Local.String() != "Pacific/Kiritimati" {
		t.Fatalf("local zone = %s", time.Local)
	}
	// 2026-10-06 20:00 UTC is already 2026-10-07 10:00 in Kiritimati (UTC+14),
	// so a deadline on the 7th falls today there, though not in UTC.
	h.clock.advance(11 * time.Hour)
	var captured captureResult
	h.ok(h.call("command.capture", map[string]any{"title": "Island errand", "due": "2026-10-07"}), &captured)
	var read taskResult
	h.ok(h.call("query.task", map[string]any{"task": locator{captured.Task.ProjectID, captured.Task.TaskID}}), &read)
	if !read.Today {
		t.Fatal("a deadline falling today in the device zone did not route to Today")
	}
	if got := captured.Task.DueDate.In(time.Local).Format("2006-01-02"); got != "2026-10-07" {
		t.Fatalf("due day in the device zone = %s", got)
	}
	// Tomorrow is the 8th on the device, though the injected clock still
	// carries UTC's 6th. Written and relative deferrals must name one instant.
	wake := time.Date(2026, time.October, 8, 0, 0, 0, 0, time.Local)
	for _, input := range []string{"tomorrow", "2026-10-08", "10/08/2026", "08.10.2026"} {
		var deferred captureResult
		h.ok(h.call("command.capture", map[string]any{"title": "Island follow-up", "defer": input}), &deferred)
		if got := deferred.Task.DeferredUntil; got == nil || !got.Equal(wake) {
			t.Errorf("defer %q = %v, want %v", input, got, wake)
		}
	}
}
