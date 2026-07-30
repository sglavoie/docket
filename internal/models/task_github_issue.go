package models

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// GitHubIssueRef identifies a GitHub issue linked from a task.
type GitHubIssueRef struct {
	Owner  string
	Repo   string
	Number int
	URL    string
}

// ParseGitHubIssueURL extracts a GitHub issue reference from a GitHub issue URL.
func ParseGitHubIssueURL(raw string) (GitHubIssueRef, bool) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return GitHubIssueRef{}, false
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return GitHubIssueRef{}, false
	}

	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" && host != "www.github.com" {
		return GitHubIssueRef{}, false
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] != "issues" {
		return GitHubIssueRef{}, false
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return GitHubIssueRef{}, false
	}

	return GitHubIssueRef{
		Owner:  parts[0],
		Repo:   parts[1],
		Number: number,
		URL:    fmt.Sprintf("https://github.com/%s/%s/issues/%d", parts[0], parts[1], number),
	}, true
}

// GitHubIssueRefs returns unique GitHub issue references linked from the task.
func (t *Task) GitHubIssueRefs() []GitHubIssueRef {
	if t == nil {
		return nil
	}

	refs := make([]GitHubIssueRef, 0, 1)
	seen := map[string]struct{}{}
	addRef := func(raw string) {
		ref, ok := ParseGitHubIssueURL(raw)
		if !ok {
			return
		}
		key := fmt.Sprintf("%s/%s#%d", strings.ToLower(ref.Owner), strings.ToLower(ref.Repo), ref.Number)
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		refs = append(refs, ref)
	}

	for _, attachment := range t.Attachments {
		if attachment.IsURL {
			addRef(attachment.Filename)
			if attachment.OriginalPath != "" && attachment.OriginalPath != attachment.Filename {
				addRef(attachment.OriginalPath)
			}
		}
	}

	return refs
}

// HasGitHubIssue reports whether the task links to at least one GitHub issue.
func (t *Task) HasGitHubIssue() bool {
	return len(t.GitHubIssueRefs()) > 0
}
