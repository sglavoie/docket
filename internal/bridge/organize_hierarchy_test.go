package bridge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// organizeHierarchyLimit nests subtasks until the engine's depth limit refuses
// one, then refuses an indent that would carry a subtree past it.
func organizeHierarchyLimit(t *testing.T, h *harness) {
	parent := task(atlasProject, proofreadTask) // level 2: Ship beta build is level 0
	depth := 2
	for {
		reply := h.call("command.hierarchy", map[string]any{"task": parent, "action": "add_child", "title": "Level"})
		if !reply.OK {
			refusal := h.refused(reply, CodeValidation)
			if refusal.Details["kind"] != "invalid_input" || !strings.Contains(refusal.Message, "depth") {
				t.Fatalf("refusal %+v", refusal)
			}
			break
		}
		var added hierarchyResult
		h.ok(reply, &added)
		parent, depth = task(added.Created.ProjectID, added.Created.TaskID), depth+1
		if depth > 10 {
			t.Fatal("no depth limit refused a subtask")
		}
	}
	if depth != 6 {
		t.Fatalf("nesting stopped at level %d, want %d", depth, 6)
	}
	// Screenshots sits at level 2 beside Proofread, whose chain is now full:
	// indenting it under Proofread with a child of its own would pass the limit.
	screenshots := task(atlasProject, screenshotsTask)
	h.ok(h.call("command.hierarchy", map[string]any{"task": screenshots, "action": "add_child", "title": "Crop"}), nil)
	h.ok(h.call("command.hierarchy", map[string]any{"task": screenshots, "action": "add_child", "title": "Annotate"}), nil)
	crop := h.childNamed(screenshots, "Crop")
	h.ok(h.call("command.hierarchy", map[string]any{"task": crop, "action": "add_child", "title": "Crop again"}), nil)
	h.unchanged("an indent past the limit", func() {
		h.rejects(h.call("command.hierarchy", map[string]any{"task": screenshots, "action": "set_parent", "parent": parent}), CodeValidation)
	})
}

// organizeBlockedParentCompletion refuses to finish a parent over open
// subtasks, alone or inside a selection, and names the subtasks.
func organizeBlockedParentCompletion(t *testing.T, h *harness) {
	icons := task(atlasProject, darkIconsTask)
	h.unchanged("a blocked completion", func() {
		refusal := h.refused(h.call("command.transition", map[string]any{"task": icons, "state": "done"}), CodeValidation)
		if refusal.Details["kind"] != "incomplete_children" {
			t.Fatalf("refusal %+v", refusal)
		}
	})
	h.unchanged("a blocked bulk completion", func() {
		refusal := h.refused(h.call("command.bulk", map[string]any{
			"tasks": []locator{task(atlasProject, localizeTask), icons}, "action": "state", "state": "done",
		}), CodeValidation)
		if refusal.Details["kind"] != "incomplete_children" {
			t.Fatalf("refusal %+v", refusal)
		}
	})
	// Finishing the subtasks first lets the parent finish.
	var children []locator
	for _, child := range h.detail(atlasProject, darkIconsTask).Relations.Children {
		children = append(children, task(child.ProjectID, child.TaskID))
	}
	h.ok(h.call("command.bulk", map[string]any{"tasks": children, "action": "state", "state": "done"}), nil)
	h.ok(h.call("command.transition", map[string]any{"task": icons, "state": "done"}), nil)
}

// organizeMoveKeepsReferences moves a task another project depends on: the
// dependent's reference follows the new address, one move at a time or as a
// selection, and planning follows too.
func organizeMoveKeepsReferences(t *testing.T, h *harness) {
	signing := task(atlasProject, signingTask)
	var moved tasksResultView
	h.ok(h.call("command.bulk", map[string]any{"tasks": []locator{signing}, "action": "move", "project_id": readingProject}), &moved)
	if len(moved.Moves) != 1 || moved.Moves[0].From != signing || moved.Moves[0].To.ProjectID != readingProject {
		t.Fatalf("moves %+v", moved.Moves)
	}
	newSigning := moved.Moves[0].To
	paint := h.detail(homeProject, paintTask)
	if len(paint.Relations.Dependencies) != 1 || !paint.Relations.Dependencies[0].Available ||
		paint.Relations.Dependencies[0].ProjectID != readingProject || paint.Relations.Dependencies[0].TaskID != newSigning.TaskID {
		t.Fatalf("the dependent's reference did not follow the move: %+v", paint.Relations.Dependencies)
	}
	if moved.Committed.Projects[atlasProject] == "" || moved.Committed.Projects[readingProject] == "" || moved.Committed.Projects[homeProject] == "" {
		t.Fatalf("a move reports every project it rewrote: %+v", moved.Committed.Projects)
	}
	// A selection moves through the batch, subtrees and references alike.
	var batch tasksResultView
	h.ok(h.call("command.bulk", map[string]any{
		"tasks": []locator{newSigning, task(homeProject, tilesTask)}, "action": "move", "project_id": atlasProject,
	}), &batch)
	if len(batch.Moves) != 2 {
		t.Fatalf("batch moves %+v", batch.Moves)
	}
	dependency := h.detail(homeProject, paintTask).Relations.Dependencies[0]
	if dependency.ProjectID != atlasProject || !dependency.Available {
		t.Fatalf("reference after the batch move %+v", dependency)
	}
	h.unchanged("a move into an archived project", func() {
		h.rejects(h.call("command.bulk", map[string]any{
			"tasks": []locator{task(homeProject, paintTask)}, "action": "move", "project_id": oldWebsiteProject,
		}), CodeNotFound)
	})
}

