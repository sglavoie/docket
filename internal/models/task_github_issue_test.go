package models

import (
	"reflect"
	"testing"
)

func TestParseGitHubIssueURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want GitHubIssueRef
		ok   bool
	}{
		{
			name: "https issue URL",
			raw:  "https://github.com/acme/roadmap/issues/123",
			want: GitHubIssueRef{Owner: "acme", Repo: "roadmap", Number: 123, URL: "https://github.com/acme/roadmap/issues/123"},
			ok:   true,
		},
		{
			name: "www host with trailing slash",
			raw:  "https://www.github.com/acme/roadmap/issues/123/",
			want: GitHubIssueRef{Owner: "acme", Repo: "roadmap", Number: 123, URL: "https://github.com/acme/roadmap/issues/123"},
			ok:   true,
		},
		{
			name: "http URL with query and fragment",
			raw:  "http://github.com/Acme/Roadmap/issues/456?notification_referrer_id=abc#issuecomment-1",
			want: GitHubIssueRef{Owner: "Acme", Repo: "Roadmap", Number: 456, URL: "https://github.com/Acme/Roadmap/issues/456"},
			ok:   true,
		},
		{name: "pull request URL", raw: "https://github.com/acme/roadmap/pull/123"},
		{name: "issue list URL", raw: "https://github.com/acme/roadmap/issues"},
		{name: "non-numeric issue number", raw: "https://github.com/acme/roadmap/issues/not-a-number"},
		{name: "zero issue number", raw: "https://github.com/acme/roadmap/issues/0"},
		{name: "empty owner", raw: "https://github.com//roadmap/issues/123"},
		{name: "empty repo", raw: "https://github.com/acme//issues/123"},
		{name: "non-http scheme", raw: "git://github.com/acme/roadmap/issues/123"},
		{name: "non-github host", raw: "https://example.com/acme/roadmap/issues/123"},
		{name: "github subdomain", raw: "https://enterprise.github.com/acme/roadmap/issues/123"},
		{name: "extra path", raw: "https://github.com/acme/roadmap/issues/123/comments"},
		{name: "empty URL", raw: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseGitHubIssueURL(tt.raw)
			if ok != tt.ok {
				t.Fatalf("ParseGitHubIssueURL(%q) ok = %v, want %v", tt.raw, ok, tt.ok)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseGitHubIssueURL(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestTaskGitHubIssueRefs(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment("https://www.github.com/acme/roadmap/issues/123?ref=duplicate", true, "https://www.github.com/acme/roadmap/issues/123?ref=duplicate"); err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment("https://github.com/acme/roadmap/issues/456#details", true, "https://github.com/acme/roadmap/issues/456#details"); err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment("https://github.com/acme/roadmap/pull/789", true, "https://github.com/acme/roadmap/pull/789"); err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment("https://github.com/acme/roadmap/issues/999", false, "https://github.com/acme/roadmap/issues/999"); err != nil {
		t.Fatal(err)
	}

	want := []GitHubIssueRef{
		{Owner: "acme", Repo: "roadmap", Number: 123, URL: "https://github.com/acme/roadmap/issues/123"},
		{Owner: "acme", Repo: "roadmap", Number: 456, URL: "https://github.com/acme/roadmap/issues/456"},
	}
	if got := task.GitHubIssueRefs(); !reflect.DeepEqual(got, want) {
		t.Errorf("GitHubIssueRefs() = %#v, want %#v", got, want)
	}
	if !task.HasGitHubIssue() {
		t.Error("HasGitHubIssue() = false, want true")
	}
}

func TestTaskGitHubIssueRefsNoMatches(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment("https://example.com/acme/roadmap/issues/456", true, "https://example.com/acme/roadmap/issues/456"); err != nil {
		t.Fatal(err)
	}

	if got := task.GitHubIssueRefs(); len(got) != 0 {
		t.Errorf("GitHubIssueRefs() = %#v, want empty", got)
	}
	if task.HasGitHubIssue() {
		t.Error("HasGitHubIssue() = true, want false")
	}
}

func TestTaskGitHubIssueRefsURLAttachmentOriginalPathFallback(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment("roadmap-123", true, "https://github.com/acme/roadmap/issues/123"); err != nil {
		t.Fatal(err)
	}

	want := []GitHubIssueRef{
		{Owner: "acme", Repo: "roadmap", Number: 123, URL: "https://github.com/acme/roadmap/issues/123"},
	}
	if got := task.GitHubIssueRefs(); !reflect.DeepEqual(got, want) {
		t.Errorf("GitHubIssueRefs() = %#v, want %#v", got, want)
	}
	if !task.HasGitHubIssue() {
		t.Error("HasGitHubIssue() = false, want true")
	}
}

func TestTaskGitHubIssueRefsURLAttachmentOriginalPathDeduplicates(t *testing.T) {
	task, err := NewGenericTask("t1", 1, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := task.AddAttachment(
		"https://github.com/acme/roadmap/issues/123",
		true,
		"https://www.github.com/ACME/Roadmap/issues/123?ref=duplicate",
	); err != nil {
		t.Fatal(err)
	}

	want := []GitHubIssueRef{
		{Owner: "acme", Repo: "roadmap", Number: 123, URL: "https://github.com/acme/roadmap/issues/123"},
	}
	if got := task.GitHubIssueRefs(); !reflect.DeepEqual(got, want) {
		t.Errorf("GitHubIssueRefs() = %#v, want %#v", got, want)
	}
}

func TestNilTaskGitHubIssueRefs(t *testing.T) {
	var task *Task
	if got := task.GitHubIssueRefs(); got != nil {
		t.Errorf("GitHubIssueRefs() = %#v, want nil", got)
	}
	if task.HasGitHubIssue() {
		t.Error("HasGitHubIssue() = true, want false")
	}
}
