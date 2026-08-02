package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// AppVersion is the application version string used when writing theme files.
// Set this from the cmd package before calling RunApp.
var AppVersion = "dev"

// RunApp starts the TUI application.
func RunApp(runtime *config.RuntimeState, manager *config.Manager, opts ...AppOption) error {
	if runtime == nil {
		return fmt.Errorf("runtime state is required")
	}
	var err error
	runtime, err = recoverTUIWorkspace(runtime)
	if err != nil {
		return err
	}
	dataDir := runtime.DataDir()
	// Keep the TUI PID lock machine-local: when the durable data dir lives on
	// iCloud Drive it must never sync, so route it through the local runtime dir.
	lockPath := config.TUIProcessLockPath(dataDir)
	if mkdirErr := os.MkdirAll(filepath.Dir(lockPath), 0700); mkdirErr != nil {
		return fmt.Errorf("create runtime dir: %w", mkdirErr)
	}
	lock, err := util.AcquirePIDLock(lockPath)
	if err != nil {
		return err
	}
	defer lock.Release()

	model, err := NewAppModel(runtime, manager, opts...)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model)

	// Recover from panics to restore terminal state before crashing.
	defer func() {
		if r := recover(); r != nil {
			p.Kill()
			_ = p.ReleaseTerminal()
			fmt.Fprintf(os.Stderr, "\ntt: unexpected panic: %v\n", r)
			fmt.Fprintf(os.Stderr, "Please report this at https://github.com/sglavoie/tt/issues\n")
			os.Exit(1)
		}
	}()

	finalModel, err := p.Run()
	if app, ok := finalModel.(*AppModel); ok {
		if app.currentProject != nil {
			now := time.Now()
			for _, t := range app.currentProject.Tasks {
				if t.State() == models.StateDoing {
					app.sessionStats.tasksDoing++
				}
				if t.SnoozedUntil != nil && t.SnoozedUntil.After(now) {
					app.sessionStats.tasksSnoozed++
				}
			}
		}
		printSessionSummary(app.sessionStats)
	}
	return err
}

// printSessionSummary writes a brief session summary to stderr after the TUI exits.
func printSessionSummary(stats sessionStats) {
	if stats.tasksCompleted == 0 && stats.tasksCreated == 0 && stats.tasksDoing == 0 && stats.tasksSnoozed == 0 {
		return
	}

	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

	var parts []string
	if stats.tasksCompleted > 0 {
		parts = append(parts, greenStyle.Render(fmt.Sprintf("%d", stats.tasksCompleted))+" completed")
	}
	if stats.tasksCreated > 0 {
		parts = append(parts, fmt.Sprintf("%d created", stats.tasksCreated))
	}
	if stats.tasksSnoozed > 0 {
		parts = append(parts, fmt.Sprintf("%d snoozed", stats.tasksSnoozed))
	}

	header := "Session"
	if dur := sessionDurationStr(stats.sessionStart); dur != "" {
		header = "Session (" + dur + ")"
	}

	var msg string
	if len(parts) > 0 {
		msg = header + ": " + strings.Join(parts, ", ")
	} else {
		msg = header + ":"
	}
	if stats.tasksDoing > 0 {
		msg += " · " + yellowStyle.Render(fmt.Sprintf("%d", stats.tasksDoing)) + " doing"
	}
	fmt.Fprintln(os.Stderr, msg)
}

// sessionDurationStr formats elapsed time since start as "Xm" or "Xh Ym".
// Returns empty string when elapsed is under one minute.
func sessionDurationStr(start time.Time) string {
	if start.IsZero() {
		return ""
	}
	d := time.Since(start).Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return ""
}
