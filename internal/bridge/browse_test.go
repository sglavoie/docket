package bridge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/syncexchange"
)

// Fixture identities the browsing tests navigate by.
const (
	atlasProject       = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01"
	homeProject        = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a02"
	shipBetaTask       = "0a7e5d10-2c3b-4f6a-8e9d-000000000101"
	releaseNotesTask   = "0a7e5d10-2c3b-4f6a-8e9d-000000000102"
	proofreadTask      = "0a7e5d10-2c3b-4f6a-8e9d-000000000103"
	screenshotsTask    = "0a7e5d10-2c3b-4f6a-8e9d-000000000104"
	signingTask        = "0a7e5d10-2c3b-4f6a-8e9d-000000000105"
	paintTask          = "0a7e5d10-2c3b-4f6a-8e9d-000000000201"
	atlasReleaseLens   = "9e8d7c6b-5a49-4382-a1b0-c9d8e7f6a501"
	todayFocusLens     = "9e8d7c6b-5a49-4382-a1b0-c9d8e7f6a502"
	signingTaskHeading = "## Task: Await signing certificate\n"
)

func (h *harness) titles(payload map[string]any) []string {
	h.t.Helper()
	var result tasksResult
	h.ok(h.call("query.tasks", payload), &result)
	titles := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		titles = append(titles, item.Task.Title)
	}
	return titles
}

func (h *harness) detail(projectID, taskID string) taskResult {
	h.t.Helper()
	var result taskResult
	h.ok(h.call("query.task", map[string]any{"task": map[string]string{"project_id": projectID, "task_id": taskID}}), &result)
	return result
}

func relationTitles(views []relationView) []string {
	titles := make([]string, 0, len(views))
	for _, view := range views {
		titles = append(titles, view.Title)
	}
	return titles
}

func requireTitles(t *testing.T, label string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s: got %q, want %q", label, got, want)
	}
}

func TestBridgeSearchesTagsFromMobile(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)
	var captured captureResult
	h.ok(h.call("command.capture", map[string]any{"text": "Plan the launch #epic #source:github ##Home.Office"}), &captured)
	for _, search := range []string{"#epic", "#Epic", "#EPIC", "#Epic launch", "#EP*", "#Source:GitHub", "##Home.Office", "#Epic -#Release", "#Epic -##Laptop"} {
		t.Run(search, func(t *testing.T) {
			requireTitles(t, search, h.titles(map[string]any{"mode": "all", "search": search}), []string{"Plan the launch"})
		})
	}
	for _, search := range []string{"#epic -#Epic", "#epic -##Home.Office", `"#Epic"`, "#Epic unrelated"} {
		t.Run(search, func(t *testing.T) {
			requireTitles(t, search, h.titles(map[string]any{"mode": "all", "search": search}), nil)
		})
	}
}

