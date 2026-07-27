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

// resolveExportProject returns the explicit direct-key target when present, then
// delegates palette/shell resolution to the canonical project-command boundary.
func (m *AppModel) resolveExportProject(msg ExportProjectMsg) *models.Project {
	if msg.Project != nil {
		return msg.Project
	}
	if _, project := m.outlinerTarget(); project != nil {
		return project
	}
	return m.currentProject
}
