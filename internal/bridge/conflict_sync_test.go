package bridge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/syncmerge"
)

// TestBridgeResolvesSyncConflictsFromEitherDevice drives conflict resolution
// from both interfaces over one engine: the phone through conflict.list,
// conflict.show and conflict.resolve, and the desktop through the engine call
// `tt sync resolve` makes. Each side reads the base and every competing
// version with the device and change that wrote it, resolves by keeping a
// version, the current bytes, or replacement text, and every replica converges
// once the resolution travels. A change that spanned files is resolved whole;
// a resolution read before an edit is refused without writing; a resolution
// racing another device's edit or resolution keeps every variant; and a
// transport failure leaves local content, conflicts included, readable and
// editable until a retry delivers.
func TestBridgeResolvesSyncConflictsFromEitherDevice(t *testing.T) {
	t.Run("inspect_base_and_competing_versions", conflictInspect)
	t.Run("resolve_from_phone_converges", conflictResolveFromPhone)
	t.Run("resolve_from_desktop_converges", conflictResolveFromDesktop)
	t.Run("replacement_text", conflictReplacementText)
	t.Run("coupled_changes_resolve_together", conflictCoupledChanges)
	t.Run("stale_resolution_is_refused", conflictStaleResolution)
	t.Run("missing_derived_result", conflictMissingDerivedResult)
	t.Run("external_edit_invalidates_resolution", conflictExternalEditInvalidatesResolution)
	t.Run("resolution_racing_an_edit_keeps_both", conflictResolutionRacesEdit)
	t.Run("competing_resolutions_keep_both", conflictCompetingResolutions)
	t.Run("transport_failure_keeps_local_content", conflictTransportFailure)
	t.Run("diagnostics_export", conflictDiagnostics)
}

const atlasTasks = "projects/atlas-launch/tasks.md"

