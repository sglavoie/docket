package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/store"
	"github.com/sglavoie/docket/internal/tui"
)

func dbPath() (string, error) {
	if p := os.Getenv("DOCKET_DB"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "docket-v2", "docket.db"), nil
}

func run() error {
	path, err := dbPath()
	if err != nil {
		return err
	}
	st, err := store.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer st.Close()
	tree, err := st.Load()
	if err != nil {
		return err
	}
	folds, err := st.Folds()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(tui.New(tree, st, folds)).Run()
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "docket:", err)
		os.Exit(1)
	}
}
