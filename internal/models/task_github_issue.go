package models

import (
	"net/url"
	"strconv"
	"strings"
)

// GitHubIssueRef identifies a GitHub issue linked from a task.
type GitHubIssueRef struct {
	Owner  string
	Repo   string
	Number int
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
	}, true
}