// History can derive a merged result that was never retained, while the
// working file still contains a competing version. Resolution must read and
// guard the actual working file, not require that synthetic blob.
func conflictMissingDerivedResult(t *testing.T) {
	for _, choice := range []string{engine.ResolveVersion, engine.ResolveCurrent, engine.ResolveReplace} {
		t.Run(choice, func(t *testing.T) {
			p := newPair(t)
			ship := task(atlasProject, shipBetaTask)
			p.phone.retitle(ship, "Ship beta build (phone)")
			p.phone.retitle(task(atlasProject, proofreadTask), "Proofread (phone)")
			p.desktop.retitle(ship, "Ship beta build (desktop)")
			p.desktop.retitle(task(atlasProject, screenshotsTask), "Screenshots (desktop)")
			p.settle()
			view := viewOn(t, p.desktop.conflictViews(), atlasTasks)
			current := view.Versions[versionHolding(t, view, "Ship beta build (desktop)")]
			history := p.desktop.bridge.workspace.runtime.SyncCommits().SyncHistory()
			derived, err := history.OpenConflicts(syncmerge.Merger{})
			if err != nil || len(derived) != 1 || derived[0].ResultBlob == nil {
				t.Fatalf("derived conflicts: %+v, %v", derived, err)
			}
			for _, version := range view.Versions {
				if version.Digest == *derived[0].ResultBlob {
					t.Fatal("fixture needs a synthetic result distinct from both candidates")
				}
			}
			if err := os.WriteFile(filepath.Join(p.desktop.dataDir, atlasTasks), []byte(*current.Text), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(history.BlobPath(*derived[0].ResultBlob)); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			view = viewOn(t, p.desktop.conflictViews(), atlasTasks)
			if view.Current == nil || !view.Current.Available || view.Current.Text == nil || *view.Current.Text != *current.Text {
				t.Fatalf("current content does not describe the working file: %+v", view.Current)
			}
			request := engine.ResolveRequest{ConflictID: view.ID, Choice: choice, Version: versionHolding(t, view, "Ship beta build (phone)"), Text: strings.Replace(*current.Text, "Ship beta build (desktop)", "Ship beta build (merged)", 1)}
			result := p.desktop.desktopResolve(request)
			if result.Remaining != 0 || len(p.settle()) != 0 {
				t.Fatal("resolution did not close the conflict on both devices")
			}
			want := "Ship beta build (desktop)"
			switch choice {
			case engine.ResolveVersion:
				want = "Ship beta build (phone)"
				requireTitle(t, p, task(atlasProject, proofreadTask), "Proofread (phone)")
			case engine.ResolveReplace:
				want = "Ship beta build (merged)"
			}
			requireTitle(t, p, ship, want)
			requireTitle(t, p, task(atlasProject, screenshotsTask), "Screenshots (desktop)")
		})
	}
}

func conflictExternalEditInvalidatesResolution(t *testing.T) {
	p, _ := titleConflict(t)
	view := viewOn(t, p.desktop.conflictViews(), atlasTasks)
	changed := strings.ReplaceAll(*view.Current.Text, "Ship beta build", "Edited outside the app")
	path := filepath.Join(p.desktop.dataDir, atlasTasks)
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := p.desktop.bridge.workspace.runtime.ResolveSyncConflict(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveCurrent})
	if !errors.Is(err, engine.ErrConflictStale) {
		t.Fatalf("old confirmation accepted an external edit: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != changed {
		t.Fatalf("external edit changed after refusal: %v", err)
	}
	updated := viewOn(t, p.desktop.conflictViews(), atlasTasks)
	if updated.ID == view.ID || updated.Current.Text == nil || *updated.Current.Text != changed {
		t.Fatal("reopening did not bind the conflict to the new working file")
	}
	p.desktop.desktopResolve(engine.ResolveRequest{ConflictID: updated.ID, Choice: engine.ResolveCurrent})
	if len(p.settle()) != 0 {
		t.Fatal("keeping the reopened external edit did not resolve the conflict")
	}
	for _, h := range []*harness{p.phone, p.desktop} {
		if got := string(syncedFiles(t, h.dataDir)[atlasTasks]); got != changed {
			t.Fatal("keeping current did not preserve the exact external edit")
		}
	}
}

// retitle edits a task's title through command.edit on either device.
func (h *harness) retitle(target locator, title string) {
	h.t.Helper()
	h.ok(h.call("command.edit", map[string]any{"task": target, "title": title}), nil)
}

func (h *harness) conflictViews() []engine.ConflictView {
	h.t.Helper()
	var listed conflictListResult
	h.ok(h.call("conflict.list", nil), &listed)
	return listed.Conflicts
}

func viewOn(t *testing.T, views []engine.ConflictView, path string) engine.ConflictView {
	t.Helper()
	for _, view := range views {
		if view.Path == path {
			return view
		}
	}
	t.Fatalf("no open conflict on %s among %d", path, len(views))
	return engine.ConflictView{}
}

// versionHolding is the index of the version whose text contains marker.
func versionHolding(t *testing.T, view engine.ConflictView, marker string) int {
	t.Helper()
	for _, version := range view.Versions {
		if version.Text != nil && strings.Contains(*version.Text, marker) {
			return version.Index
		}
	}
	t.Fatalf("no version of %s holds %q", view.Path, marker)
	return -1
}

func (h *harness) resolve(request engine.ResolveRequest) engine.ResolveResult {
	h.t.Helper()
	var result engine.ResolveResult
	h.ok(h.call("conflict.resolve", request), &result)
	return result
}

// desktopResolve is the desktop's resolution: the engine call `tt sync
// resolve` makes, on the desktop's own runtime.
func (h *harness) desktopResolve(request engine.ResolveRequest) *engine.ResolveResult {
	h.t.Helper()
	result, err := h.bridge.workspace.runtime.ResolveSyncConflict(request)
	if err != nil {
		h.t.Fatalf("desktop resolution: %v", err)
	}
	return result
}

// titleConflict leaves one conflict on the Atlas tasks: both devices retitle
// Ship beta build while apart.
func titleConflict(t *testing.T) (*pair, locator) {
	t.Helper()
	p := newPair(t)
	ship := task(atlasProject, shipBetaTask)
	p.phone.retitle(ship, "Ship beta build (phone)")
	p.desktop.retitle(ship, "Ship beta build (desktop)")
	if conflicts := p.settle(); len(conflicts) != 1 {
		t.Fatalf("two retitles left %d conflicts", len(conflicts))
	}
	return p, ship
}

func requireTitle(t *testing.T, p *pair, target locator, want string) {
	t.Helper()
	for _, h := range []*harness{p.phone, p.desktop} {
		if got := h.detail(target.ProjectID, target.TaskID).Task.Title; got != want {
			t.Fatalf("a device reads the title %q, want %q", got, want)
		}
	}
}

func conflictInspect(t *testing.T) {
	p, _ := titleConflict(t)
	for _, side := range []struct {
		h    *harness
		self string
	}{{p.phone, "phone-device"}, {p.desktop, "desktop-device"}} {
		view := viewOn(t, side.h.conflictViews(), atlasTasks)
		if view.Kind != "project_document" || view.Label != "Tasks in Atlas Launch" || !view.Replaceable {
			t.Fatalf("the conflict describes itself as %q %q replaceable=%v", view.Kind, view.Label, view.Replaceable)
		}
		if view.Base == nil || view.Base.Text == nil || !strings.Contains(*view.Base.Text, "## Task: Ship beta build\n") {
			t.Fatalf("the base is not the version both started from: %+v", view.Base)
		}
		if len(view.Versions) != 2 || len(view.Details) == 0 {
			t.Fatalf("want two versions and the differing field, got %+v", view)
		}
		current := 0
		for _, marker := range []string{"(phone)", "(desktop)"} {
			version := view.Versions[versionHolding(t, view, marker)]
			if version.Current {
				current++
			}
			if len(version.Writers) != 1 {
				t.Fatalf("version %q names writers %+v", marker, version.Writers)
			}
			writer := version.Writers[0]
			wantDevice := strings.Trim(marker, "()") + "-device"
			if writer.DeviceID != wantDevice || writer.ThisDevice != (wantDevice == side.self) || writer.Change != "edit" || writer.CreatedAt == "" || writer.Files == 0 {
				t.Fatalf("version %q is attributed to %+v on %s", marker, writer, side.self)
			}
		}
		if current != 1 || view.Current == nil || view.Current.Text == nil {
			t.Fatalf("exactly one version is what the workspace holds: %+v", view)
		}
		var shown conflictShowResult
		side.h.ok(side.h.call("conflict.show", map[string]any{"conflict_id": view.ID, "device_names": map[string]string{"desktop-device": "Studio Mac"}}), &shown)
		named := false
		for _, version := range shown.Conflict.Versions {
			for _, writer := range version.Writers {
				named = named || writer.DeviceName == "Studio Mac"
			}
		}
		if shown.Conflict.ID != view.ID || !named {
			t.Fatalf("conflict.show does not attribute by advertised name: %+v", shown.Conflict)
		}
	}
	p.phone.rejects(p.phone.call("conflict.show", map[string]any{"conflict_id": "sha256:" + strings.Repeat("0", 64)}), CodeNotFound)
}

func conflictResolveFromPhone(t *testing.T) {
	p, ship := titleConflict(t)
	view := viewOn(t, p.phone.conflictViews(), atlasTasks)
	result := p.phone.resolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, view, "(desktop)")})
	if result.TxnID == "" || result.Remaining != 0 || len(result.Resolved) != 1 || result.Resolved[0].ID != view.ID {
		t.Fatalf("the phone's resolution reports %+v", result)
	}
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("after the phone's resolution traveled %d conflicts remain", len(conflicts))
	}
	requireTitle(t, p, ship, "Ship beta build (desktop)")

	// Keeping a version keeps that side of what conflicted and nothing less:
	// the other side's independent change in the same file survives.
	q := newPair(t)
	q.phone.retitle(ship, "Ship beta build (phone)")
	q.desktop.retitle(ship, "Ship beta build (desktop)")
	q.desktop.retitle(task(atlasProject, proofreadTask), "Proofread (desktop)")
	q.settle()
	view = viewOn(t, q.phone.conflictViews(), atlasTasks)
	q.phone.resolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, view, "(phone)")})
	if conflicts := q.settle(); len(conflicts) != 0 {
		t.Fatalf("%d conflicts remain", len(conflicts))
	}
	requireTitle(t, q, ship, "Ship beta build (phone)")
	requireTitle(t, q, task(atlasProject, proofreadTask), "Proofread (desktop)")
}

