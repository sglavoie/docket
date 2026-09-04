package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

func TestParseQuickCaptureChildReferences(t *testing.T) {
	project := quickCaptureTestProject()

	tests := []struct {
		name         string
		input        string
		wantTitle    string
		wantChildIDs []string
		wantParentID string
		errContains  string
	}{
		{
			name:         "by short id",
			input:        "Ship it <1",
			wantTitle:    "Ship it",
			wantChildIDs: []string{"epic-1"},
		},
		{
			name:         "by title prefix",
			input:        "Ship it <Login",
			wantTitle:    "Ship it",
			wantChildIDs: []string{"story-2"},
		},
		{
			name:         "repeatable and ordered",
			input:        "Ship it <3 <1",
			wantTitle:    "Ship it",
			wantChildIDs: []string{"task-3", "epic-1"},
		},
		{
			name:         "aliases for one task collapse to first occurrence",
			input:        "Ship it <Login <2 <1",
			wantTitle:    "Ship it",
			wantChildIDs: []string{"story-2", "epic-1"},
		},
		{
			name:         "parent and children coexist",
			input:        "Ship it >1 <2 <3",
			wantTitle:    "Ship it",
			wantParentID: "epic-1",
			wantChildIDs: []string{"story-2", "task-3"},
		},
		{
			name:        "zero is not a clear",
			input:       "Ship it <0",
			errContains: "child not found: '0'",
		},
		{
			name:        "unknown short id",
			input:       "Ship it <99",
			errContains: "child not found: '99'",
		},
		{
			name:        "ambiguous title prefix",
			input:       "Ship it <Implement",
			errContains: `child reference "Implement" is ambiguous`,
		},
		{
			name:        "parent named as its own child",
			input:       "Ship it >1 <1",
			errContains: "#1 cannot be both the parent and a child",
		},
		{
			name:        "conflict is independent of token order",
			input:       "Ship it <1 >1",
			errContains: "#1 cannot be both the parent and a child",
		},
		{
			name:      "embedded less-than stays literal",
			input:     "Compare a<b now",
			wantTitle: "Compare a<b now",
		},
		{
			name:      "numeric comparison stays literal",
			input:     "Ratio 5<3 holds",
			wantTitle: "Ratio 5<3 holds",
		},
		{
			name:      "spaced operator stays literal",
			input:     "Assert x < y",
			wantTitle: "Assert x < y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := grammar.ParseQuickCapture(tt.input, project, nil, false)
			if tt.errContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got none", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error %q should contain %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", result.Title, tt.wantTitle)
			}
			if !slices.Equal(result.ChildIDs(), tt.wantChildIDs) {
				t.Errorf("childIDs = %v, want %v", result.ChildIDs(), tt.wantChildIDs)
			}
			switch {
			case tt.wantParentID == "" && result.ParentID != nil:
				t.Errorf("parentID = %q, want nil", *result.ParentID)
			case tt.wantParentID != "" && (result.ParentID == nil || *result.ParentID != tt.wantParentID):
				t.Errorf("parentID = %v, want %q", result.ParentID, tt.wantParentID)
			}
		})
	}
}

