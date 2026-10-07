package bridge

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/syncexchange"
	"github.com/sglavoie/tt/internal/synclog"
)

// TestBridgeOrganizingConvergesAcrossDevices runs the organizing work on two
// devices that share one workspace through the exchange: the phone through
// the bridge, the desktop through the same application calls its keys make.
// Each scenario edits concurrently or in turn, then delivers every manifest
// both ways until nothing is pending. Both copies must then hold
// byte-identical synced files, read the same through the bridge, and keep
// every competing edit recoverable as a durable conflict rather than choosing
// one silently.
func TestBridgeOrganizingConvergesAcrossDevices(t *testing.T) {
	t.Run("concurrent_checklist_edits", syncConcurrentChecklistEdits)
	t.Run("recurring_check_in", syncRecurringCheckIn)
	t.Run("move_related_references", syncMoveRelatedReferences)
	t.Run("hierarchy_limit", syncHierarchyLimit)
	t.Run("blocked_parent_completion", syncBlockedParentCompletion)
	t.Run("destructive_confirmation", syncDestructiveConfirmation)
}

// phoneIDs mints identities no desktop sequence shares, so concurrent
// creations on the two devices never collide.
type phoneIDs struct {
	mu   sync.Mutex
	next int
}

func (g *phoneIDs) NewID() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next++
	return fmt.Sprintf("00000000-0000-4000-9000-%012d", g.next)
}

// pair is a desktop and a phone over two copies of the representative
// workspace, opened as two devices.
type pair struct {
	t       *testing.T
	desktop *harness
	phone   *harness
}

func newPair(t *testing.T) *pair {
	t.Helper()
	desktop := newHarness(t, representativeFixture)
	phone := newHarness(t, representativeFixture)
	phone.bridge = New(Options{Clock: phone.clock, IDs: &phoneIDs{}})
	desktop.openAs("desktop-device")
	phone.openAs("phone-device")
	return &pair{t: t, desktop: desktop, phone: phone}
}

// service is the desktop's application service, the one its keys call.
func (h *harness) service() *app.Service { return h.bridge.workspace.runtime.Service() }

// deliver publishes everything from has pending and imports it into to. It
// reports how many manifests traveled.
func deliver(t *testing.T, from, to *harness) int {
	t.Helper()
	exported := from.exportPending()
	if len(exported.Pending) == 0 {
		return 0
	}
	inbox := filepath.Join(t.TempDir(), "inbox")
	var published []string
	for _, outgoing := range exported.Pending {
		stage(t, inbox, outgoing.Manifest, nil)
		for _, blob := range outgoing.Blobs {
			stage(t, inbox, blob, nil)
		}
		published = append(published, outgoing.TxnID)
	}
	report := to.importInbox(inbox)
	for _, result := range report.Results {
		if result.Outcome != syncexchange.Imported && result.Outcome != syncexchange.Known {
			t.Fatalf("import of %s: %+v", result.Path, result)
		}
	}
	from.ok(from.call("sync.acknowledge", map[string]any{"txn_ids": published}), nil)
	return len(published)
}

// settle exchanges both ways until a full round moves nothing, then holds the
// two copies to identical synced bytes and identical open conflicts.
func (p *pair) settle() []synclog.Conflict {
	p.t.Helper()
	for round := 0; ; round++ {
		if round == 6 {
			p.t.Fatal("the devices never stopped exchanging")
		}
		if deliver(p.t, p.desktop, p.phone)+deliver(p.t, p.phone, p.desktop) == 0 {
			break
		}
	}
	desktopFiles, phoneFiles := syncedFiles(p.t, p.desktop.dataDir), syncedFiles(p.t, p.phone.dataDir)
	for path, data := range desktopFiles {
		if !bytes.Equal(data, phoneFiles[path]) {
			p.t.Fatalf("%s differs between the devices:\ndesktop:\n%s\nphone:\n%s", path, data, phoneFiles[path])
		}
	}
	for path := range phoneFiles {
		if _, ok := desktopFiles[path]; !ok {
			p.t.Fatalf("%s exists on the phone alone", path)
		}
	}
	desktopConflicts, phoneConflicts := p.desktop.conflicts(), p.phone.conflicts()
	if !slices.Equal(conflictIDs(desktopConflicts), conflictIDs(phoneConflicts)) {
		p.t.Fatalf("the devices disagree about open conflicts: %v and %v", conflictIDs(desktopConflicts), conflictIDs(phoneConflicts))
	}
	return phoneConflicts
}