func conflictResolveFromDesktop(t *testing.T) {
	p, ship := titleConflict(t)
	view := viewOn(t, p.desktop.conflictViews(), atlasTasks)
	result := p.desktop.desktopResolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, view, "(phone)")})
	if result.Remaining != 0 {
		t.Fatalf("the desktop's resolution left %d open", result.Remaining)
	}
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("after the desktop's resolution traveled %d conflicts remain", len(conflicts))
	}
	requireTitle(t, p, ship, "Ship beta build (phone)")

	// Keeping the current bytes is a resolution too, and it travels.
	p.phone.retitle(ship, "Ship it (phone)")
	p.desktop.retitle(ship, "Ship it (desktop)")
	p.settle()
	view = viewOn(t, p.desktop.conflictViews(), atlasTasks)
	p.desktop.desktopResolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveCurrent})
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("keeping the current version left %d conflicts", len(conflicts))
	}
	if *view.Current.Text != string(syncedFiles(t, p.phone.dataDir)[atlasTasks]) {
		t.Fatal("the phone does not hold the version the desktop kept")
	}
}

func conflictReplacementText(t *testing.T) {
	p := newPair(t)
	var read noteResult
	p.phone.ok(p.phone.call("query.note", map[string]any{"location": atlasNotes(), "filename": releaseRunbook}), &read)
	p.phone.ok(p.phone.call("command.note", map[string]any{
		"action": "update", "location": atlasNotes(), "filename": releaseRunbook, "revision": read.Note.Revision, "body": "Phone: run it from CI.",
	}), nil)
	var desk noteResult
	p.desktop.ok(p.desktop.call("query.note", map[string]any{"location": atlasNotes(), "filename": releaseRunbook}), &desk)
	p.desktop.ok(p.desktop.call("command.note", map[string]any{
		"action": "update", "location": atlasNotes(), "filename": releaseRunbook, "revision": desk.Note.Revision, "body": "Desktop: run it by hand.",
	}), nil)
	p.settle()
	path := "projects/atlas-launch/notes/" + releaseRunbook
	view := viewOn(t, p.phone.conflictViews(), path)
	if !view.Replaceable || view.Label != "Note “Release runbook” in Atlas Launch" {
		t.Fatalf("a note conflict takes replacement text: %+v", view)
	}
	merged := strings.Replace(*view.Current.Text, *view.Current.Text, *view.Versions[0].Text, 1)
	merged = strings.Replace(merged, "Phone: run it from CI.", "Run it from CI; by hand if CI is down.", 1)
	merged = strings.Replace(merged, "Desktop: run it by hand.", "Run it from CI; by hand if CI is down.", 1)

	// A planning conflict takes a version, never text, and text the readers
	// would refuse is refused; neither writes anything.
	p.phone.unchanged("an invalid replacement", func() {
		refusal := p.phone.refused(p.phone.call("conflict.resolve", engine.ResolveRequest{ConflictID: view.ID, Choice: "merge"}), CodeValidation)
		if refusal.Details["field"] != "resolution" {
			t.Fatalf("an unknown choice is refused by field: %+v", refusal)
		}
		p.phone.rejects(p.phone.call("conflict.resolve", engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: 7}), CodeValidation)
	})
	p.phone.resolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveReplace, Text: merged})
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("a replacement left %d conflicts", len(conflicts))
	}
	for _, h := range []*harness{p.phone, p.desktop} {
		if got := string(syncedFiles(t, h.dataDir)[path]); got != merged {
			t.Fatalf("a device holds %q, not the replacement", got)
		}
	}

	// Tasks take text too, checked the way the readers check it.
	q, _ := titleConflict(t)
	tasks := viewOn(t, q.phone.conflictViews(), atlasTasks)
	q.phone.unchanged("task text the readers refuse", func() {
		q.phone.rejects(q.phone.call("conflict.resolve", engine.ResolveRequest{ConflictID: tasks.ID, Choice: engine.ResolveReplace, Text: "## Task: no metadata\n"}), CodeValidation)
	})
	text := strings.Replace(*tasks.Versions[0].Text, "Ship beta build (phone)", "Ship beta build (agreed)", 1)
	text = strings.Replace(text, "Ship beta build (desktop)", "Ship beta build (agreed)", 1)
	q.phone.resolve(engine.ResolveRequest{ConflictID: tasks.ID, Choice: engine.ResolveReplace, Text: text})
	if conflicts := q.settle(); len(conflicts) != 0 {
		t.Fatalf("a task replacement left %d conflicts", len(conflicts))
	}
	requireTitle(t, q, task(atlasProject, shipBetaTask), "Ship beta build (agreed)")
}

