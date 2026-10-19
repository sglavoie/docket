package bridge

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/syncexchange"
)

func conflictStaleResolution(t *testing.T) {
	p, ship := titleConflict(t)
	read := viewOn(t, p.phone.conflictViews(), atlasTasks)
	// An edit after reading changes what the conflict holds.
	p.phone.ok(p.phone.call("command.edit", map[string]any{"task": task(atlasProject, releaseNotesTask), "title": "Write release notes today"}), nil)
	var refusal *Error
	p.phone.unchanged("a resolution read before an edit", func() {
		refusal = p.phone.refused(p.phone.call("conflict.resolve", engine.ResolveRequest{ConflictID: read.ID, Choice: engine.ResolveVersion, Version: 0}), CodeStaleRevision)
	})
	open, _ := refusal.Details["conflicts"].([]any)
	now := viewOn(t, p.phone.conflictViews(), atlasTasks)
	if len(open) != 1 || open[0] != now.ID || now.ID == read.ID {
		t.Fatalf("the refusal names %v, the conflict open now is %s", open, now.ID)
	}
	// Both versions and the edit are still there to choose from again.
	for _, marker := range []string{"(phone)", "(desktop)"} {
		versionHolding(t, now, marker)
	}
	if !strings.Contains(*now.Current.Text, "Write release notes today") {
		t.Fatal("the edit made after reading is gone")
	}
	p.phone.resolve(engine.ResolveRequest{ConflictID: now.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, now, "(phone)")})
	p.settle()
	requireTitle(t, p, ship, "Ship beta build (phone)")
	requireTitle(t, p, task(atlasProject, releaseNotesTask), "Write release notes today")

	// The desktop's engine refuses the same way, by sentinel.
	q, _ := titleConflict(t)
	old := viewOn(t, q.desktop.conflictViews(), atlasTasks)
	q.desktop.desktopResolve(engine.ResolveRequest{ConflictID: old.ID, Choice: engine.ResolveCurrent})
	if _, err := q.desktop.bridge.workspace.runtime.ResolveSyncConflict(engine.ResolveRequest{ConflictID: old.ID, Choice: engine.ResolveCurrent}); !errors.Is(err, engine.ErrConflictStale) {
		t.Fatalf("resolving a closed conflict again: %v", err)
	}
}

func conflictResolutionRacesEdit(t *testing.T) {
	// The same task: the phone resolves while the desktop, not yet told,
	// retitles again. Neither is dropped: a new conflict holds both.
	p, ship := titleConflict(t)
	view := viewOn(t, p.phone.conflictViews(), atlasTasks)
	p.phone.resolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, view, "(phone)")})
	p.desktop.retitle(ship, "Ship beta build (desktop, again)")
	conflicts := p.settle()
	if len(conflicts) != 1 {
		t.Fatalf("a resolution racing an edit left %d conflicts", len(conflicts))
	}
	for _, h := range []*harness{p.phone, p.desktop} {
		for _, marker := range []string{"Ship beta build (phone)", "Ship beta build (desktop, again)"} {
			if !h.recoverable(conflicts, atlasTasks, containing("## Task: "+marker+"\n")) {
				t.Fatalf("%q is lost on one device", marker)
			}
		}
	}
	again := viewOn(t, p.desktop.conflictViews(), atlasTasks)
	kinds := map[string]bool{}
	for _, version := range again.Versions {
		for _, writer := range version.Writers {
			kinds[writer.Change] = true
		}
	}
	if !kinds["conflict resolution"] || !kinds["edit"] {
		t.Fatalf("the new conflict does not name the racing resolution and edit: %v", kinds)
	}
	p.desktop.desktopResolve(engine.ResolveRequest{ConflictID: again.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, again, "(desktop, again)")})
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("%d conflicts remain", len(conflicts))
	}
	requireTitle(t, p, ship, "Ship beta build (desktop, again)")

	// Another task in the same file: the resolution and the edit both hold,
	// with nothing left to resolve.
	q, ship := titleConflict(t)
	view = viewOn(t, q.phone.conflictViews(), atlasTasks)
	q.phone.resolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, view, "(desktop)")})
	q.desktop.retitle(task(atlasProject, releaseNotesTask), "Write release notes (desktop)")
	if conflicts := q.settle(); len(conflicts) != 0 {
		t.Fatalf("a resolution beside an unrelated edit left %d conflicts", len(conflicts))
	}
	requireTitle(t, q, ship, "Ship beta build (desktop)")
	requireTitle(t, q, task(atlasProject, releaseNotesTask), "Write release notes (desktop)")
}

