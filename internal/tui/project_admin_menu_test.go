package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestProjectAdminMenuDispatchesContextualCommands(t *testing.T) {
	project := &models.Project{ID: "P", Name: "Peta"}

	cases := []struct {
		name   string
		action projectAdminAction
		verify func(t *testing.T, msg any)
	}{
		{"preview", adminActionPreview, func(t *testing.T, msg any) {
			m, ok := msg.(ShowMetricsDashboardMsg)
			if !ok || m.Project != project {
				t.Fatalf("expected metrics preview, got %#v", msg)
			}
		}},
		{"scope", adminActionScope, func(t *testing.T, msg any) {
			m, ok := msg.(ApplyProjectScopeMsg)
			if !ok || m.ProjectID != project.ID {
				t.Fatalf("expected scope, got %#v", msg)
			}
		}},
		{"edit", adminActionEdit, func(t *testing.T, msg any) {
			if m, ok := msg.(EditProjectMsg); !ok || m.Project != project {
				t.Fatalf("expected edit, got %#v", msg)
			}
		}},
		{"status", adminActionStatus, func(t *testing.T, msg any) {
			if m, ok := msg.(ShowStatusInputMsg); !ok || m.Project != project {
				t.Fatalf("expected status, got %#v", msg)
			}
		}},
		{"pin", adminActionPin, func(t *testing.T, msg any) {
			if m, ok := msg.(ProjectPinnedToggledMsg); !ok || m.Project != project {
				t.Fatalf("expected pin toggle, got %#v", msg)
			}
		}},
		{"archive", adminActionArchive, func(t *testing.T, msg any) {
			m, ok := msg.(ProjectArchiveRequestMsg)
			if !ok || !m.Archive || m.Project != project {
				t.Fatalf("expected archive request, got %#v", msg)
			}
		}},
		{"notes", adminActionNotes, func(t *testing.T, msg any) {
			if m, ok := msg.(ShowNotesMsg); !ok || m.Project != project {
				t.Fatalf("expected notes, got %#v", msg)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			menu := newProjectAdminMenu(project)
			// Position the cursor on the requested action.
			found := false
			for i, item := range menu.items {
				if item.action == tc.action {
					menu.cursor = i
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("action %d not present in menu", tc.action)
			}
			tc.verify(t, menu.dispatch())
		})
	}
}

func TestProjectAdminMenuLabelsReflectLifecycleState(t *testing.T) {
	archivedPinned := &models.Project{ID: "P", Name: "Peta", Archived: true, Pinned: true}
	menu := newProjectAdminMenu(archivedPinned)

	labels := map[projectAdminAction]string{}
	for _, item := range menu.items {
		labels[item.action] = item.label
	}
	if labels[adminActionArchive] != "Unarchive project" {
		t.Fatalf("expected unarchive label for archived project, got %q", labels[adminActionArchive])
	}
	if labels[adminActionPin] != "Unpin project" {
		t.Fatalf("expected unpin label for pinned project, got %q", labels[adminActionPin])
	}

	// An archived project's admin archive action must request unarchival.
	menu.cursor = 0
	for i, item := range menu.items {
		if item.action == adminActionArchive {
			menu.cursor = i
		}
	}
	msg, ok := menu.dispatch().(ProjectArchiveRequestMsg)
	if !ok || msg.Archive {
		t.Fatalf("expected unarchive request for archived project, got %#v", menu.dispatch())
	}
}
