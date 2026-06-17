package tui

import (
	"strings"
	"testing"
)

func TestSearchHelpIncludesGitHubIssuePresenceFilter(t *testing.T) {
	localHelp := renderSearchHelpPane(80, 24)
	if !strings.Contains(localHelp, "issue") {
		t.Fatalf("local search help should include issue presence filter, got:\n%s", localHelp)
	}

	globalHelp := (&GlobalSearchModel{previewWidth: 80}).renderSearchHelp()
	if !strings.Contains(globalHelp, "issue") {
		t.Fatalf("global search help should include issue presence filter, got:\n%s", globalHelp)
	}
}
