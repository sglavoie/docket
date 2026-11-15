package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	// One snapshot a day, taken before anything in the outline is changed.
	// A failure is said on screen and does not stop the work. A session
	// left open takes the later days' itself.
	backup := st.DailyBackup(time.Now().Format("2006-01-02"))
	tree, err := st.Load()
	if err != nil {
		return err
	}
	folds, err := st.Folds()
	if err != nil {
		return err
	}
	m := tui.New(tree, st, folds)
	if st.Renamed() {
		m.Notice("this database was copied from another place: it now syncs under a name of its own")
	}
	if backup != nil {
		m.Notice("no backup today: " + backup.Error())
	}
	_, err = tea.NewProgram(m).Run()
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "docket:", err)
		os.Exit(1)
	}
}
