// Package gotree names the directories a source audit walking the repository
// must skip: exactly the ones the Go tool never compiles.
package gotree

import "strings"

// Ignored reports whether the Go tool ignores a directory with this name:
// dot and underscore directories, plus testdata. A walk skipping these reads
// what `go build ./...` builds and nothing else, which is what keeps it out of
// .git, the git-tracked .tt/ test residue, and tool caches such as the Zig
// cache under macos/.cache. Never add an ordinary directory name here: a
// package could then opt out of every audit by being named, which is the
// failure those audits exist to prevent.
//
// A walk rooted at "." must not ask about its own root, whose name is ".".
func Ignored(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata"
}
