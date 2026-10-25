package mobileparity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/models"
)

const (
	atlasProject   = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a01"
	homeProject    = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a02"
	readingProject = "6f1d3c2a-1b4e-4c8a-9a0e-0d5b7c1e2a03"
	releaseNotes   = "0a7e5d10-2c3b-4f6a-8e9d-000000000102"
	signing        = "0a7e5d10-2c3b-4f6a-8e9d-000000000105"
	localize       = "0a7e5d10-2c3b-4f6a-8e9d-000000000106"
	darkIcons      = "0a7e5d10-2c3b-4f6a-8e9d-000000000107"
	paint          = "0a7e5d10-2c3b-4f6a-8e9d-000000000201"
	tiles          = "0a7e5d10-2c3b-4f6a-8e9d-000000000202"
)

// TestDesktopAndMobileOrganizeAlike holds the organizing work to one durable
// outcome on both frontends: the desktop through the application calls its
// checklist rows, `@` picker, `R`, hierarchy keys, selection actions, and
// project hub make, the phone through the bridge. Every scenario must leave
// byte-identical task Markdown and planning state, and the same catalog.
func TestDesktopAndMobileOrganizeAlike(t *testing.T) {
	t.Run("checklist", parityChecklist)
	t.Run("check_in", parityCheckIn)
	t.Run("hierarchy", parityHierarchy)
	t.Run("moves", parityMoves)
	t.Run("bulk", parityBulk)
	t.Run("projects", parityProjects)
}

func at(projectID, taskID string) app.TaskLocator {
	return app.TaskLocator{ProjectID: projectID, TaskID: taskID}
}

// step runs one desktop call and the phone's request for the same act, then
// compares the workspaces.
func (p *pair) step(desktop func(*app.Service) error, op string, payload any) {
	p.t.Helper()
	p.act(desktop, op, payload)
	p.requireSameWorkspace()
}

func (p *pair) act(desktop func(*app.Service) error, op string, payload any) {
	p.t.Helper()
	if err := desktop(p.desktop.runtime.Service()); err != nil {
		p.t.Fatalf("desktop %s: %v", op, err)
	}
	if err := p.mobile.call(op, payload, nil); err != nil {
		p.t.Fatalf("mobile %s: %v", op, err)
	}
}

// touchStamp is a task's touch instant. The model's sibling-order and
// cross-project primitives stamp it from the wall clock rather than the
// service clock, on the desktop and the phone alike, so the steps that reach
// them compare everything else.
var touchStamp = regexp.MustCompile(`updated: [^,]+`)

// stepTouched is step for an act whose touch instant is the wall clock's.
func (p *pair) stepTouched(desktop func(*app.Service) error, op string, payload any) {
	p.t.Helper()
	p.act(desktop, op, payload)
	p.requireSameFiles(func(data []byte) []byte { return touchStamp.ReplaceAll(data, []byte("updated: *")) })
}

func (p *pair) checklist(target app.TaskLocator) []map[string]any {
	p.t.Helper()
	ref := p.desktop.lookup(target)
	items := make([]map[string]any, 0, len(ref.Task.Checklist))
	for _, item := range ref.Task.Checklist {
		items = append(items, map[string]any{"text": item.Text, "done": item.Done})
	}
	return items
}

func parityChecklist(t *testing.T) {
	p := newPair(t)
	notes := at(atlasProject, releaseNotes)
	p.step(func(s *app.Service) error { _, err := s.AddChecklistItem(notes, "Proofread"); return err },
		"command.checklist", map[string]any{"task": wire(notes), "action": "add", "text": "Proofread"})
	p.step(func(s *app.Service) error { _, err := s.ToggleChecklistItem(notes, 1); return err },
		"command.checklist", map[string]any{"task": wire(notes), "action": "toggle", "index": 1, "checklist": p.checklist(notes)})
	p.step(func(s *app.Service) error { _, err := s.EditChecklistItem(notes, 2, "Proofread twice"); return err },
		"command.checklist", map[string]any{"task": wire(notes), "action": "edit", "index": 2, "text": "Proofread twice", "checklist": p.checklist(notes)})
	p.step(func(s *app.Service) error { _, err := s.MoveChecklistItem(notes, 2, 0); return err },
		"command.checklist", map[string]any{"task": wire(notes), "action": "move", "index": 2, "to": 0, "checklist": p.checklist(notes)})
	frozen := slices.Clone(p.desktop.lookup(notes).Task.Checklist)
	seen := p.checklist(notes)
	p.step(func(s *app.Service) error {
		_, err := s.PromoteChecklistItem(app.ChecklistPromotionRequest{Owner: notes, Index: 0, Checklist: frozen})
		return err
	}, "command.checklist", map[string]any{"task": wire(notes), "action": "promote", "index": 0, "checklist": seen})
	p.step(func(s *app.Service) error { _, err := s.RemoveChecklistItem(notes, 1); return err },
		"command.checklist", map[string]any{"task": wire(notes), "action": "remove", "index": 1, "checklist": p.checklist(notes)})
}