func TestParseQuickCaptureChildMetadataFeedsPreviewWithoutTaskPointers(t *testing.T) {
	project := quickCaptureTestProject()
	result, err := grammar.ParseQuickCapture("Ship it <2", project, nil, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if len(result.Children) != 1 {
		t.Fatalf("children = %+v, want one", result.Children)
	}
	child := result.Children[0]
	if child.Id != "story-2" || child.ShortID != 2 || child.Title != "Login Flow" {
		t.Fatalf("resolved child = %+v, want story-2 / #2 / Login Flow", child)
	}

	// The parse result carries copies, so rewriting it cannot reach the project's task.
	result.Children[0].Title = "rewritten"
	if project.Tasks[1].Title != "Login Flow" {
		t.Fatalf("view-owned task title = %q, want Login Flow", project.Tasks[1].Title)
	}
}

func TestParseQuickCaptureChildErrorsSurfaceInPreviewWithoutFailingEarly(t *testing.T) {
	project := quickCaptureTestProject()
	result, errMsg := grammar.ParseQuickCaptureForPreview("Ship it !1 <99", project, nil, false)
	if !strings.Contains(errMsg, "child not found: '99'") {
		t.Fatalf("preview error = %q, want a child-not-found message", errMsg)
	}
	if result.Title != "Ship it" || result.Priority != 1 {
		t.Fatalf("preview kept parsing = %q / %d, want 'Ship it' / 1", result.Title, result.Priority)
	}
	if len(result.Children) != 0 {
		t.Fatalf("preview children = %+v, want none", result.Children)
	}
}

// TestQuickCaptureTerminalDestinationRefusesIncompleteAdoptedChild mirrors the
// inline editor on the one terminal-adoption policy: the composer refuses the
// line the application layer refuses instead of previewing finished work above
// an open task, and a finished child still captures terminal.
func TestQuickCaptureTerminalDestinationRefusesIncompleteAdoptedChild(t *testing.T) {
	for _, destination := range []string{"done", "canceled"} {
		project := quickCaptureTestProject()
		project.Tasks = append(project.Tasks, &models.Task{
			ID: "done-5", ShortID: 5, Title: "Finished work", WorkflowState: models.StateDone,
		})

		t.Run("open adopted child blocks "+destination, func(t *testing.T) {
			_, err := grammar.ParseQuickCapture("Ship it $"+destination+" <2", project, nil, false)
			want := "cannot capture as " + destination + ": adopted #2 is incomplete"
			if err == nil || err.Error() != want {
				t.Fatalf("error = %v, want %q", err, want)
			}
		})

		t.Run("finished adopted child allows "+destination, func(t *testing.T) {
			result, err := grammar.ParseQuickCapture("Ship it $"+destination+" <5", project, nil, false)
			if err != nil {
				t.Fatalf("parseQuickCapture: %v", err)
			}
			if got := result.ChildIDs(); len(got) != 1 || got[0] != "done-5" {
				t.Fatalf("children = %v, want [done-5]", got)
			}
		})
	}
}

func TestCrossProjectCaptureResolvesChildrenInTheRoutedProject(t *testing.T) {
	currentProject := &models.Project{
		ID:    "current-project",
		Slug:  "current-project",
		Name:  "Current Project",
		Tasks: []*models.Task{{ID: "local-1", ShortID: 1, Title: "Local Task"}},
	}
	targetProject := &models.Project{
		ID:   "other-project",
		Slug: "other-project",
		Name: "Other Project",
		Tasks: []*models.Task{
			{ID: "other-1", ShortID: 1, Title: "Other Parent"},
			{ID: "other-2", ShortID: 2, Title: "Other Child"},
		},
	}
	allProjects := []*models.Project{currentProject, targetProject}

	tests := []struct {
		name         string
		input        string
		wantChildIDs []string
		errContains  string
	}{
		{
			name:         "project token after the child token",
			input:        "Routed <2 %other-project",
			wantChildIDs: []string{"other-2"},
		},
		{
			name:         "project token before the child token",
			input:        "Routed %other-project <2",
			wantChildIDs: []string{"other-2"},
		},
		{
			name:         "parent and child both resolve in the routed project",
			input:        "Routed <2 >1 %other-project",
			wantChildIDs: []string{"other-2"},
		},
		{
			name:        "a source-project title never resolves after routing",
			input:       "Routed <Local %other-project",
			errContains: "child not found: 'Local'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := grammar.ParseQuickCapture(tt.input, currentProject, allProjects, false)
			if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("error = %v, want one containing %q", err, tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.TargetProjectID == nil || *result.TargetProjectID != "other-project" {
				t.Fatalf("targetProjectID = %v, want other-project", result.TargetProjectID)
			}
			if !slices.Equal(result.ChildIDs(), tt.wantChildIDs) {
				t.Fatalf("childIDs = %v, want %v", result.ChildIDs(), tt.wantChildIDs)
			}
		})
	}
}

func TestTaskComposerMapsResolvedChildrenIntoEveryRequest(t *testing.T) {
	due := time.Date(2030, 4, 1, 0, 0, 0, 0, time.Local)
	project := &models.Project{
		ID:   "project",
		Slug: "project",
		Name: "Project",
		Tasks: []*models.Task{
			{ID: "task-1", ShortID: 1, Title: "Alpha"},
			{ID: "task-2", ShortID: 2, Title: "Bravo"},
			{ID: "task-3", ShortID: 3, Title: "Charlie"},
		},
	}
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:        taskform.ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
		Shared: taskform.TaskComposerSharedAttributes{
			Priority: 2,
			Tags:     []string{"shared"},
			Due:      &due,
		},
	}, nil)
	composer.Input.SetValue("First <1 <Bravo #line\nSecond <3")

	reqs, ok := composer.ParseCompactLines()
	if !ok || len(reqs) != 2 {
		t.Fatalf("parseCompactLines = %+v, %v", reqs, ok)
	}

	first := reqs[0]
	if !slices.Equal(first.ChildIDs, []string{"task-1", "task-2"}) {
		t.Fatalf("first ChildIDs = %v, want [task-1 task-2]", first.ChildIDs)
	}
	if first.SourceLine != 1 || first.Title != "First" || first.ProjectID != "project" {
		t.Fatalf("first request = %+v", first)
	}
	if first.Priority != 2 || !slices.Equal(first.Tags, []string{"shared", "line"}) {
		t.Fatalf("first request dropped preset attributes: %+v", first)
	}
	if first.Due == nil || !first.Due.Equal(due) || !first.Prepend {
		t.Fatalf("first request due/prepend = %v / %v", first.Due, first.Prepend)
	}
	// Adopting children alone never selects the child destination default.
	if first.ParentID != nil || first.Destination != app.CaptureDestinationInbox {
		t.Fatalf("first request routing = %v / %q, want root Inbox", first.ParentID, first.Destination)
	}

	second := reqs[1]
	if second.SourceLine != 2 || second.Title != "Second" || !slices.Equal(second.ChildIDs, []string{"task-3"}) {
		t.Fatalf("second request = %+v", second)
	}
}

