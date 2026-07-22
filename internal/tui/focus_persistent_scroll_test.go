package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestFocusBoardViewPersistentScrollKeepsWindowOnMoveUp(t *testing.T) {
	project, err := models.NewProject("proj", "proj", "/tmp/test-proj")
	if err != nil {
		t.Fatal(err)
	}

	m := newTestFocusBoard()
	m.SetSize(100, 10)
	m.tabs.Active = boardTabNext

	for i := range 8 {
		task := makeBoardTask(t, fmt.Sprintf("id-%02d", i), i+1, fmt.Sprintf("Task %02d", i))
		row := boardRow{Task: task, Project: project, Source: project.Name}
		m.rows = append(m.rows, row)
		m.filtered = append(m.filtered, row)
	}

	m.cursor = 4
	_ = m.View()
	start := m.viewScrollOffset
	if start == 0 {
		t.Fatalf("expected initial render to scroll down, offset stayed at 0")
	}

	m.cursor = 3
	_ = m.View()
	if m.viewScrollOffset != start {
		t.Fatalf("moving up inside viewport changed first rendered row: offset=%d want %d", m.viewScrollOffset, start)
	}
}