func conflictCoupledChanges(t *testing.T) {
	p := newPair(t)
	ship, paint := task(atlasProject, shipBetaTask), task(homeProject, paintTask)
	p.phone.ok(p.phone.call("command.bulk", map[string]any{"tasks": []locator{ship, paint}, "action": "tags", "add_tags": []string{"phone"}}), nil)
	p.desktop.ok(p.desktop.call("command.bulk", map[string]any{"tasks": []locator{ship, paint}, "action": "tags", "add_tags": []string{"desk"}}), nil)
	conflicts := p.settle()
	if len(conflicts) < 2 {
		t.Fatalf("one bulk change on each side over two projects left %d conflicts", len(conflicts))
	}
	view := viewOn(t, p.phone.conflictViews(), atlasTasks)
	chosen := versionHolding(t, view, "phone")
	for _, version := range view.Versions {
		if len(version.AlsoResolves) == 0 {
			t.Fatalf("version %d does not name the conflicts its change also left: %+v", version.Index, version)
		}
	}
	result := p.phone.resolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: chosen})
	if len(result.Resolved) != len(conflicts) || result.Remaining != 0 {
		t.Fatalf("keeping one side of a two-project change resolved %+v, %d remain", result.Resolved, result.Remaining)
	}
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("the coupled resolution left %d conflicts", len(conflicts))
	}
	for _, h := range []*harness{p.phone, p.desktop} {
		for _, target := range []locator{ship, paint} {
			tags := h.detail(target.ProjectID, target.TaskID).Task.Tags
			if !slices.Contains(tags, "phone") || slices.Contains(tags, "desk") {
				t.Fatalf("%s carries %v after keeping the phone's change", target.TaskID, tags)
			}
		}
	}
}

