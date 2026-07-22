package tui

import (
	"sort"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func intPtr(value int) *int { return &value }

func deleteRequestFromCmd(t *testing.T, cmd tea.Cmd) TaskDeleteRequestedMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("delete action returned a nil command")
	}
	raw := cmd()
	msg, ok := raw.(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("delete action returned %T, want TaskDeleteRequestedMsg", raw)
	}
	return msg
}

func requestTaskIDs(msg TaskDeleteRequestedMsg) []string {
	ids := make([]string, 0, len(msg.Refs))
	for _, ref := range msg.Refs {
		if ref.Task != nil {
			ids = append(ids, ref.Task.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func mustNewTestTask(project *models.Project, title string) *models.Task {
	task, err := models.NewTask(title, len(project.Tasks)+1, models.TaskTypeTask, title)
	if err != nil {
		panic(err)
	}
	return task
}
