package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/store"
)

// copies lists the day files this database has in the copies' folder.
func (h *harness) copies(dir string) []string {
	h.t.Helper()
	device, err := h.st.Meta("device")
	if err != nil {
		h.t.Fatal(err)
	}
	found, err := filepath.Glob(filepath.Join(dir, device, "docket-*.db"))
	if err != nil {
		h.t.Fatal(err)
	}
	return found
}

// copied opens the newest copy and says whether it holds a row with text.
func (h *harness) copied(dir, text string) bool {
	h.t.Helper()
	found := h.copies(dir)
	if len(found) == 0 {
		return false
	}
	st, err := store.Open(found[len(found)-1])
	if err != nil {
		h.t.Fatal(err)
	}
	defer st.Close()
	tree, err := st.Load()
	if err != nil {
		h.t.Fatal(err)
	}
	for _, id := range tree.Children("") {
		for _, c := range append([]string{id}, tree.Children(id)...) {
			if r := tree.Row(c); r != nil && r.Text() == text {
				return true
			}
		}
	}
	return false
}

func TestCopiesGoOffThisMachineHourlyAndOnQuit(t *testing.T) {
	cloud := filepath.Join(t.TempDir(), "Docket Backups")
	h := start(t)
	h.press("o", `"Alpha"`, "esc")
	h.m.Backups(cloud)
	h.settle()
	if !h.copied(cloud, "Alpha") {
		t.Fatal("launch made no copy")
	}

	// Within the hour a change waits; past it, the minute's tick copies.
	h.press("o", `"Beta"`, "esc")
	h.tick()
	if h.copied(cloud, "Beta") {
		t.Fatal("copied before the hour was up")
	}
	h.clock = h.clock.Add(time.Hour)
	h.tick()
	if !h.copied(cloud, "Beta") {
		t.Fatal("the hourly copy did not come")
	}
	if h.m.msg != "" {
		t.Fatalf("an hourly copy spoke: %q", h.m.msg)
	}

	// An hour with nothing changed writes nothing.
	file := h.copies(cloud)[0]
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	h.clock = h.clock.Add(time.Hour)
	h.tick()
	if len(h.copies(cloud)) != 0 {
		t.Fatal("an unchanged outline was copied again")
	}

	// :backup copies now, changed or not, and says where.
	h.press(":", `"backup"`, "enter")
	if !h.copied(cloud, "Beta") || h.m.msg != "backed up to "+tilde(cloud) {
		t.Fatalf("msg = %q", h.m.msg)
	}

	// Quitting copies what changed since.
	h.press("o", `"Gamma"`, "esc", ":", `"q"`, "enter")
	if !h.copied(cloud, "Gamma") {
		t.Fatal("quit made no copy")
	}
}