func (h *harness) conflicts() []synclog.Conflict {
	h.t.Helper()
	conflicts, err := h.bridge.workspace.runtime.SyncCommits().SyncConflicts()
	if err != nil {
		h.t.Fatal(err)
	}
	return conflicts
}

func conflictIDs(conflicts []synclog.Conflict) []string {
	ids := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		ids = append(ids, conflict.ID)
	}
	slices.Sort(ids)
	return ids
}

// conflictOn is the one open conflict over a path, with every candidate's
// bytes recoverable from the device's history.
func conflictOn(t *testing.T, conflicts []synclog.Conflict, path string) synclog.Conflict {
	t.Helper()
	var found []synclog.Conflict
	for _, conflict := range conflicts {
		if conflict.Path == path {
			found = append(found, conflict)
		}
	}
	if len(found) != 1 {
		t.Fatalf("conflicts on %s: %+v (all %+v)", path, found, conflicts)
	}
	if len(found[0].Candidates) < 2 {
		t.Fatalf("a conflict names fewer than two candidates: %+v", found[0])
	}
	return found[0]
}

// syncedFiles is every exchanged workspace file: task Markdown, project and
// global notes, attachments, and the catalog, planning, and lens documents.
func syncedFiles(t *testing.T, dataDir string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	for _, name := range []string{"workspace.json", "planning.json", "lenses.json"} {
		if data, err := os.ReadFile(filepath.Join(dataDir, ".tt", name)); err == nil {
			files[".tt/"+name] = data
		}
	}
	for _, root := range []string{filepath.Join(dataDir, "projects"), filepath.Join(dataDir, ".tt", "global-notes")} {
		walkSyncedFiles(t, dataDir, root, files)
	}
	return files
}

func walkSyncedFiles(t *testing.T, dataDir, root string, files map[string][]byte) {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || strings.HasSuffix(path, ".bak") || strings.HasSuffix(path, ".lock") {
			return walkErr
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(dataDir, path)
		files[filepath.ToSlash(relative)] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// taskLine is one task's stored metadata line.
func taskLine(t *testing.T, dataDir, slug, taskID string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dataDir, "projects", slug, "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "<!-- id: "+taskID+",") {
			return line
		}
	}
	t.Fatalf("%s holds no task %s", slug, taskID)
	return ""
}

var checkInFields = regexp.MustCompile(`check_in: [0-9-]+, check_in_repeat: [0-9]+, reviewed: [^,]+`)

