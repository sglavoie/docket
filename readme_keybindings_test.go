package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// repositoryRootDir resolves the repository root from this source file rather
// than the working directory, so a contract test keeps reading the same
// documents after a t.Chdir or a subtest that moves cwd.
func repositoryRootDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve caller path")
	}
	return filepath.Dir(file)
}

// readRepositoryDoc loads one document for the contracts where file identity is
// itself the contract: the README's own phrases, and a heading a test anchors to
// a named guide file.
func readRepositoryDoc(t *testing.T, relative string) string {
	t.Helper()

	path := filepath.Join(repositoryRootDir(t), filepath.FromSlash(relative))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// readGuideCorpus concatenates every behavior-guide file. The guide is one
// reference split for reading, not for asserting: a contract may state its rule
// wherever it reads best, and requireDocumentationText normalizes whitespace on
// both sides, so the concatenation behaves exactly like one document. Placement
// is pinned only where a heading anchor says so.
func readGuideCorpus(t *testing.T) string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(repositoryRootDir(t), "docs", "guide", "*.md"))
	if err != nil {
		t.Fatalf("glob behavior guide: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("docs/guide holds no Markdown files")
	}
	sort.Strings(paths)

	var corpus strings.Builder
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read %s: %v", path, readErr)
		}
		corpus.Write(data)
		corpus.WriteString("\n")
	}
	return corpus.String()
}

func readREADMEForContractTest(t *testing.T) string {
	t.Helper()

	return readRepositoryDoc(t, "README.md")
}

func TestReadREADMEForContractTestUsesStableBaseDir(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(wd); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})

	if err := os.Chdir("cmd"); err != nil {
		t.Fatalf("chdir to cmd: %v", err)
	}

	content := readREADMEForContractTest(t)
	if !strings.Contains(content, "# tt — Terminal Tasks") {
		t.Fatal("expected README content to load regardless of cwd")
	}
}

func requireDocumentationText(t *testing.T, content string, required []string) {
	t.Helper()
	content = strings.Join(strings.Fields(content), " ")
	for _, needle := range required {
		if !strings.Contains(content, strings.Join(strings.Fields(needle), " ")) {
			t.Errorf("documentation missing %q", needle)
		}
	}
}

func rejectDocumentationText(t *testing.T, content string, forbidden []string) {
	t.Helper()
	content = strings.Join(strings.Fields(content), " ")
	for _, needle := range forbidden {
		if strings.Contains(content, strings.Join(strings.Fields(needle), " ")) {
			t.Errorf("documentation contains retired text %q", needle)
		}
	}
}

func TestREADMEDocumentsFinalDailyLoop(t *testing.T) {
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"The Outliner is the only primary task presentation",
		"`Tab` cycles exactly All → Today → Inbox → Next → Backlog → Due",
		"capture, triage, plan, execute, and complete",
	})
	// The final section catalog is intentionally checked as one ordered string.
	requireDocumentationText(t, content, []string{
		"Pinned, Today, Doing, Review, Inbox, Next, Paused, Backlog, Waiting, Someday",
	})
	rejectDocumentationText(t, content, []string{
		// All has no automatic Stale lane; old work stays in its lifecycle group.
		"Review, Stale, Inbox",
		"Next → Backlog → Active → All",
		"Command Palette",
		"Inbox Processor",
		"Weekly Review",
		"`Ctrl+Esc` reset",
		"Press `:` for Finder",
	})
}

// TestREADMEDocumentsDevelopmentEntryPoints keeps the two verification entry
// points on the front door itself. They answer a question a reader has before
// they reach the guide, so the corpus is the wrong place to accept them.
func TestREADMEDocumentsDevelopmentEntryPoints(t *testing.T) {
	content := readREADMEForContractTest(t)
	requireDocumentationText(t, content, []string{
		"./scripts/run_tui_scenarios.sh",
		"docs/architecture.md",
	})
	rejectDocumentationText(t, content, []string{
		"## Settings",
		"## Finder",
	})
}