func parityCheckIn(t *testing.T) {
	p := newPair(t)
	targets := []app.TaskLocator{at(atlasProject, signing), at(atlasProject, localize)}
	next := time.Date(2026, time.October, 6, 0, 0, 0, 0, time.Local)
	p.step(func(s *app.Service) error {
		_, err := s.ExecuteTaskOperation(app.TaskOperationRequest{
			Operation: app.TaskOperationSetCheckIn, Targets: targets,
			CheckIn: models.CheckInSchedule{Next: &next, RepeatDays: 7},
		})
		return err
	}, "command.check_in", map[string]any{"tasks": []wireLocator{wire(targets[0]), wire(targets[1])}, "action": "set", "date": "2026-10-06", "repeat_days": 7})
	p.step(func(s *app.Service) error { _, err := s.MarkReviewed(targets[0]); return err },
		"command.check_in", map[string]any{"tasks": []wireLocator{wire(targets[0])}, "action": "acknowledge"})
	p.step(func(s *app.Service) error {
		_, err := s.ExecuteTaskOperation(app.TaskOperationRequest{Operation: app.TaskOperationSetCheckIn, Targets: targets[1:]})
		return err
	}, "command.check_in", map[string]any{"tasks": []wireLocator{wire(targets[1])}, "action": "clear"})
}

func parityHierarchy(t *testing.T) {
	p := newPair(t)
	icons := at(atlasProject, darkIcons)
	for _, title := range []string{"Draw the moon", "Draw the sun"} {
		p.step(func(s *app.Service) error {
			_, err := s.ExecuteTaskOperation(app.TaskOperationRequest{
				Operation: app.TaskOperationCreateChild, Targets: []app.TaskLocator{icons}, Text: title,
			})
			return err
		}, "command.hierarchy", map[string]any{"task": wire(icons), "action": "add_child", "title": title})
	}
	var children []string
	for _, candidate := range p.desktop.lookup(icons).Project.Tasks {
		if candidate.ParentID != nil && *candidate.ParentID == darkIcons {
			children = append(children, candidate.ID)
		}
	}
	if len(children) != 2 {
		t.Fatalf("children %v", children)
	}
	moon, sun := at(atlasProject, children[0]), at(atlasProject, children[1])
	p.stepTouched(func(s *app.Service) error { _, err := s.MoveTaskUp(sun); return err },
		"command.hierarchy", map[string]any{"task": wire(sun), "action": "move_up"})
	p.stepTouched(func(s *app.Service) error { _, err := s.IndentTask(moon); return err },
		"command.hierarchy", map[string]any{"task": wire(moon), "action": "indent"})
	p.stepTouched(func(s *app.Service) error { _, err := s.OutdentTask(moon); return err },
		"command.hierarchy", map[string]any{"task": wire(moon), "action": "outdent"})
	p.stepTouched(func(s *app.Service) error { _, err := s.Reparent(moon, nil); return err },
		"command.hierarchy", map[string]any{"task": wire(moon), "action": "clear_parent"})
	parentID := darkIcons
	p.stepTouched(func(s *app.Service) error { _, err := s.Reparent(moon, &parentID); return err },
		"command.hierarchy", map[string]any{"task": wire(moon), "action": "set_parent", "parent": wire(icons)})
}

func parityMoves(t *testing.T) {
	p := newPair(t)
	one := at(atlasProject, signing)
	// One task takes the line edit the `%` key submits.
	p.stepTouched(func(s *app.Service) error {
		ref := p.desktop.lookup(one)
		_, err := s.EditTaskLine(app.TaskLineEditRequest{
			Source: one, Title: ref.Task.Title, DestinationProjectID: readingProject, Action: "Move to Project",
		})
		return err
	}, "command.bulk", map[string]any{"tasks": []wireLocator{wire(one)}, "action": "move", "project_id": readingProject})
	// A selection takes the batch.
	moved := p.desktop.lookup(at(homeProject, paint)).Task.Dependencies[0]
	selection := []app.TaskLocator{at(moved.ProjectID, moved.TaskID), at(homeProject, tiles)}
	p.stepTouched(func(s *app.Service) error { _, err := s.BulkMoveToProject(selection, atlasProject); return err },
		"command.bulk", map[string]any{"tasks": []wireLocator{wire(selection[0]), wire(selection[1])}, "action": "move", "project_id": atlasProject})
}

