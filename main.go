package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

const usage = `usage: docket              open the outline
       docket add LINE...   capture a row without opening it, as the a key
                            would: docket add "Buy milk %home @fri"
       docket add           capture one row for each line of standard input
       docket ls FILTER...  print the rows a filter matches, as / reads it, one
                            a line with a tab and where each lives:
                            docket ls '$next #home'
       docket ls            print the whole outline, as :export writes it`

func run() error {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Println(usage)
		return nil
	}
	if len(args) > 0 && args[0] != "add" && args[0] != "ls" {
		return errors.New("unknown command " + args[0] + "\n" + usage)
	}
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
	if len(args) > 0 && args[0] == "ls" {
		out := bufio.NewWriter(os.Stdout)
		for _, line := range tui.List(tree, st, strings.Join(args[1:], " ")) {
			fmt.Fprintln(out, line)
		}
		return out.Flush()
	}
	if len(args) > 0 {
		lines := []string{strings.Join(args[1:], " ")}
		if len(args) == 1 {
			lines = nil
			in := bufio.NewScanner(os.Stdin)
			in.Buffer(nil, 1<<20)
			for in.Scan() {
				lines = append(lines, in.Text())
			}
			if err := in.Err(); err != nil {
				return err
			}
		}
		said, err := tui.Capture(tree, st, lines)
		for _, line := range said {
			fmt.Println(line)
		}
		if err == nil && len(said) == 0 {
			err = errors.New("nothing to add")
		}
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