func TestTaskComposerReportsChildFailuresOnTheirSourceLine(t *testing.T) {
	project := &models.Project{
		ID:   "project",
		Slug: "project",
		Name: "Project",
		Tasks: []*models.Task{
			{ID: "task-1", ShortID: 1, Title: "Implement button"},
			{ID: "task-2", ShortID: 2, Title: "Implement search"},
		},
	}
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:        taskform.ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
	}, nil)
	composer.Input.SetValue("Fine <1\nMissing <9\nAmbiguous <Implement")

	reqs, ok := composer.ParseCompactLines()
	if ok || reqs != nil {
		t.Fatalf("parseCompactLines = %+v, %v; want a rejected batch", reqs, ok)
	}
	if len(composer.Errors) != 2 {
		t.Fatalf("errors = %+v, want two", composer.Errors)
	}
	if composer.Errors[0].Line != 2 || !strings.Contains(composer.Errors[0].Message, "child not found: '9'") {
		t.Fatalf("first error = %+v", composer.Errors[0])
	}
	if composer.Errors[1].Line != 3 || !strings.Contains(composer.Errors[1].Message, "is ambiguous") {
		t.Fatalf("second error = %+v", composer.Errors[1])
	}
}

func TestTaskComposerExpandRefusesToDropAdoptedChildren(t *testing.T) {
	appModel, project := newTestAppModelWithProject()
	existing, err := models.NewGenericTask("existing", 1, "Existing")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, existing)
	project.BuildTaskTree()

	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:    taskform.ComposerProject,
		Project: project,
	}, appModel.domain.runtime)
	composer.Input.SetValue("Adopter <1")

	if cmd := composer.ExpandSingle(); cmd != nil {
		t.Fatal("expand accepted a line carrying child references")
	}
	if composer.Mode != taskform.ComposerCompact || composer.Expanded != nil {
		t.Fatal("composer entered expanded mode despite the refusal")
	}
	if len(composer.Errors) != 1 || !strings.Contains(composer.Errors[0].Message, "cannot adopt children") {
		t.Fatalf("errors = %+v", composer.Errors)
	}
}