func parityBulk(t *testing.T) {
	p := newPair(t)
	selection := []app.TaskLocator{at(atlasProject, localize), at(homeProject, tiles), at(unfiledProject, callTheBank)}
	wired := []wireLocator{wire(selection[0]), wire(selection[1]), wire(selection[2])}
	p.step(func(s *app.Service) error {
		_, err := s.ApplyPlanningBulk(app.PlanningBulkRequest{Targets: selection, Action: app.PlanningAddNext})
		return err
	}, "command.bulk", map[string]any{"tasks": wired, "action": "plan", "plan": "add_next"})
	p.step(func(s *app.Service) error {
		_, err := s.ExecuteTaskOperation(app.TaskOperationRequest{
			Operation: app.TaskOperationSetTags, Targets: selection, TagPatch: true, TagsToAdd: []string{"weekend"}, TagsToRemove: []string{"personal"},
		})
		return err
	}, "command.bulk", map[string]any{"tasks": wired, "action": "tags", "add_tags": []string{"weekend"}, "remove_tags": []string{"personal"}})
	p.step(func(s *app.Service) error {
		_, err := s.ExecuteTaskOperation(app.TaskOperationRequest{Operation: app.TaskOperationSetState, Targets: selection, State: models.StateDoing})
		return err
	}, "command.bulk", map[string]any{"tasks": wired, "action": "state", "state": "doing"})
}

// catalogProjects is the catalog's project records, the stored fields both
// frontends write.
func catalogProjects(t *testing.T, dataDir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dataDir, ".tt", "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Data struct {
			Projects []map[string]any `json:"projects"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	for _, project := range document.Data.Projects {
		// Creation and archive instants come from the wall clock on both sides.
		delete(project, "created_at")
		delete(project, "updated_at")
		delete(project, "archived_at")
	}
	encoded, _ := json.Marshal(document.Data.Projects)
	return string(encoded)
}

func parityProjects(t *testing.T) {
	p := newPair(t)
	runtime := p.desktop.runtime
	project := func(desktop func() (engine.ProjectChange, error), payload map[string]any) {
		t.Helper()
		if _, err := desktop(); err != nil {
			t.Fatalf("desktop %v: %v", payload["action"], err)
		}
		if err := p.mobile.call("command.project", payload, nil); err != nil {
			t.Fatalf("mobile %v: %v", payload["action"], err)
		}
		p.requireSameWorkspace()
		if desktop, mobile := catalogProjects(t, p.desktop.dataDir), catalogProjects(t, p.mobile.dataDir); desktop != mobile {
			t.Fatalf("catalogs differ:\n--- desktop\n%s\n--- mobile\n%s", desktop, mobile)
		}
	}
	project(func() (engine.ProjectChange, error) { return runtime.CreateProject("Garden Shed") },
		map[string]any{"action": "create", "name": "Garden Shed"})
	created := runtime.State().GetProject("garden-shed")
	if created == nil {
		t.Fatal("the desktop created no project")
	}
	id := created.ID
	project(func() (engine.ProjectChange, error) {
		return runtime.EditProject(id, engine.ProjectEdit{Name: "Garden Workshop", Description: "Out back", Tags: []string{"home"}})
	}, map[string]any{"action": "edit", "project_id": id, "name": "Garden Workshop", "description": "Out back", "tags": []string{"home"}})
	project(func() (engine.ProjectChange, error) { return runtime.SetProjectTags(id, []string{"home", "parked"}) },
		map[string]any{"action": "set_tags", "project_id": id, "tags": []string{"home", "parked"}})
	project(func() (engine.ProjectChange, error) { return runtime.SetProjectTags(id, []string{"home"}) },
		map[string]any{"action": "set_tags", "project_id": id, "tags": []string{"home"}})
	project(func() (engine.ProjectChange, error) { return runtime.ArchiveProject(homeProject) },
		map[string]any{"action": "archive", "project_id": homeProject})
	project(func() (engine.ProjectChange, error) { return runtime.UnarchiveProject(homeProject) },
		map[string]any{"action": "unarchive", "project_id": homeProject})
	var detail struct {
		Impact map[string]any `json:"impact"`
	}
	if err := p.mobile.call("query.project", map[string]any{"project_id": readingProject}, &detail); err != nil {
		t.Fatal(err)
	}
	project(func() (engine.ProjectChange, error) { return runtime.DeleteProject(readingProject) },
		map[string]any{"action": "delete", "project_id": readingProject, "confirm": detail.Impact})
	for _, dataDir := range []string{p.desktop.dataDir, p.mobile.dataDir} {
		if _, err := os.Stat(filepath.Join(dataDir, "projects", "reading-list", "tasks.md")); !os.IsNotExist(err) {
			t.Fatalf("the deleted project remains in %s (err %v)", dataDir, err)
		}
	}
}
