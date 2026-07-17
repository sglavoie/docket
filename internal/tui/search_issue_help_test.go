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

func TestSearchHelpExcludesRetiredQueueMembershipFilters(t *testing.T) {
	localHelp := renderSearchHelpPane(80, 24)
	globalHelp := (&GlobalSearchModel{previewWidth: 80}).renderSearchHelp()

	for name, help := range map[string]string{"local": localHelp, "global": globalHelp} {
		if strings.Contains(help, "active") || strings.Contains(help, "Active tasks") {
			t.Errorf("%s search help includes a retired queue-membership filter: %q", name, help)
		}
	}
}