func organizeBulkActions(t *testing.T, h *harness) {
	unfiled := h.unfiled()
	var captured captureResult
	h.ok(h.call("command.capture", map[string]any{"project_id": unfiled.ID, "text": "Fold laundry\nWater plants\nCall mum"}), &captured)
	var selection []locator
	for _, view := range captured.Tasks {
		selection = append(selection, task(view.ProjectID, view.TaskID))
	}
	var planned tasksResultView
	h.ok(h.call("command.bulk", map[string]any{"tasks": selection, "action": "plan", "plan": "add_next"}), &planned)
	if planned.Committed.Domains["planning"] == "" || len(planned.Tasks) != 3 {
		t.Fatalf("plan %+v", planned)
	}
	for _, target := range selection {
		if !h.detail(target.ProjectID, target.TaskID).Next {
			t.Fatalf("%s is not in Next", target.TaskID)
		}
	}
	var tagged tasksResultView
	h.ok(h.call("command.bulk", map[string]any{"tasks": selection, "action": "tags", "add_tags": []string{"Home", "weekend"}}), &tagged)
	h.ok(h.call("command.bulk", map[string]any{"tasks": selection[:1], "action": "tags", "remove_tags": []string{"weekend"}}), nil)
	if tags := h.detail(selection[0].ProjectID, selection[0].TaskID).Task.Tags; !slices.Equal(tags, []string{"home"}) {
		t.Fatalf("tags %v", tags)
	}
	if tags := h.detail(selection[1].ProjectID, selection[1].TaskID).Task.Tags; !slices.Equal(tags, []string{"home", "weekend"}) {
		t.Fatalf("tags %v", tags)
	}
	var started tasksResultView
	h.ok(h.call("command.bulk", map[string]any{"tasks": selection, "action": "state", "state": "doing"}), &started)
	for _, view := range started.Tasks {
		if view.State != "doing" {
			t.Fatalf("%s is %s", view.Title, view.State)
		}
	}
	// Planning follows a move: each task keeps the plan it held before.
	h.ok(h.call("command.bulk", map[string]any{"tasks": selection[:1], "action": "plan", "plan": "add_today"}), nil)
	plannedBefore := map[string][2]bool{}
	for _, target := range selection {
		read := h.detail(target.ProjectID, target.TaskID)
		plannedBefore[target.TaskID] = [2]bool{read.Today, read.Next}
	}
	var moved tasksResultView
	h.ok(h.call("command.bulk", map[string]any{"tasks": selection, "action": "move", "project_id": homeProject}), &moved)
	if len(moved.Moves) != 3 {
		t.Fatalf("moves %+v", moved.Moves)
	}
	for _, move := range moved.Moves {
		read := h.detail(move.To.ProjectID, move.To.TaskID)
		if got := [2]bool{read.Today, read.Next}; got != plannedBefore[move.From.TaskID] {
			t.Fatalf("%s planned %v before the move and %v after", move.To.TaskID, plannedBefore[move.From.TaskID], got)
		}
	}
	if plannedBefore[selection[0].TaskID] != [2]bool{true, false} {
		t.Fatalf("the Today task reads %v", plannedBefore[selection[0].TaskID])
	}
	h.unchanged("a refused selection", func() {
		h.rejects(h.call("command.bulk", map[string]any{
			"tasks": []locator{moved.Moves[0].To, task(atlasProject, "0a7e5d10-2c3b-4f6a-8e9d-000000000108")}, "action": "plan", "plan": "add_today",
		}), CodeValidation)
	})
	h.rejects(h.call("command.bulk", map[string]any{"tasks": []locator{}, "action": "plan", "plan": "add_today"}), CodeInvalidRequest)
}

