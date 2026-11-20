package tui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/store"
)

// copyEvery is how often a session left open copies the database off this
// machine, when something has changed since the last copy.
var copyEvery = time.Hour

// keepCopies is how many days' copies each machine keeps in the folder.
const keepCopies = 14

// copyDoneMsg is a copy's work in the folder, finished.
type copyDoneMsg struct{ done *store.CopyDone }

// ICloudBackups names the folder copies go to by default: Docket Backups in
// iCloud Drive, when this machine has iCloud Drive. Else there is none and
// copies are off until a folder is named.
func ICloudBackups() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	drive := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs")
	if info, err := os.Stat(drive); err != nil || !info.IsDir() {
		return ""
	}
	return filepath.Join(drive, "Docket Backups")
}

// Backups turns copies on with home as the folder they go to unless one was
// named or they were turned off, and begins the launch's copy.
func (m *Model) Backups(home string) {
	m.backupHome = home
	switch v, _ := m.backend.Meta(metaBackupDir); v {
	case "off":
		m.backupDir = ""
	case "":
		m.backupDir = home
	default:
		m.backupDir = v
	}
	m.copyStart(false, false)
}

// copyStart begins a copy of the database into the backup folder. The
// snapshot and the folder are worked in away from the keys, and copyDone
// records what was written. Unless force is set, a copy is skipped when
// nothing has changed since the last one.
func (m *Model) copyStart(loud, force bool) {
	if m.backupDir == "" {
		return
	}
	m.copyLoud = m.copyLoud || loud
	if m.copyIO != nil {
		m.copyWant = m.copyWant || loud
		return
	}
	if inside(m.backupDir, m.syncDir) {
		m.msg = "no backup copy: " + tilde(m.backupDir) + " shares the sync folder, which the database never goes in; :backup ~/folder names another"
		m.copyLoud = false
		return
	}
	m.copiedAt = m.now()
	job, err := m.backend.CopyBegin(m.backupDir, m.today().Format("2006-01-02"), keepCopies, force)
	if err != nil {
		m.copyFail(err)
		return
	}
	if job == nil {
		m.copyLoud = false
		return
	}
	m.copyIO = func() tea.Msg { return copyDoneMsg{job.Run()} }
}

// copyCmd hands the program a copy that has begun, once.
func (m *Model) copyCmd() tea.Cmd {
	if m.copyIO == nil || m.copySent {
		return nil
	}
	m.copySent = true
	return m.copyIO
}

func (m *Model) copyFail(err error) {
	m.msg = "no backup copy: " + err.Error()
	m.copyLoud, m.copyWant = false, false
}

// copyDone records a finished copy and says how it went when it was asked
// for. A failure is always said.
func (m *Model) copyDone(done *store.CopyDone) {
	m.copyIO, m.copySent = nil, false
	if err := m.backend.CopyEnd(done); err != nil {
		m.copyFail(err)
		return
	}
	if m.copyLoud {
		m.msg = "backed up to " + tilde(m.backupDir)
	}
	m.copyLoud = false
	if m.copyWant {
		m.copyWant = false
		m.copyStart(true, true)
	}
}

// copyQuit is the last copy, written whole before the program ends.
func (m *Model) copyQuit() {
	if m.backupDir == "" || inside(m.backupDir, m.syncDir) {
		return
	}
	_ = m.backend.Copy(m.backupDir, m.today().Format("2006-01-02"), keepCopies)
}

// backupCommand is :backup: a copy now, a folder for the copies, or off.
func (m *Model) backupCommand(arg string) {
	switch {
	case arg == "off":
		m.backupDir = ""
		if err := m.backend.SetMeta(metaBackupDir, "off"); err != nil {
			m.msg = "not saved: " + err.Error()
			return
		}
		m.msg = "backup copies are off"
		return
	case arg != "":
		dir := expandHome(arg)
		if inside(dir, m.syncDir) {
			m.msg = "no backup copy: " + tilde(dir) + " shares the sync folder, which the database never goes in"
			return
		}
		if err := m.backend.SetMeta(metaBackupDir, dir); err != nil {
			m.msg = "not saved: " + err.Error()
			return
		}
		m.backupDir = dir
	case m.backupDir == "" && m.backupHome == "":
		m.msg = "name a folder for the copies: backup ~/folder"
		return
	case m.backupDir == "":
		if err := m.backend.SetMeta(metaBackupDir, ""); err != nil {
			m.msg = "not saved: " + err.Error()
			return
		}
		m.backupDir = m.backupHome
	}
	m.copyStart(true, true)
}

// expandHome reads a leading ~/ as the home folder.
func expandHome(dir string) string {
	if rest, ok := strings.CutPrefix(dir, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return dir
}

// tilde writes the home folder as ~, so a long folder reads on one line.
func tilde(dir string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rest, ok := strings.CutPrefix(dir, home+string(filepath.Separator)); ok {
			return "~/" + rest
		}
	}
	return dir
}

// inside reports that one folder is the other or holds it, so copies and
// sync files would share a folder.
func inside(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = resolved(a), resolved(b)
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// resolved is a folder as an absolute path with its links followed as far
// as it exists, so two names for one folder compare alike.
func resolved(dir string) string {
	dir, _ = filepath.Abs(expandHome(dir))
	tail := ""
	for d := dir; ; d = filepath.Dir(d) {
		if real, err := filepath.EvalSymlinks(d); err == nil {
			return filepath.Join(real, tail)
		}
		if d == filepath.Dir(d) {
			return dir
		}
		tail = filepath.Join(filepath.Base(d), tail)
	}
}