// syncConcurrentChecklistEdits: a checklist has no per-item identity, so two
// devices editing one checklist at once is a conflict both keep, while a
// checklist edit beside another field merges cleanly, and the phone's frozen
// selection refuses an index the desktop's work moved.
func syncConcurrentChecklistEdits(t *testing.T) {
	p := newPair(t)
	notes := task(atlasProject, releaseNotesTask)
	seen := p.phone.checklist(notes)

	// One field each: the phone ticks an item while the desktop retitles.
	p.phone.ok(p.phone.call("command.checklist", map[string]any{"task": notes, "action": "toggle", "index": 1, "checklist": seen}), nil)
	if _, err := p.desktop.service().EditTaskLine(app.TaskLineEditRequest{Source: notes.app(), Title: "Write the release notes", Action: "Edit Task"}); err != nil {
		t.Fatal(err)
	}
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("a checklist tick and a retitle conflicted: %+v", conflicts)
	}
	merged := p.phone.detail(atlasProject, releaseNotesTask).Task
	if merged.Title != "Write the release notes" || !merged.Checklist[1].Done {
		t.Fatalf("merged %q %+v", merged.Title, merged.Checklist)
	}

	// Both devices change the checklist: the desktop renames an item while the
	// phone adds one. Neither may be lost.
	seen = p.phone.checklist(notes)
	p.phone.ok(p.phone.call("command.checklist", map[string]any{"task": notes, "action": "add", "text": "Ship the notes"}), nil)
	if _, err := p.desktop.service().EditChecklistItem(notes.app(), 0, "Gather the full changelog"); err != nil {
		t.Fatal(err)
	}
	conflicts := p.settle()
	conflict := conflictOn(t, conflicts, "projects/atlas-launch/tasks.md")
	if !slices.ContainsFunc(conflict.Details, func(detail synclog.ConflictDetail) bool { return detail.Field == "checklist" }) {
		t.Fatalf("the conflict does not name the checklist: %+v", conflict.Details)
	}
	held := checklistTexts(p.phone.checklist(notes))
	if !slices.Equal(held, checklistTexts(p.desktop.checklist(notes))) {
		t.Fatalf("the devices read different checklists")
	}
	if !slices.Contains(held, "[ ] Ship the notes") && !slices.Contains(held, "[x] Gather the full changelog") {
		t.Fatalf("the merged checklist is neither side: %v", held)
	}

	// The phone's selection from before the exchange no longer matches, so an
	// index into it is refused rather than landing on another item.
	p.phone.unchanged("a stale checklist selection after an import", func() {
		p.phone.rejects(p.phone.call("command.checklist", map[string]any{"task": notes, "action": "remove", "index": 0, "checklist": seen}), CodeValidation)
	})
}

// syncRecurringCheckIn: a weekly check-in scheduled on the phone is
// acknowledged on the desktop and both read the schedule the acknowledgement
// moved on; two acknowledgements at once keep one whole schedule, never a
// date from one and a review from the other.
func syncRecurringCheckIn(t *testing.T) {
	p := newPair(t)
	signing := task(atlasProject, signingTask)
	p.phone.ok(p.phone.call("command.check_in", map[string]any{"tasks": []locator{signing}, "action": "set", "date": "2026-10-06", "repeat_days": 7}), nil)
	p.settle()
	p.desktop.clock.advance(48 * time.Hour)
	if _, err := p.desktop.service().MarkReviewed(signing.app()); err != nil {
		t.Fatal(err)
	}
	p.settle()
	read := p.phone.detail(atlasProject, signingTask).Task
	if want := localDay(p.desktop.clock.Now().AddDate(0, 0, 7)); read.NextCheckIn == nil || localDay(*read.NextCheckIn) != want {
		t.Fatalf("the phone reads check-in %v, want %s", read.NextCheckIn, want)
	}
	if read.State != "waiting" {
		t.Fatalf("acknowledging moved the state to %s", read.State)
	}

	// Both acknowledge, a day apart.
	p.phone.clock.advance(72 * time.Hour)
	p.desktop.clock.advance(48 * time.Hour)
	p.phone.ok(p.phone.call("command.check_in", map[string]any{"tasks": []locator{signing}, "action": "acknowledge"}), nil)
	phoneSchedule := checkInFields.FindString(taskLine(t, p.phone.dataDir, "atlas-launch", signingTask))
	if _, err := p.desktop.service().MarkReviewed(signing.app()); err != nil {
		t.Fatal(err)
	}
	desktopSchedule := checkInFields.FindString(taskLine(t, p.desktop.dataDir, "atlas-launch", signingTask))
	if phoneSchedule == "" || phoneSchedule == desktopSchedule {
		t.Fatalf("the two acknowledgements wrote %q and %q", phoneSchedule, desktopSchedule)
	}
	conflict := conflictOn(t, p.settle(), "projects/atlas-launch/tasks.md")
	if !slices.ContainsFunc(conflict.Details, func(detail synclog.ConflictDetail) bool { return detail.Field == "check_in" }) {
		t.Fatalf("the conflict does not name the check-in: %+v", conflict.Details)
	}
	held := checkInFields.FindString(taskLine(t, p.phone.dataDir, "atlas-launch", signingTask))
	if held != phoneSchedule && held != desktopSchedule {
		t.Fatalf("the merged schedule %q is a hybrid of %q and %q", held, phoneSchedule, desktopSchedule)
	}
}

