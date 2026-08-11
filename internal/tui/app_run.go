package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/util"
)

// AppVersion is the application version string used when writing theme files.
// Set this from the cmd package before calling RunApp.
var AppVersion = "dev"

// RunOptions configures process-level TUI behavior independently from the
// AppModel options used by tests and dependency injection.
type RunOptions struct {
	DisableColor bool
}

// RunApp starts the TUI application.
func RunApp(runtime *config.RuntimeState, manager *config.Manager, runOpts RunOptions, opts ...AppOption) error {
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

	p := tea.NewProgram(model, tea.WithEnvironment(tuiEnvironment(os.Environ(), runOpts.DisableColor)))

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

	_, err = p.Run()
	return err
}

// tuiEnvironment ignores an inherited NO_COLOR for the interactive TUI while
// preserving every other terminal capability hint. The explicit --no-color
// flag sets DisableColor, in which case cmd has already normalized NO_COLOR=1
// and the renderer receives the process environment unchanged.
func tuiEnvironment(environ []string, disableColor bool) []string {
	result := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if !disableColor && strings.EqualFold(key, "NO_COLOR") {
			continue
		}
		result = append(result, entry)
	}
	return result
}
