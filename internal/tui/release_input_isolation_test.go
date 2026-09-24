package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

// This file is the combined isolation audit for the keys this release put on
// the Outliner. Each feature already proves its own key literal inside the one
// overlay it opens; what no single feature can prove is that all of them stay
// literal inside all of the others. A key that leaked would be silent: the
// character reaches the buffer *and* the operation fires behind it, so the typed
// text still looks right and the damage is a mutation nobody asked for.
//
// The keys are audited together because that is the property. Adding a seventh
// belongs in releaseInputKeys, not in a seventh test.

// releaseInputKeys are the Outliner root keys this release bound or rebound: c
// appends a checklist item, C a comment, b toggles Peek, D repeats a task, R
// acknowledges a review, and = flags one. r is here too — it opens the resume
// note prompt one of these contexts is — because a key that is safe inside its
// own overlay is the least obvious case, not an exempt one.
var releaseInputKeys = []string{"c", "C", "b", "D", "R", "=", "r"}

const releaseIsolationSettleTimeout = 2 * time.Second

// releaseIsolationEnv is one task carrying every field a leaked key could
// disturb: a checklist c would extend, a comment C would append to, a review
// flag = would flip and R would clear, and a resume note r would rewrite. The
// project's task count is what catches a D that reached the row behind an
// overlay.
type releaseIsolationEnv struct {
	driver    *UITestDriver
	projectID string
	taskID    string
}

const (
	releaseIsolationTitle = "Audit the combined key surface"
	releaseIsolationNote  = "left off in the isolation pass"
	releaseIsolationItem  = "Check the first item"
)

func newReleaseIsolationEnv(t *testing.T) *releaseIsolationEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var taskID string
	project, err := workspace.addProject("Release Isolation", func(project *models.Project) {
		task := newFixtureTask(project, releaseIsolationTitle)
		task.ApplyWorkflowState(models.StateBacklog)
		task.ResumeNote = releaseIsolationNote
		task.Checklist = []models.ChecklistItem{{Text: releaseIsolationItem}}
		task.Comments = []*models.Comment{{Text: "the one comment the audit starts with", CreatedAt: time.Now()}}
		taskID = task.ID
	})
	if err != nil {
		t.Fatalf("add project: %v", err)
	}
	workspace.setCurrentProject(project.ID)
	driver, err := workspace.launch(100, 35)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	driver.SetDrainTimeout(750 * time.Millisecond)
	env := &releaseIsolationEnv{driver: driver, projectID: project.ID, taskID: taskID}
	env.focusTask(t)
	return env
}

