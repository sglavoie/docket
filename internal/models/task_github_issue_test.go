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
			want: GitHubIssueRef{Owner: "acme", Repo: "roadmap", Number: 123},
			ok:   true,
		},
		{
			name: "www host with trailing slash",
			raw:  "https://www.github.com/acme/roadmap/issues/123/",
			want: GitHubIssueRef{Owner: "acme", Repo: "roadmap", Number: 123},
			ok:   true,
		},
		{
			name: "http URL with query and fragment",
			raw:  "http://github.com/Acme/Roadmap/issues/456?notification_referrer_id=abc#issuecomment-1",
			want: GitHubIssueRef{Owner: "Acme", Repo: "Roadmap", Number: 456},
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
