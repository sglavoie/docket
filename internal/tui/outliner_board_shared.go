package tui

import (
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/focus"
	"github.com/sglavoie/tt/internal/models"
)

// FocusTask is an alias for focus.FocusTask — the cross-project ordered task the
// daily plan / Next order index are built from. Retained after the standalone
// Planner (Focus Board) model was retired; the Outliner's Today section and
// Next mode build their ordering from these.
type FocusTask = focus.FocusTask

// Planner tab identities retained for daily-plan / Next reconciliation
// (planning_reconcile.go, task_composer.go).
const (
	boardTabToday   = 0
	boardTabNext    = 1
	boardTabBacklog = 2
)

// boardRow is a unified display item for an active/due task, retained as the
// row shape shared by the Outliner mode row builders and cross-project filters.
type boardRow struct {
	Task        *models.Task
	Project     *models.Project
	Priority    int
	Source      string
	TreePrefix  string // non-empty for auto-expanded children of planned items in Today
	GroupHeader string // non-empty for non-selectable group separator rows (Task will be nil)
	ParentID    string // immediate parent task ID for child rows in Today; empty for roots/non-tree rows
	Depth       int    // 0 for root tasks; n for n-th level descendant in Today
}

// buildBoardTreePrefix builds a tree prefix with shorter horizontal lines
// (one fewer ─) to fit the compact daily-driver layout.
func buildBoardTreePrefix(ancestorIsLast []bool, isLast bool) string {
	var b strings.Builder
	b.WriteByte(' ')
	for _, last := range ancestorIsLast {
		if last {
			b.WriteString("     ")
		} else {
			b.WriteString("│    ")
		}
	}
	if isLast {
		b.WriteString("└─── ")
	} else {
		b.WriteString("├─── ")
	}
	return b.String()
}

// openBrowserCmd opens a URL in the platform browser.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		if runtime.GOOS == "darwin" {
			cmd = exec.Command("open", url)
		} else {
			cmd = exec.Command("xdg-open", url)
		}
		if err := cmd.Start(); err == nil {
			go func() { _ = cmd.Wait() }()
		}
		return nil
	}
}
