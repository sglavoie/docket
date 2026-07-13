package util

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Path-containment sentinel errors. Callers use errors.Is to classify a
// rejection without matching on message text.
var (
	// ErrPathEmpty is returned for an empty root or child reference.
	ErrPathEmpty = errors.New("path is empty")
	// ErrPathAbsolute is returned when a child reference is an absolute path
	// (Unix "/...", a Windows drive volume "C:...", or a UNC "\..." prefix).
	ErrPathAbsolute = errors.New("absolute paths are not allowed")
	// ErrPathSeparator is returned when a single-component name contains a path
	// separator, or a joined reference has an empty (leading/doubled) component.
	ErrPathSeparator = errors.New("path separators are not allowed")
	// ErrPathTraversal is returned for a "." or ".." dot component.
	ErrPathTraversal = errors.New("path traversal is not allowed")
	// ErrPathEscape is returned when a reference resolves outside its root,
	// either lexically or after resolving existing symlinks.
	ErrPathEscape = errors.New("path escapes its containing directory")
)

// ValidateChildName reports whether name is a safe single path component: a bare
// file or directory name that is a direct child of some directory. It rejects
// empty names, absolute paths, "." and ".." dot components, and both Unix ("/")
// and Windows ("\") separators. Backslash is rejected on every OS so a name
// crafted on one platform cannot smuggle traversal onto another. The check is
// purely lexical and performs no I/O.
func ValidateChildName(name string) error {
	switch {
	case name == "":
		return ErrPathEmpty
	case filepath.IsAbs(name) || hasWindowsVolume(name):
		return fmt.Errorf("%q: %w", name, ErrPathAbsolute)
	case name == "." || name == "..":
		return fmt.Errorf("%q: %w", name, ErrPathTraversal)
	case strings.ContainsRune(name, '/') || strings.ContainsRune(name, '\\'):
		return fmt.Errorf("%q: %w", name, ErrPathSeparator)
	default:
		return nil
	}
}

// SafeJoin joins rel beneath root and guarantees the result stays inside root.
// rel may name a nested descendant (multiple components) but must not be
// absolute, must not contain "." or ".." components, and — after resolving any
// symlinks that already exist along the path — must not escape root. Backslash
// is treated as a separator on every OS so Windows-style traversal ("..\..")
// cannot slip past a Unix-only cleaner. It returns the cleaned, contained path.
func SafeJoin(root, rel string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("root: %w", ErrPathEmpty)
	}
	if rel == "" {
		return "", ErrPathEmpty
	}
	if filepath.IsAbs(rel) || hasWindowsVolume(rel) {
		return "", fmt.Errorf("%q: %w", rel, ErrPathAbsolute)
	}

	// Treat both separators uniformly so a crafted "..\.." is caught on Unix,
	// where filepath.Clean would otherwise keep the backslash as a filename byte.
	unified := strings.ReplaceAll(rel, "\\", "/")
	for _, part := range strings.Split(unified, "/") {
		switch part {
		case "":
			// Leading, trailing, or doubled separator: reject a malformed ref
			// rather than silently normalizing it away.
			return "", fmt.Errorf("%q: %w", rel, ErrPathSeparator)
		case ".", "..":
			return "", fmt.Errorf("%q: %w", rel, ErrPathTraversal)
		}
	}

	target := filepath.Join(root, filepath.FromSlash(unified))
	if !withinRoot(root, target) {
		return "", fmt.Errorf("%q: %w", rel, ErrPathEscape)
	}
	if err := symlinkContained(root, target); err != nil {
		return "", err
	}
	return target, nil
}

// PathWithinRoot reports whether target resolves inside root (or equals it).
// Unlike SafeJoin, which joins a relative reference beneath a root, this accepts
// an already-joined destination — typically an absolute path a caller intends to
// write, move, or delete — and validates it against an approved root before any
// filesystem mutation. It applies the same lexical and existing-symlink
// containment rules SafeJoin uses, so a target that escapes root either lexically
// (via "..") or through an existing symlink is rejected. An empty root or target
// reports false. The check performs read-only I/O to resolve existing symlinks.
func PathWithinRoot(root, target string) bool {
	if root == "" || target == "" {
		return false
	}
	if !withinRoot(root, target) {
		return false
	}
	return symlinkContained(root, target) == nil
}

// hasWindowsVolume reports whether p begins with a Windows drive-letter volume
// (e.g. "C:") or a UNC-style leading backslash. filepath.IsAbs recognizes these
// only on Windows, so we detect them explicitly to stay safe on Unix hosts.
func hasWindowsVolume(p string) bool {
	if len(p) >= 2 && p[1] == ':' {
		if c := p[0]; (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return strings.HasPrefix(p, `\`)
}

// withinRoot reports whether target is lexically inside root (or equal to it).
func withinRoot(root, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// symlinkContained verifies that resolving existing symlinks on target keeps it
// inside root. Non-existent leaf components are fine — the file is about to be
// created; only symlinks that already exist and escape root are rejected. Both
// root and target are resolved through the same evaluator so a symlinked root
// (e.g. macOS /var -> /private/var) does not produce a false positive.
func symlinkContained(root, target string) error {
	realRoot := resolveExisting(root)
	realTarget := resolveExisting(target)
	if !withinRoot(realRoot, realTarget) {
		return fmt.Errorf("%q: %w", target, ErrPathEscape)
	}
	return nil
}

// resolveExisting resolves symlinks on the longest existing prefix of p and
// rejoins the not-yet-existing remainder unchanged. It never fails: when no
// prefix resolves it returns p cleaned.
func resolveExisting(p string) string {
	cur := filepath.Clean(p)
	var rest []string
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			for i := len(rest) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, rest[i])
			}
			return resolved
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return filepath.Clean(p)
		}
		rest = append(rest, filepath.Base(cur))
		cur = parent
	}
}