func TestTaskComposerCommitsAdoptedChildrenInOneTransaction(t *testing.T) {
	adoptee, err := models.NewGenericTask("adoptee", 1, "Adoptee")
	if err != nil {
		t.Fatal(err)
	}
	grandchild, err := models.NewGenericTask("grandchild", 2, "Grandchild")
	if err != nil {
		t.Fatal(err)
	}
	grandchild.ParentID = &adoptee.ID
	env := newScenarioEnv(t, "composer-adoption", adoptee, grandchild)
	composer := taskform.NewTaskComposerModel(taskform.TaskComposerPreset{
		Kind:    taskform.ComposerProject,
		Project: env.project,
		Shared:  taskform.TaskComposerSharedAttributes{Destination: app.CaptureDestinationInbox},
	}, nil)
	composer.SetCaptureCoordinator(env.features.Capture)
	composer.Input.SetValue("Adopter <1 <Adoptee")

	cmd := composer.CreateCompact(false)
	if cmd == nil {
		t.Fatal("adoption returned nil command")
	}
	if msg, ok := cmd().(coordinator.TaskCreatedMsg); !ok || msg.Result == nil {
		t.Fatalf("adoption result = %#v", msg)
	}

	fresh := env.reload(t)
	adopter := fresh.GetTaskByShortID(3)
	moved := fresh.GetTaskByID("adoptee")
	kept := fresh.GetTaskByID("grandchild")
	if adopter == nil || adopter.Title != "Adopter" {
		t.Fatalf("adopter = %+v", adopter)
	}
	if moved == nil || moved.ParentID == nil || *moved.ParentID != adopter.ID {
		t.Fatalf("adopted child = %+v, want parent %v", moved, adopter.ID)
	}
	if kept == nil || kept.ParentID == nil || *kept.ParentID != adoptee.ID {
		t.Fatalf("adopted subtree did not follow its parent: %+v", kept)
	}

	env.undo(t)
	reverted := env.reload(t)
	if len(reverted.Tasks) != 2 {
		t.Fatalf("one undo left %d tasks", len(reverted.Tasks))
	}
	if restored := reverted.GetTaskByID("adoptee"); restored == nil || restored.ParentID != nil {
		t.Fatalf("undo did not restore the adopted child to a root: %+v", restored)
	}
}

func TestQuickCaptureChildZeroIsReservedFromTitlePrefixResolution(t *testing.T) {
	project := &models.Project{
		ID: "zero-project", Name: "Zero Project", Slug: "zero",
		Tasks: []*models.Task{
			{ID: "zero-day", ShortID: 1, Title: "0 day triage"},
			{ID: "nine-lives", ShortID: 2, Title: "9 lives remaining"},
		},
	}

	_, err := grammar.ParseQuickCapture("Ship it <0", project, nil, false)
	if err == nil || !strings.Contains(err.Error(), "child not found: '0'") {
		t.Fatalf("<0 error = %v, want the reserved token to resolve nothing", err)
	}

	result, err := grammar.ParseQuickCapture("Ship it <9", project, nil, false)
	if err != nil {
		t.Fatalf("<9 parse: %v", err)
	}
	if ids := result.ChildIDs(); len(ids) != 1 || ids[0] != "nine-lives" {
		t.Fatalf("<9 children = %v, want [nine-lives]", ids)
	}

	q := draftedCaptureInput(t, "Ship it <0 <", project, nil)
	if got := suggestionDisplays(q.AC.Items); !slices.Contains(got, "<1 0 day triage") {
		t.Fatalf("suggestions after <0 = %v, want the zero-titled task still offered", got)
	}
}