func TestBackupNamesAFolderOrTurnsCopiesOff(t *testing.T) {
	cloud := filepath.Join(t.TempDir(), "Docket Backups")
	path := filepath.Join(t.TempDir(), "docket.db")
	h := open(t, path)
	h.press("o", `"Alpha"`, "esc")

	h.press(":", `"backup off"`, "enter")
	if h.m.msg != "backup copies are off" {
		t.Fatalf("msg = %q", h.m.msg)
	}
	// Off stays off across a launch, the default folder notwithstanding.
	h = resume(t, path, h.st, h.clock)
	h.m.Backups(cloud)
	h.settle()
	h.press(":", `"q"`, "enter")
	if len(h.copies(cloud)) != 0 {
		t.Fatal("copies were made while off")
	}

	// :backup alone turns them on again, to the default folder.
	h = resume(t, path, h.st, h.clock)
	h.m.Backups(cloud)
	h.press(":", `"backup"`, "enter")
	if !h.copied(cloud, "Alpha") {
		t.Fatal(":backup did not turn copies back on")
	}

	// A named folder is kept for later launches.
	other := filepath.Join(t.TempDir(), "elsewhere")
	h.press(":", `"backup `+other+`"`, "enter")
	if !h.copied(other, "Alpha") || h.m.msg != "backed up to "+tilde(other) {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h = resume(t, path, h.st, h.clock)
	h.m.Backups(cloud)
	if h.m.backupDir != other {
		t.Fatalf("backup folder = %q", h.m.backupDir)
	}

	// Without iCloud Drive there is no default to fall back on.
	h.press(":", `"backup off"`, "enter")
	h = resume(t, path, h.st, h.clock)
	h.m.Backups("")
	h.press(":", `"backup"`, "enter")
	if h.m.msg != "name a folder for the copies: backup ~/folder" {
		t.Fatalf("msg = %q", h.m.msg)
	}
}

func TestCopiesNeverShareTheSyncFolder(t *testing.T) {
	shared := t.TempDir()
	h := start(t)
	h.press(":", `"sync `+shared+`"`, "enter")

	h.press(":", `"backup `+filepath.Join(shared, "copies")+`"`, "enter")
	if !failing(h.m.msg) || h.m.backupDir != "" {
		t.Fatalf("msg = %q, folder = %q", h.m.msg, h.m.backupDir)
	}
	// A default inside the sync folder is said, and nothing is copied.
	h.m.Backups(filepath.Join(shared, "Docket Backups"))
	h.settle()
	if !failing(h.m.msg) || len(h.copies(filepath.Join(shared, "Docket Backups"))) != 0 {
		t.Fatalf("msg = %q", h.m.msg)
	}

	// Nor does sync move into the copies' folder.
	copies := t.TempDir()
	h.press(":", `"sync off"`, "enter", ":", `"backup `+copies+`"`, "enter")
	h.press(":", `"sync `+copies+`"`, "enter")
	if !failing(h.m.msg) || h.m.syncDir != "" {
		t.Fatalf("msg = %q, sync = %q", h.m.msg, h.m.syncDir)
	}
}

func TestAFailedCopyIsSaidAndWorkGoesOn(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "a file")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	h := start(t)
	h.press(":", `"backup `+blocked+`"`, "enter")
	if !failing(h.m.msg) {
		t.Fatalf("msg = %q", h.m.msg)
	}
	h.press("o", `"Still works"`, "esc")
	h.want("▌ · Still works")
}

func TestBackupFolderIsReadAsAPathNotAShellWord(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "Mobile Documents"), 0o700); err != nil {
		t.Fatal(err)
	}
	h := start(t)
	h.press("o", `"Alpha"`, "esc")

	// A path pasted from a shell keeps its escapes off.
	pasted := filepath.Join(root, `Mobile\ Documents`, `com\~apple\~CloudDocs`)
	want := filepath.Join(root, "Mobile Documents", "com~apple~CloudDocs")
	h.press(":", `"backup `+pasted+`"`, "enter")
	h.settle()
	if h.m.backupDir != want || !h.copied(want, "Alpha") {
		t.Fatalf("folder = %q, msg = %q", h.m.backupDir, h.m.msg)
	}
	if _, err := os.Stat(filepath.Join(root, `Mobile\ Documents`)); err == nil {
		t.Fatal("a folder was made with the escapes in its name")
	}

	// A relative folder names a different place from Docket.app and from a
	// terminal, so it is refused, and the folder named before stays.
	h.press(":", `"backup copies"`, "enter")
	if !failing(h.m.msg) || h.m.backupDir != want {
		t.Fatalf("msg = %q, folder = %q", h.m.msg, h.m.backupDir)
	}

	// A folder whose parent is not there is a typo, not a tree to make.
	missing := filepath.Join(root, "Mobile Documentz", "Docket Backups")
	h.press(":", `"backup `+missing+`"`, "enter")
	if !failing(h.m.msg) || h.m.backupDir != want {
		t.Fatalf("msg = %q, folder = %q", h.m.msg, h.m.backupDir)
	}
	if _, err := os.Stat(filepath.Dir(missing)); err == nil {
		t.Fatal("a missing parent was made")
	}

	// :sync reads its folder the same way.
	h.press(":", `"sync shared"`, "enter")
	if !failing(h.m.msg) || h.m.syncDir != "" {
		t.Fatalf("msg = %q, sync = %q", h.m.msg, h.m.syncDir)
	}
}