func organizeProjectLifecycle(t *testing.T, h *harness) {
	var created projectResult
	h.ok(h.call("command.project", map[string]any{"action": "create", "name": "Garden Shed"}), &created)
	if created.Project.Slug != "garden-shed" || created.Committed.Domains["catalog"] == "" {
		t.Fatalf("created %+v", created)
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "projects", "garden-shed", "tasks.md")); err != nil {
		t.Fatalf("no tasks.md: %v", err)
	}
	h.unchanged("a duplicate project", func() {
		refusal := h.refused(h.call("command.project", map[string]any{"action": "create", "name": "garden shed"}), CodeValidation)
		if !strings.Contains(refusal.Message, "already exists") {
			t.Fatalf("refusal %+v", refusal)
		}
	})
	id := created.Project.ID
	h.ok(h.call("command.capture", map[string]any{"project_id": id, "title": "Buy timber"}), nil)
	var edited projectResult
	h.ok(h.call("command.project", map[string]any{
		"action": "edit", "project_id": id, "name": "Garden Workshop", "description": "Out back", "tags": []string{"Home"},
	}), &edited)
	if edited.Project.Slug != "garden-workshop" || edited.Project.Description != "Out back" || !slices.Equal(edited.Project.Tags, []string{"home"}) {
		t.Fatalf("edited %+v", edited.Project)
	}
	if _, err := os.Stat(filepath.Join(h.dataDir, "projects", "garden-shed")); !os.IsNotExist(err) {
		t.Fatalf("the old slug's directory remains (err %v)", err)
	}
	if !slices.Contains(h.titles(map[string]any{"mode": "all", "scope": id}), "Buy timber") {
		t.Fatal("the renamed project lost its task")
	}
	var parked projectResult
	h.ok(h.call("command.project", map[string]any{"action": "set_tags", "project_id": id, "tags": []string{"home", "parked"}}), &parked)
	if !parked.Project.Parked {
		t.Fatalf("parked %+v", parked.Project)
	}
	var projects projectsResult
	h.ok(h.call("query.projects", nil), &projects)
	if !slices.ContainsFunc(projects.Parked, func(view projectView) bool { return view.ID == id }) {
		t.Fatal("the parked project is not listed as parked")
	}
	h.ok(h.call("command.project", map[string]any{"action": "set_tags", "project_id": id, "tags": []string{"home"}}), nil)
	h.ok(h.call("command.project", map[string]any{"action": "archive", "project_id": id}), nil)
	h.ok(h.call("query.projects", nil), &projects)
	if !slices.ContainsFunc(projects.Archived, func(view projectView) bool { return view.ID == id }) {
		t.Fatal("the archived project is not listed as archived")
	}
	h.ok(h.call("command.project", map[string]any{"action": "unarchive", "project_id": id}), nil)
	unfiled := h.unfiled()
	for _, refused := range []map[string]any{
		{"action": "edit", "project_id": unfiled.ID, "name": "Inbox"},
		{"action": "set_tags", "project_id": unfiled.ID, "tags": []string{"parked"}},
		{"action": "archive", "project_id": unfiled.ID},
	} {
		h.unchanged("a protected Unfiled "+refused["action"].(string), func() {
			h.rejects(h.call("command.project", refused), CodeRefused)
		})
	}
}

// organizeGuardedProjectDelete is the destructive confirmation: a delete must
// echo the impact the person was shown, a stale one is refused naming the
// current impact, and the system Unfiled project is never deleted.
func organizeGuardedProjectDelete(t *testing.T, h *harness) {
	var detail projectDetail
	h.ok(h.call("query.project", map[string]any{"project_id": homeProject}), &detail)
	if detail.Impact.Tasks == 0 || detail.Impact.Digest == "" {
		t.Fatalf("impact %+v", detail.Impact)
	}
	h.unchanged("an unconfirmed delete", func() {
		h.rejects(h.call("command.project", map[string]any{"action": "delete", "project_id": homeProject}), CodeInvalidRequest)
	})
	shown := detail.Impact
	h.ok(h.call("command.capture", map[string]any{"project_id": homeProject, "title": "Paint the hallway"}), nil)
	h.unchanged("a delete confirmed before the project grew", func() {
		refusal := h.refused(h.call("command.project", map[string]any{"action": "delete", "project_id": homeProject, "confirm": shown}), CodeStaleRevision)
		if refusal.Details["impact"] == nil {
			t.Fatalf("refusal %+v", refusal)
		}
	})
	h.ok(h.call("query.project", map[string]any{"project_id": homeProject}), &detail)
	if detail.Impact.Tasks != shown.Tasks+1 {
		t.Fatalf("impact after the capture %+v", detail.Impact)
	}
	var deleted projectResult
	h.ok(h.call("command.project", map[string]any{"action": "delete", "project_id": homeProject, "confirm": detail.Impact}), &deleted)
	if _, err := os.Stat(filepath.Join(h.dataDir, "projects", "home-renovation")); !os.IsNotExist(err) {
		t.Fatalf("the deleted project's directory remains (err %v)", err)
	}
	h.rejects(h.call("query.project", map[string]any{"project_id": homeProject}), CodeNotFound)
	unfiled := h.unfiled()
	var unfiledDetail projectDetail
	h.ok(h.call("query.project", map[string]any{"project_id": unfiled.ID}), &unfiledDetail)
	h.unchanged("deleting Unfiled", func() {
		h.rejects(h.call("command.project", map[string]any{"action": "delete", "project_id": unfiled.ID, "confirm": unfiledDetail.Impact}), CodeRefused)
	})
}