func conflictDiagnostics(t *testing.T) {
	p, _ := titleConflict(t)
	p.phone.retitle(task(atlasProject, releaseNotesTask), "Unpublished phone edit")
	var report engine.SyncDiagnostics
	p.phone.ok(p.phone.call("sync.diagnostics", nil), &report)
	if report.Format != engine.SyncDiagnosticsFormat || report.Status == nil || report.DeviceID != "phone-device" {
		t.Fatalf("diagnostics identify %+v", report)
	}
	if len(report.Outbox) != 1 || len(report.Conflicts) != 1 || len(report.Conflicts[0].Versions) != 2 {
		t.Fatalf("diagnostics report outbox %v and conflicts %+v", report.Outbox, report.Conflicts)
	}
	for _, version := range report.Conflicts[0].Versions {
		if version.Digest == "" || len(version.Writers) == 0 {
			t.Fatalf("a diagnostic version names nothing: %+v", version)
		}
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, content := range []string{"Ship beta build", "Unpublished phone edit"} {
		if strings.Contains(string(data), content) && !strings.Contains(detailsText(report), content) {
			t.Fatalf("diagnostics carry file content %q", content)
		}
	}
	if strings.Contains(string(data), "## Task:") {
		t.Fatal("diagnostics carry task Markdown")
	}
	// Reading diagnostics changes nothing.
	p.phone.unchanged("a diagnostics read", func() { p.phone.ok(p.phone.call("sync.diagnostics", nil), nil) })
}

// detailsText is the merge's own description of what differs, which names the
// field values it judged; that much is the conflict's structure.
func detailsText(report engine.SyncDiagnostics) string {
	var out strings.Builder
	for _, conflict := range report.Conflicts {
		for _, detail := range conflict.Details {
			out.WriteString(detail.Subject)
			out.WriteString(detail.Field)
			out.WriteString(detail.Detail)
		}
	}
	return out.String()
}
