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
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/store"
	"github.com/sglavoie/docket/internal/tui"
)

// Set by the release build's ldflags; a plain `go build` leaves them so.
var (
	Version   = "dev"
	Commit    = ""
	BuildTime = ""
)

// versionLine is what `docket --version` prints. Docket.app reads it to
// say which docket it runs.
func versionLine() string {
	line := "docket version " + Version
	var detail []string
	if Commit != "" {
		detail = append(detail, "commit: "+Commit)
	}
	if BuildTime != "" {
		detail = append(detail, "built: "+BuildTime)
	}
	if len(detail) > 0 {
		line += " (" + strings.Join(detail, ", ") + ")"
	}
	return line
}

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
       docket ls            print the whole outline, as :export writes it
       docket --version     print the version`

func run() error {
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		fmt.Println(usage)
		return nil
	}
	if len(args) > 0 && args[0] == "--version" {
		fmt.Println(versionLine())
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
	// The outline reads the day from this clock; tests hand it one of their own.
	now := func() int64 { return time.Now().UnixMilli() }
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
		for _, line := range tui.List(tree, st, strings.Join(args[1:], " "), now) {
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
		said, err := tui.Capture(tree, st, lines, now)
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
	m := tui.Launch(tree, st, folds, now)
	if st.Renamed() {
		m.Notice("this database was copied from another place: it now syncs under a name of its own")
	}
	if backup != nil {
		m.Notice("no backup today: " + backup.Error())
	}
	// Copies go to iCloud Drive unless a folder was named or they were
	// turned off. A database named by DOCKET_DB, as just run's dev.db, has
	// no default, so a trial outline never reaches iCloud unasked.
	home := ""
	if os.Getenv("DOCKET_DB") == "" {
		home = tui.ICloudBackups()
	}
	m.Backups(home)
	final, err := tea.NewProgram(m).Run()
	// The TUI turned on light and dark reports; a shell must not receive them.
	fmt.Print(ansi.ResetModeLightDark)
	if last, ok := final.(*tui.Model); ok && err == nil {
		if line := last.Farewell(); line != "" {
			fmt.Println("docket: " + line)
		}
	}
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "docket:", err)
		os.Exit(1)
	}
}