// TestBridgeBrowsesASeededWorkspace walks the representative workspace the way
// the iPhone's Today, Inbox, and Browse destinations do: every mode, saved
// lenses, the search box, sorting, and the hierarchy and dependency relations a
// task detail navigates.
func TestBridgeBrowsesASeededWorkspace(t *testing.T) {
	h := newHarness(t, representativeFixture)
	h.open(false)

	t.Run("modes", func(t *testing.T) {
		requireTitles(t, "today", h.titles(map[string]any{"mode": "today"}), []string{"Ship beta build", "Order bathroom tiles"})
		requireTitles(t, "next", h.titles(map[string]any{"mode": "next"}), []string{"Pick paint colors"})
		requireTitles(t, "inbox", h.titles(map[string]any{"mode": "inbox"}), []string{"Proofread release notes", "Call the bank"})
		for _, mode := range []string{"all", "backlog", "due", "recent"} {
			var result tasksResult
			h.ok(h.call("query.tasks", map[string]any{"mode": mode}), &result)
		}
		requireTitles(t, "due", h.titles(map[string]any{"mode": "due"}), []string{"Ship beta build", "Await signing certificate"})
	})
	t.Run("sort", func(t *testing.T) {
		got := h.titles(map[string]any{"mode": "all", "scope": atlasProject, "sort": map[string]any{"field": "title", "ascending": true}})
		if !slices.IsSorted(got) || len(got) < 4 {
			t.Fatalf("title sort is not ascending: %q", got)
		}
	})
	t.Run("lenses_listed", func(t *testing.T) {
		var lenses lensesResult
		h.ok(h.call("query.lenses", nil), &lenses)
		if len(lenses.Lenses) != 2 || lenses.Lenses[0].ID != atlasReleaseLens || lenses.Lenses[1].ID != todayFocusLens {
			t.Fatalf("lenses = %+v", lenses.Lenses)
		}
		if first := lenses.Lenses[0]; first.Name != "Atlas release" || first.Scope != atlasProject || first.Query != "#release" || first.Sort.Field != "due" {
			t.Fatalf("Atlas release lens = %+v", first)
		}
		var raw struct {
			Lenses []struct {
				Sort map[string]any `json:"sort"`
			} `json:"lenses"`
		}
		h.ok(h.call("query.lenses", nil), &raw)
		if _, spelled := raw.Lenses[1].Sort["ascending"]; !spelled {
			t.Fatalf("a descending lens's sort omits its direction: %+v", raw.Lenses[1].Sort)
		}
	})
	t.Run("lens_reopens_its_query", func(t *testing.T) {
		requireTitles(t, "atlas release", h.titles(map[string]any{"lens": atlasReleaseLens}), []string{"Ship beta build"})
		requireTitles(t, "today focus", h.titles(map[string]any{"lens": todayFocusLens}), h.titles(map[string]any{"mode": "today"}))
	})
	t.Run("lens_narrowed_by_search", func(t *testing.T) {
		requireTitles(t, "today focus + tiles", h.titles(map[string]any{"lens": todayFocusLens, "search": "tiles"}), []string{"Order bathroom tiles"})
	})
	t.Run("lens_owns_its_axes", func(t *testing.T) {
		h.rejects(h.call("query.tasks", map[string]any{"lens": atlasReleaseLens, "mode": "all"}), CodeInvalidRequest)
		h.rejects(h.call("query.tasks", map[string]any{"lens": "no-such-lens"}), CodeNotFound)
	})
	t.Run("search_text", func(t *testing.T) {
		requireTitles(t, "paint", h.titles(map[string]any{"mode": "all", "search": "PAINT"}), []string{"Pick paint colors"})
	})
	t.Run("search_sigils", func(t *testing.T) {
		requireTitles(t, "$waiting", h.titles(map[string]any{"mode": "all", "search": "$waiting"}), []string{"Await signing certificate"})
		requireTitles(t, "#release ship", h.titles(map[string]any{"mode": "all", "search": "#release ship"}), []string{"Ship beta build"})
	})
	t.Run("search_refuses_by_name", func(t *testing.T) {
		refusal := h.refused(h.call("query.tasks", map[string]any{"mode": "all", "search": "$ready"}), CodeValidation)
		if !strings.Contains(refusal.Message, "ready") {
			t.Fatalf("refusal does not name the value: %s", refusal.Message)
		}
	})
	t.Run("hierarchy", func(t *testing.T) {
		proofread := h.detail(atlasProject, proofreadTask)
		requireTitles(t, "ancestors", relationTitles(proofread.Relations.Ancestors), []string{"Ship beta build", "Write release notes"})
		notes := h.detail(atlasProject, releaseNotesTask)
		requireTitles(t, "children", relationTitles(notes.Relations.Children), []string{"Proofread release notes", "Collect screenshots"})
		if len(notes.Task.Checklist) != 2 || !notes.Task.Checklist[0].Done || notes.Task.Checklist[1].Text != "Draft summary" {
			t.Fatalf("checklist = %+v", notes.Task.Checklist)
		}
		ship := h.detail(atlasProject, shipBetaTask)
		if ship.Task.ResumeNote == "" || ship.Task.Comments != 2 || ship.Task.Attachments != 3 {
			t.Fatalf("detail fields = resume %q comments %d attachments %d", ship.Task.ResumeNote, ship.Task.Comments, ship.Task.Attachments)
		}
	})
	t.Run("dependencies", func(t *testing.T) {
		ship := h.detail(atlasProject, shipBetaTask)
		requireTitles(t, "ship depends on", relationTitles(ship.Relations.Dependencies), []string{"Pick paint colors"})
		paint := h.detail(homeProject, paintTask)
		requireTitles(t, "paint depends on", relationTitles(paint.Relations.Dependencies), []string{"Await signing certificate"})
		requireTitles(t, "paint dependents", relationTitles(paint.Relations.Dependents), []string{"Ship beta build"})
		if dep := paint.Relations.Dependents[0]; dep.ProjectID != atlasProject || dep.TaskID != shipBetaTask || !dep.Available || dep.ProjectName != "Atlas Launch" {
			t.Fatalf("dependent locator = %+v", dep)
		}
	})
}