func conflictCompetingResolutions(t *testing.T) {
	p, ship := titleConflict(t)
	phoneView := viewOn(t, p.phone.conflictViews(), atlasTasks)
	deskView := viewOn(t, p.desktop.conflictViews(), atlasTasks)
	if phoneView.ID != deskView.ID {
		t.Fatal("the devices name the conflict differently")
	}
	// Both devices write their own resolution before either hears of the
	// other's: a new conflict keeps both.
	agreed := func(view engine.ConflictView, title string) string {
		text := *view.Current.Text
		for _, marker := range []string{"Ship beta build (phone)", "Ship beta build (desktop)"} {
			text = strings.Replace(text, "## Task: "+marker+"\n", "## Task: "+title+"\n", 1)
		}
		return text
	}
	p.phone.resolve(engine.ResolveRequest{ConflictID: phoneView.ID, Choice: engine.ResolveReplace, Text: agreed(phoneView, "Ship beta (phone's call)")})
	p.desktop.desktopResolve(engine.ResolveRequest{ConflictID: deskView.ID, Choice: engine.ResolveReplace, Text: agreed(deskView, "Ship beta (desktop's call)")})
	conflicts := p.settle()
	if len(conflicts) != 1 {
		t.Fatalf("two disagreeing resolutions left %d conflicts", len(conflicts))
	}
	view := viewOn(t, p.phone.conflictViews(), atlasTasks)
	for _, h := range []*harness{p.phone, p.desktop} {
		for _, marker := range []string{"Ship beta (phone's call)", "Ship beta (desktop's call)"} {
			if !h.recoverable(conflicts, atlasTasks, containing("## Task: "+marker+"\n")) {
				t.Fatalf("%q is lost on one device", marker)
			}
		}
	}
	for _, version := range view.Versions {
		if len(version.Writers) != 1 || version.Writers[0].Change != "conflict resolution" {
			t.Fatalf("the new conflict's versions are not the two resolutions: %+v", version.Writers)
		}
	}
	p.phone.resolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, view, "(desktop's call)")})
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("%d conflicts remain", len(conflicts))
	}
	requireTitle(t, p, ship, "Ship beta (desktop's call)")

	// A resolution keeping what the merge already held changes nothing, so
	// the other device's choice stands everywhere without a new conflict.
	q, ship := titleConflict(t)
	held := viewOn(t, q.phone.conflictViews(), atlasTasks)
	var kept, other string
	for _, version := range held.Versions {
		marker := "Ship beta build (phone)"
		if strings.Contains(*version.Text, "## Task: Ship beta build (desktop)\n") {
			marker = "Ship beta build (desktop)"
		}
		if version.Current {
			kept = marker
		} else {
			other = marker
		}
	}
	q.phone.resolve(engine.ResolveRequest{ConflictID: held.ID, Choice: engine.ResolveCurrent})
	q.desktop.desktopResolve(engine.ResolveRequest{ConflictID: held.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, held, strings.TrimPrefix(other, "Ship beta build "))})
	if conflicts := q.settle(); len(conflicts) != 0 {
		t.Fatalf("a resolution agreeing with the merge left %d conflicts", len(conflicts))
	}
	if kept == other {
		t.Fatal("both versions read as the current one")
	}
	requireTitle(t, q, ship, other)
}

func conflictTransportFailure(t *testing.T) {
	p := newPair(t)
	ship := task(atlasProject, shipBetaTask)
	p.phone.retitle(ship, "Ship beta build (phone)")
	p.desktop.retitle(ship, "Ship beta build (desktop)")
	p.desktop.ok(p.desktop.call("command.capture", map[string]any{"title": "Desk-only follow-up", "project_id": atlasProject}), nil)

	// The transport delivers the desktop's manifests but none of their
	// content: nothing applies, and nothing is lost.
	exported := p.desktop.exportPending()
	inbox := filepath.Join(t.TempDir(), "inbox")
	for _, outgoing := range exported.Pending {
		stage(t, inbox, outgoing.Manifest, nil)
	}
	p.phone.unchanged("an import whose content never arrived", func() {
		for _, result := range p.phone.importInbox(inbox).Results {
			if result.Outcome != syncexchange.Incomplete || len(result.Missing) == 0 {
				t.Fatalf("a manifest without its blobs reports %+v", result)
			}
		}
	})
	// Local content stays readable and editable meanwhile.
	if got := p.phone.detail(atlasProject, shipBetaTask).Task.Title; got != "Ship beta build (phone)" {
		t.Fatalf("the phone reads %q while the transport fails", got)
	}
	p.phone.ok(p.phone.call("command.capture", map[string]any{"title": "Captured while offline", "project_id": atlasProject}), nil)
	if !p.phone.hasTask("Captured while offline") || p.phone.hasTask("Desk-only follow-up") {
		t.Fatal("the phone's local work is not what it reads")
	}
	// A retry with the content applies everything, and the conflict that
	// then exists reads from local history even with the transport down.
	for _, outgoing := range exported.Pending {
		for _, blob := range outgoing.Blobs {
			stage(t, inbox, blob, nil)
		}
	}
	for _, result := range p.phone.importInbox(inbox).Results {
		if result.Outcome != syncexchange.Imported && result.Outcome != syncexchange.Known {
			t.Fatalf("the retry reports %+v", result)
		}
	}
	view := viewOn(t, p.phone.conflictViews(), atlasTasks)
	for _, version := range view.Versions {
		if !version.Available || version.Text == nil {
			t.Fatalf("a competing version is not readable locally: %+v", version)
		}
	}
	p.phone.resolve(engine.ResolveRequest{ConflictID: view.ID, Choice: engine.ResolveVersion, Version: versionHolding(t, view, "(phone)")})
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("%d conflicts remain after the retry", len(conflicts))
	}
	for _, h := range []*harness{p.phone, p.desktop} {
		if !h.hasTask("Captured while offline") || !h.hasTask("Desk-only follow-up") {
			t.Fatal("work from either side of the failure is missing")
		}
	}
	requireTitle(t, p, ship, "Ship beta build (phone)")
}
