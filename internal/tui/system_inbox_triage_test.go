package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSystemInboxTriageOffersMoveAndCreateProject(t *testing.T) {
	inbox := testutil.NewTestProject(config.SystemInboxName)
	inbox.Slug = config.SystemInboxSlug
	task := testutil.NewTestTask("Route me", models.TaskTypeTask)
	task.ApplyWorkflowState(models.StateInbox)
	inbox.Tasks = []*models.Task{task}
	destination := testutil.NewTestProject("Destination")

	runtime := testConfigWithProjects(inbox, destination)
	catalog := cloneWorkspaceCatalog(runtime.Catalog())
	catalog.SystemInboxProjectID = inbox.ID
	runtime.SetCatalogInMemory(catalog)

	for _, tc := range []struct {
		key   rune
		check func(t *testing.T, msg tea.Msg)
	}{
		{
			key: 'm',
			check: func(t *testing.T, msg tea.Msg) {
				move, ok := msg.(ShowProjectSelectorForMoveMsg)
				if !ok || move.Task != task || move.SourceProject != inbox {
					t.Fatalf("m emitted %#v, want move from system Inbox", msg)
				}
			},
		},
		{
			key: 'p',
			check: func(t *testing.T, msg tea.Msg) {
				create, ok := msg.(CreateProjectMsg)
				if !ok || create.MoveTask != task || create.SourceProject != inbox {
					t.Fatalf("p emitted %#v, want create-project continuation", msg)
				}
			},
		},
	} {
		t.Run(string(tc.key), func(t *testing.T) {
			processor := NewInboxProcessorModel(inbox, nil, runtime)
			_, cmd := processor.Update(tea.KeyPressMsg{Code: tc.key, Text: string(tc.key)})
			if cmd == nil {
				t.Fatalf("%c returned no command", tc.key)
			}
			tc.check(t, cmd())
		})
	}
}
