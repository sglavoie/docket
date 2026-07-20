package tui

import (
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func (m *AppModel) handleExportMessages(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case ExportProjectMsg:
		return true, m.handleExportProjectMsg(msg)
	}
	return false, nil
}

func (m *AppModel) handleExportProjectMsg(msg ExportProjectMsg) tea.Cmd {
	p := m.resolveExportProject(msg)
	if p == nil {
		return tea.Batch(
			func() tea.Msg { return StatusMsg("No project under cursor") },
			defaultStatusTimeout(),
		)
	}

	defaultPath := m.configManager.ExportPath(p.Slug)

	// Repeat-press: copy path to clipboard if within 10s of last export.
	if time.Since(m.lastExportAt) < 10*time.Second && m.lastExportPath == defaultPath {
		path := m.lastExportPath
		m.lastExportPath = ""
		m.lastExportAt = time.Time{}
		if err := clipboard.WriteAll(path); err != nil {
			return tea.Batch(
				func() tea.Msg { return StatusMsg(fmt.Sprintf("Clipboard error: %v", err)) },
				defaultStatusTimeout(),
			)
		}
		return tea.Batch(
			func() tea.Msg { return StatusMsg("Path copied: " + path) },
			defaultStatusTimeout(),
		)
	}

	if err := os.MkdirAll(m.configManager.ExportsDir(), 0700); err != nil {
		return tea.Batch(
			func() tea.Msg { return StatusMsg(fmt.Sprintf("Export failed: %v", err)) },
			defaultStatusTimeout(),
		)
	}

	path, err := storage.ExportProjectMarkdown(p, defaultPath)
	if err != nil {
		return tea.Batch(
			func() tea.Msg { return StatusMsg(fmt.Sprintf("Export failed: %v", err)) },
			defaultStatusTimeout(),
		)
	}
	m.lastExportPath = path
	m.lastExportAt = time.Now()
	return tea.Batch(
		func() tea.Msg {
			return StatusMsg("Exported to " + path + " — Ctrl+E again to copy path")
		},
		defaultStatusTimeout(),
	)
}

// resolveExportProject returns the project to export based on msg and the active view.
func (m *AppModel) resolveExportProject(msg ExportProjectMsg) *models.Project {
	if msg.Project != nil {
		return msg.Project
	}
	switch m.currentView() { //nolint:exhaustive // only a few views support project export
	case ViewDashboard:
		return m.dashboard.SelectedProject()
	case ViewOutliner:
		if m.outliner != nil {
			return m.outliner.getProject()
		}
	case ViewCrossProjectOutliner:
		return m.crossOutlinerCursorProject()
	case ViewAllTasks:
		if m.allTasksView != nil {
			_, p := m.allTasksView.getTaskRef()
			return p
		}
	}
	return nil
}

// crossOutlinerCursorProject returns the project for the cursor row in the
// cross-project outliner, or nil for separators, section headers, and project
// sub-headers.
func (m *AppModel) crossOutlinerCursorProject() *models.Project {
	if m.crossOutliner == nil {
		return nil
	}
	rows := m.crossOutliner.rows
	cursor := m.crossOutliner.cursor
	if cursor < 0 || cursor >= len(rows) {
		return nil
	}
	row := rows[cursor]
	if !row.isTaskRow() || row.task == nil {
		return nil
	}
	for _, p := range m.crossOutliner.loadedProjects {
		if p.ID == row.ProjectID {
			return p
		}
	}
	return nil
}