// syncMoveRelatedReferences: the phone moves a task another project depends
// on while the desktop edits the dependent and points a second task at the
// old address. Both read every reference the same way: the rewritten one
// resolves at the new address, the one written against the old address
// degrades to its stored locator rather than vanishing.
func syncMoveRelatedReferences(t *testing.T) {
	p := newPair(t)
	signing := task(atlasProject, signingTask)
	var moved tasksResultView
	p.phone.ok(p.phone.call("command.bulk", map[string]any{"tasks": []locator{signing}, "action": "move", "project_id": readingProject}), &moved)
	newSigning := moved.Moves[0].To
	if _, err := p.desktop.service().EditTaskLine(app.TaskLineEditRequest{
		Source: task(homeProject, paintTask).app(), Title: "Pick paint colors", Action: "Edit Task",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.desktop.service().AddTaskDependencies(task(homeProject, tilesTask).app(), []app.TaskLocator{signing.app()}); err != nil {
		t.Fatal(err)
	}
	p.settle()
	for _, h := range []*harness{p.phone, p.desktop} {
		paint := h.detail(homeProject, paintTask)
		if paint.Task.Title != "Pick paint colors" {
			t.Fatalf("the dependent's edit was lost: %q", paint.Task.Title)
		}
		dependency := paint.Relations.Dependencies[0]
		if !dependency.Available || dependency.ProjectID != readingProject || dependency.TaskID != newSigning.TaskID {
			t.Fatalf("the moved reference reads %+v", dependency)
		}
		tiles := h.detail(homeProject, tilesTask).Relations.Dependencies
		if len(tiles) != 1 || tiles[0].ProjectID != atlasProject || tiles[0].TaskID != signingTask || tiles[0].Available {
			t.Fatalf("the reference to the old address reads %+v", tiles)
		}
	}
	if _, ok := syncedFiles(t, p.phone.dataDir)["projects/reading-list/tasks.md"]; !ok {
		t.Fatal("the destination project is missing")
	}
	if strings.Contains(taskLine(t, p.desktop.dataDir, "reading-list", newSigning.TaskID), "check_in") == false {
		t.Fatal("the moved task lost its own fields")
	}
}

// syncHierarchyLimit: the phone nests subtasks to the limit under a task the
// desktop concurrently moves one level deeper. Each was legal alone; together
// they would pass the limit, so the merge keeps a valid hierarchy and records
// the conflict, and both devices still read every task.
func syncHierarchyLimit(t *testing.T) {
	p := newPair(t)
	parent := task(atlasProject, proofreadTask)
	var created []locator
	for level := 3; level <= 6; level++ {
		var added hierarchyResult
		p.phone.ok(p.phone.call("command.hierarchy", map[string]any{"task": parent, "action": "add_child", "title": fmt.Sprintf("Level %d", level)}), &added)
		parent = task(added.Created.ProjectID, added.Created.TaskID)
		created = append(created, parent)
	}
	screenshots := screenshotsTask
	if _, err := p.desktop.service().Reparent(task(atlasProject, proofreadTask).app(), &screenshots); err != nil {
		t.Fatal(err)
	}
	conflict := conflictOn(t, p.settle(), "projects/atlas-launch/tasks.md")
	if !strings.Contains(fmt.Sprint(conflict.Details), "hierarchy") {
		t.Fatalf("the conflict does not name the hierarchy: %+v", conflict.Details)
	}
	for _, h := range []*harness{p.phone, p.desktop} {
		for _, target := range created {
			read := h.detail(target.ProjectID, target.TaskID)
			if level := len(read.Relations.Ancestors); level > 6 {
				t.Fatalf("%s sits at level %d", read.Task.Title, level)
			}
		}
	}
}

// syncBlockedParentCompletion: the phone finishes a parent's only subtask and
// then the parent while the desktop adds a new subtask under it. Both devices
// keep the new subtask, and once the parent is reopened both refuse to finish
// it again over that subtask, naming it.
func syncBlockedParentCompletion(t *testing.T) {
	p := newPair(t)
	icons := task(atlasProject, darkIconsTask)
	var added hierarchyResult
	p.phone.ok(p.phone.call("command.hierarchy", map[string]any{"task": icons, "action": "add_child", "title": "Draw the moon"}), &added)
	p.settle()
	p.phone.ok(p.phone.call("command.transition", map[string]any{"task": task(added.Created.ProjectID, added.Created.TaskID), "state": "done"}), nil)
	p.phone.ok(p.phone.call("command.transition", map[string]any{"task": icons, "state": "done"}), nil)
	if _, err := p.desktop.service().ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationCreateChild, Targets: []app.TaskLocator{icons.app()}, Text: "Draw the sun",
	}); err != nil {
		t.Fatal(err)
	}
	p.settle()
	for _, h := range []*harness{p.phone, p.desktop} {
		children := relationTitles(h.detail(atlasProject, darkIconsTask).Relations.Children)
		if !slices.Equal(children, []string{"Draw the moon", "Draw the sun"}) {
			t.Fatalf("children %v", children)
		}
	}
	p.phone.ok(p.phone.call("command.transition", map[string]any{"task": icons, "state": "inbox"}), nil)
	p.settle()
	for _, h := range []*harness{p.phone, p.desktop} {
		h.unchanged("a completion over the other device's subtask", func() {
			refusal := h.refused(h.call("command.transition", map[string]any{"task": icons, "state": "done"}), CodeValidation)
			if refusal.Details["kind"] != "incomplete_children" || !strings.Contains(fmt.Sprint(refusal.Details["children"]), "Draw the sun") {
				t.Fatalf("refusal %+v", refusal)
			}
		})
	}
}