func (env *releaseIsolationEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

func (env *releaseIsolationEnv) outliner(t *testing.T) *outliner.OutlinerModel {
	t.Helper()
	out, ok := env.driver.App().outlinerPrimary()
	if !ok || out == nil {
		t.Fatal("the Outliner is not the active route")
	}
	return out
}

func (env *releaseIsolationEnv) focusTask(t *testing.T) {
	t.Helper()
	if env.walkToTask(t) {
		return
	}
	// A launch folds All's Backlog lane, so a task living there is reached by
	// opening every fold first.
	env.outliner(t).ClearAllFolds()
	if env.walkToTask(t) {
		return
	}
	t.Fatalf("the cursor never reached task %s", env.taskID)
}

func (env *releaseIsolationEnv) walkToTask(t *testing.T) bool {
	t.Helper()
	env.press(t, "home")
	for range len(env.outliner(t).Rows) + 1 {
		if task, _ := env.driver.App().getCurrentCursorTaskAndProject(); task != nil && task.ID == env.taskID {
			return true
		}
		env.press(t, "down")
	}
	return false
}

func (env *releaseIsolationEnv) project(t *testing.T) *models.Project {
	t.Helper()
	project := env.driver.App().projectByID(env.projectID)
	if project == nil {
		t.Fatalf("project %s is no longer loaded", env.projectID)
	}
	return project
}

// releaseIsolationState is everything a leaked release key would move. It is
// compared whole rather than field by field, so a key that starts writing a
// field this audit did not anticipate still has to be added here deliberately.
type releaseIsolationState struct {
	tasks      int
	review     bool
	comments   int
	checklist  int
	resumeNote string
	title      string
}

func (env *releaseIsolationEnv) state(t *testing.T) releaseIsolationState {
	t.Helper()
	project := env.project(t)
	task := project.GetTaskByID(env.taskID)
	if task == nil {
		t.Fatalf("task %s is no longer loaded", env.taskID)
	}
	return releaseIsolationState{
		tasks:      len(project.Tasks),
		review:     task.Review,
		comments:   len(task.Comments),
		checklist:  len(task.Checklist),
		resumeNote: task.ResumeNote,
		title:      task.Title,
	}
}

// releaseTextContext is one transient input overlay: how it opens, how to read
// the buffer the keys landed in, and how to leave it without committing.
type releaseTextContext struct {
	name string
	open []string
	// value reads the overlay's own buffer. A context whose overlay is gone by
	// the time this runs is a context that closed on one of the audited keys,
	// which is the failure it is looking for.
	value func(t *testing.T, env *releaseIsolationEnv) string
	close []string
}

// TestReleaseKeysStayLiteralInsideEveryTextEntry types every release key into
// every transient input overlay and requires two things at once: the buffer
// holds the characters verbatim, and the workspace behind the overlay is
// untouched. Either alone would pass a leak — a key that both inserted and
// fired writes the text and the mutation.
func TestReleaseKeysStayLiteralInsideEveryTextEntry(t *testing.T) {
	typed := strings.Join(releaseInputKeys, "")
	for _, context := range []releaseTextContext{
		{
			name: "slash search",
			open: []string{"/"},
			value: func(t *testing.T, env *releaseIsolationEnv) string {
				t.Helper()
				return env.outliner(t).SearchInputValue()
			},
			close: []string{"esc"},
		},
		{
			// The composer opens empty, so its Esc closes outright until text is
			// typed; the audited keys are that text, which is why it confirms.
			name: "task composer",
			open: []string{"a"},
			value: func(t *testing.T, env *releaseIsolationEnv) string {
				t.Helper()
				composer := env.driver.App().taskComposer
				if composer == nil {
					t.Fatal("the composer overlay closed on an audited key")
				}
				return composer.Input.Value()
			},
			close: []string{"esc", "y"},
		},
		{
			name: "resume note prompt",
			open: []string{"r"},
			value: func(t *testing.T, env *releaseIsolationEnv) string {
				t.Helper()
				return releaseIsolationPromptValue(t, env)
			},
			close: []string{"esc"},
		},
		{
			name: "quick comment popup",
			open: []string{"C"},
			value: func(t *testing.T, env *releaseIsolationEnv) string {
				t.Helper()
				return releaseIsolationPromptValue(t, env)
			},
			close: []string{"esc", "y"},
		},
		{
			name: "checklist add prompt",
			open: []string{"c"},
			value: func(t *testing.T, env *releaseIsolationEnv) string {
				t.Helper()
				return releaseIsolationPromptValue(t, env)
			},
			close: []string{"esc"},
		},
	} {
		t.Run(context.name, func(t *testing.T) {
			env := newReleaseIsolationEnv(t)
			before := env.state(t)

			env.press(t, context.open...)
			env.press(t, releaseInputKeys...)

			if got := context.value(t, env); !strings.Contains(got, typed) {
				t.Fatalf("%s buffer = %q, want it to carry the literal %q", context.name, got, typed)
			}
			if after := env.state(t); after != before {
				t.Fatalf("keys typed into %s changed the workspace: %+v, want %+v", context.name, after, before)
			}

			env.press(t, context.close...)
			env.driver.Wait(releaseIsolationSettleTimeout)
			if view := env.driver.App().currentView(); view != presentation.ViewCrossProjectOutliner {
				t.Fatalf("leaving %s landed on %s", context.name, view)
			}
			if after := env.state(t); after != before {
				t.Fatalf("abandoning %s committed something: %+v, want %+v", context.name, after, before)
			}
		})
	}
}

func releaseIsolationPromptValue(t *testing.T, env *releaseIsolationEnv) string {
	t.Helper()
	prompt := env.driver.App().operations.TextPrompt
	if prompt == nil {
		t.Fatal("the text prompt closed on an audited key")
	}
	return prompt.Value()
}

// TestReleaseKeysDoNotReachThroughSecondaryOverlays covers the overlays that
// take no text. A viewer or a panel has its own grammar, and an Outliner
// operation that fired underneath one would mutate a task the user was only
// reading — the case a literal-text assertion cannot see, because there is no
// buffer for the character to land in.
func TestReleaseKeysDoNotReachThroughSecondaryOverlays(t *testing.T) {
	for _, overlay := range []struct {
		name string
		open []string
		view presentation.PresentationID
	}{
		{name: "help viewer", open: []string{"?"}, view: presentation.ViewHelp},
		{name: "comments panel", open: []string{";"}, view: presentation.ViewTaskComments},
	} {
		t.Run(overlay.name, func(t *testing.T) {
			env := newReleaseIsolationEnv(t)
			before := env.state(t)

			env.press(t, overlay.open...)
			if view := env.driver.App().currentView(); view != overlay.view {
				t.Fatalf("%s opened %s, want %s", overlay.name, view, overlay.view)
			}
			for _, key := range releaseInputKeys {
				env.press(t, key)
				env.driver.Wait(releaseIsolationSettleTimeout)
				if after := env.state(t); after != before {
					t.Fatalf("%q reached the task behind the %s: %+v, want %+v", key, overlay.name, after, before)
				}
			}
		})
	}
}
