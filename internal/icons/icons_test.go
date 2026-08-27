package icons

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestGetUsesStableUnicodeRegistry(t *testing.T) {
	for key, want := range map[string]string{
		"date": "⊡", "status.backlog": "□", "priority.1": "◉",
		"attachment": "⊕", "checkbox.checked": "☑", "github_issue": "GH",
	} {
		if got := Get(key); got != want {
			t.Errorf("Get(%q) = %q, want %q", key, got, want)
		}
	}
	if got := Get("nonexistent_key"); got != "" {
		t.Errorf("Get(unknown) = %q", got)
	}
}

// dynamicIconKeys are the keys no literal names because PriorityIcon builds
// them with fmt.Sprintf. Everything else must be reachable from a literal call.
var dynamicIconKeys = []string{"priority.1", "priority.2", "priority.3"}

var literalIconGetCall = regexp.MustCompile(`icons\.Get\("([^"]+)"\)`)

// TestRegistryHoldsExactlyTheKeysCallersName is the anti-drift gate: the
// registry was once a speculative vocabulary whose unreferenced half nothing
// caught. A key that no call site names is dead, and a named key that the
// registry lacks renders as the empty string, so the two sets must be equal.
func TestRegistryHoldsExactlyTheKeysCallersName(t *testing.T) {
	reachable := map[string]bool{}
	for _, key := range dynamicIconKeys {
		reachable[key] = true
	}
	root := repositoryRootDir(t)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); path != root && (strings.HasPrefix(name, ".") || name == "testdata") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range literalIconGetCall.FindAllStringSubmatch(string(source), -1) {
			reachable[match[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	var unreachable, missing []string
	for key := range registry {
		if !reachable[key] {
			unreachable = append(unreachable, key)
		}
	}
	for key := range reachable {
		if _, ok := registry[key]; !ok {
			missing = append(missing, key)
		}
	}
	sort.Strings(unreachable)
	sort.Strings(missing)
	if len(unreachable) > 0 {
		t.Errorf("registry keys no icons.Get call names: %v", unreachable)
	}
	if len(missing) > 0 {
		t.Errorf("icons.Get keys the registry lacks: %v", missing)
	}
}

// repositoryRootDir walks up from this test file to the directory holding go.mod.
func repositoryRootDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", filepath.Dir(file))
		}
		dir = parent
	}
}

func TestColumnAlignedGroupsHaveConsistentWidth(t *testing.T) {
	for group, keys := range map[string][]string{
		"status":   {"status.inbox", "status.backlog", "status.doing", "status.waiting", "status.paused", "status.someday", "status.done", "status.canceled"},
		"priority": {"priority.1", "priority.2", "priority.3"},
		"checkbox": {"checkbox.checked", "checkbox.unchecked"},
	} {
		width := lipgloss.Width(Get(keys[0]))
		for _, key := range keys[1:] {
			if got := lipgloss.Width(Get(key)); got != width {
				t.Errorf("%s icon %q width = %d, want %d", group, key, got, width)
			}
		}
	}
}