// syncDestructiveConfirmation: the phone confirms deleting a project, the
// desktop adds work to it before the phone's delete runs, and the delivered
// work makes the confirmation stale, so nothing is deleted until the person
// confirms what the project now holds. The delete then reaches the desktop.
func syncDestructiveConfirmation(t *testing.T) {
	p := newPair(t)
	var shown projectDetail
	p.phone.ok(p.phone.call("query.project", map[string]any{"project_id": homeProject}), &shown)
	if _, err := p.desktop.service().Capture(app.CaptureRequest{ProjectID: homeProject, Title: "Seal the grout"}); err != nil {
		t.Fatal(err)
	}
	p.settle()
	p.phone.unchanged("a delete confirmed before the other device's work arrived", func() {
		p.phone.rejects(p.phone.call("command.project", map[string]any{"action": "delete", "project_id": homeProject, "confirm": shown.Impact}), CodeStaleRevision)
	})
	var current projectDetail
	p.phone.ok(p.phone.call("query.project", map[string]any{"project_id": homeProject}), &current)
	if current.Impact.Tasks != shown.Impact.Tasks+1 {
		t.Fatalf("impact %+v after %+v", current.Impact, shown.Impact)
	}
	p.phone.ok(p.phone.call("command.project", map[string]any{"action": "delete", "project_id": homeProject, "confirm": current.Impact}), nil)
	if conflicts := p.settle(); len(conflicts) != 0 {
		t.Fatalf("an uncontested delete conflicted: %+v", conflicts)
	}
	for _, h := range []*harness{p.phone, p.desktop} {
		h.rejects(h.call("query.project", map[string]any{"project_id": homeProject}), CodeNotFound)
		for path := range syncedFiles(t, h.dataDir) {
			if strings.HasPrefix(path, "projects/home-renovation/") {
				t.Fatalf("the deleted project's %s remains on %s", path, h.dataDir)
			}
		}
	}
}
