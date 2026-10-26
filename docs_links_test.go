package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// Relative Markdown links are checked the way GitHub renders them: the target
// file must exist and a #fragment must name a heading in it. A rename that
// moves a heading or a file otherwise leaves a link pointing nowhere, and no
// content assertion would notice.

var (
	markdownLinkPattern = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	markdownHeading     = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)
	markdownFence       = regexp.MustCompile("^\\s*(```|~~~)")
	markdownInlineCode  = regexp.MustCompile("`[^`]*`")
)

func TestCurrentProductDocsLinksResolve(t *testing.T) {
	var problems []string
	anchors := map[string]map[string]bool{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != "." && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "DerivedData" || name == "build" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		for _, link := range markdownLinks(t, path) {
			if problem := checkMarkdownLink(path, link, anchors); problem != "" {
				problems = append(problems, problem)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) > 0 {
		t.Fatalf("%d broken relative links:\n%s", len(problems), strings.Join(problems, "\n"))
	}
}

// markdownLinks returns the link targets in path, skipping fenced blocks and
// inline code, where a bracket pair is text rather than a link.
func markdownLinks(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var links []string
	fenced := false
	for _, line := range strings.Split(string(data), "\n") {
		if markdownFence.MatchString(line) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		line = markdownInlineCode.ReplaceAllString(line, "")
		for _, match := range markdownLinkPattern.FindAllStringSubmatch(line, -1) {
			links = append(links, match[1])
		}
	}
	return links
}

func checkMarkdownLink(from, link string, anchors map[string]map[string]bool) string {
	if strings.Contains(link, "://") || strings.HasPrefix(link, "mailto:") {
		return ""
	}
	target, fragment, _ := strings.Cut(link, "#")
	resolved := from
	if target != "" {
		resolved = filepath.Join(filepath.Dir(from), filepath.FromSlash(target))
		if _, err := os.Stat(resolved); err != nil {
			return fmt.Sprintf("%s: %s: target does not exist", from, link)
		}
	}
	if fragment == "" || filepath.Ext(resolved) != ".md" {
		return ""
	}
	known, ok := anchors[resolved]
	if !ok {
		known = markdownAnchors(resolved)
		anchors[resolved] = known
	}
	if !known[strings.ToLower(fragment)] {
		return fmt.Sprintf("%s: %s: no heading for #%s", from, link, fragment)
	}
	return ""
}

// markdownAnchors is every heading slug in path, with GitHub's -N suffix for
// a repeated heading and any explicit <a id> or name anchor.
func markdownAnchors(path string) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]bool{}
	}
	known := map[string]bool{}
	seen := map[string]int{}
	explicit := regexp.MustCompile(`<a\s+(?:id|name)="([^"]+)"`)
	fenced := false
	for _, line := range strings.Split(string(data), "\n") {
		if markdownFence.MatchString(line) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		for _, match := range explicit.FindAllStringSubmatch(line, -1) {
			known[strings.ToLower(match[1])] = true
		}
		heading := markdownHeading.FindStringSubmatch(line)
		if heading == nil {
			continue
		}
		slug := githubHeadingSlug(heading[1])
		if n := seen[slug]; n > 0 {
			known[fmt.Sprintf("%s-%d", slug, n)] = true
		} else {
			known[slug] = true
		}
		seen[slug]++
	}
	return known
}

// githubHeadingSlug lowercases a heading, drops everything but letters,
// digits, spaces, hyphens, and underscores, and turns each space into a hyphen.
func githubHeadingSlug(heading string) string {
	heading = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(heading, "$1")
	var b strings.Builder
	for _, r := range strings.ToLower(heading) {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}