// TestBridgeNamesAnUnavailableDependency keeps a reference whose target is gone
// as its stored locator, marked unavailable, rather than dropping it.
func TestBridgeNamesAnUnavailableDependency(t *testing.T) {
	h := newHarness(t, representativeFixture)
	tasks := filepath.Join(h.dataDir, "projects", "atlas-launch", "tasks.md")
	data, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, signingTaskHeading)
	end := strings.Index(text[start+len(signingTaskHeading):], "## Task:")
	if start < 0 || end < 0 {
		t.Fatalf("fixture no longer holds %q", signingTaskHeading)
	}
	text = text[:start] + text[start+len(signingTaskHeading)+end:]
	if err := os.WriteFile(tasks, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	h.open(false)
	paint := h.detail(homeProject, paintTask)
	if len(paint.Relations.Dependencies) != 1 {
		t.Fatalf("dependencies = %+v", paint.Relations.Dependencies)
	}
	dependency := paint.Relations.Dependencies[0]
	if dependency.Available || dependency.ProjectID != atlasProject || dependency.TaskID != signingTask || dependency.Title != "" {
		t.Fatalf("unavailable dependency = %+v", dependency)
	}
	h.rejects(h.call("query.task", map[string]any{"task": map[string]string{"project_id": atlasProject, "task_id": signingTask}}), CodeNotFound)
}

// TestBridgeJoinsFromAStagedInbox is the iPhone's onboarding join: a desktop
// enrolls the representative workspace and publishes it, the transport stages
// the exchange into an inbox, and workspace.join builds and opens a working
// copy from that history alone.
func TestBridgeJoinsFromAStagedInbox(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "desk-home")
	t.Setenv("HOME", home)
	original := filepath.Join(root, "original")
	copyTree(t, representativeFixture, original)
	exchange := filepath.Join(root, "cloud", "exchange")
	if err := os.MkdirAll(exchange, 0o700); err != nil {
		t.Fatal(err)
	}
	clock := &fixedClock{now: time.Date(2026, time.October, 6, 9, 0, 0, 0, time.UTC)}
	if _, err := engine.Enroll(engine.EnrollConfig{
		Original:     original,
		WorkingCopy:  filepath.Join(root, "local", "desk-copy"),
		Exchange:     exchange,
		SnapshotRoot: filepath.Join(root, "snapshots"),
		Machine:      engine.ExplicitMachine(home, "desk-a"),
		Clock:        clock,
		AppVersion:   "test",
	}); err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	workspaceID, err := syncexchange.ReadExchangeIdentity(exchange)
	if err != nil {
		t.Fatal(err)
	}
	published := filepath.Join(exchange, filepath.FromSlash(syncexchange.ExchangeRoot))
	inbox := filepath.Join(root, "phone", "inbox")
	for _, dir := range []string{"txns", "blobs"} {
		copyTree(t, filepath.Join(published, dir), filepath.Join(inbox, dir))
	}

	h := &harness{
		t:          t,
		bridge:     New(Options{Clock: clock, IDs: &sequentialIDs{}}),
		clock:      clock,
		dataDir:    filepath.Join(root, "phone", "workspace"),
		runtimeDir: filepath.Join(root, "phone", "device"),
	}
	join := func(device, dataDir, inboxDir string) response {
		return h.call("workspace.join", map[string]any{
			"data_dir": dataDir, "runtime_dir": h.runtimeDir, "device_id": device,
			"workspace_id": workspaceID, "exchange": exchange, "inbox_dir": inboxDir,
		})
	}

	t.Run("publishing_device_is_refused", func(t *testing.T) {
		h.rejects(join("desk-a", filepath.Join(root, "phone", "refused"), inbox), CodeRefused)
	})
	t.Run("empty_inbox_delivers_nothing", func(t *testing.T) {
		empty := filepath.Join(root, "phone", "empty-inbox")
		if err := os.MkdirAll(empty, 0o700); err != nil {
			t.Fatal(err)
		}
		h.rejects(join("phone-a", filepath.Join(root, "phone", "undelivered"), empty), CodeRefused)
		h.rejects(h.call("workspace.status", nil, func(e map[string]any) { e["workspace"] = workspaceID }), CodeNotOpen)
	})
	t.Run("joined_copy_browses", func(t *testing.T) {
		var joined joinResult
		h.ok(join("phone-a", h.dataDir, inbox), &joined)
		if joined.Workspace != workspaceID || joined.DeviceID != "phone-a" || joined.Schemas.Catalog != config.SyncEnrolledConfigVersion {
			t.Fatalf("joined status = %+v", joined.statusResult)
		}
		if joined.Import == nil || len(joined.Import.Results) == 0 {
			t.Fatalf("join imported nothing: %+v", joined.Import)
		}
		for _, result := range joined.Import.Results {
			if result.Outcome != syncexchange.Imported {
				t.Fatalf("staged manifest %s: %s (%s)", result.Path, result.Outcome, result.Reason)
			}
		}
		h.workspace = joined.Workspace
		requireTitles(t, "joined today", h.titles(map[string]any{"mode": "today"}), []string{"Ship beta build", "Order bathroom tiles"})
		status := h.syncStatus()
		if status.DeviceID != "phone-a" || len(status.Heads) == 0 {
			t.Fatalf("joined sync status = %+v", status)
		}
	})
	t.Run("joined_copy_is_not_joined_twice", func(t *testing.T) {
		h.rejects(join("phone-a", h.dataDir, inbox), CodeRefused)
	})
}
