package config

import (
	"path/filepath"

	"github.com/sglavoie/tt/internal/util"
)

// ExportsDir returns the directory where project export files are stored:
// <data>/exports, a sibling of projects/ beside the workspace config.
func (m *Manager) ExportsDir() string {
	return filepath.Join(filepath.Dir(m.configPath), "exports")
}

// ExportPath returns the full path for a project export file given a project slug.
// The slug is a validated project slug; the containment join guarantees the
// result stays inside ExportsDir, falling back to the sanitized base name if a
// malformed slug ever reaches here so an export can never be written outside the
// exports directory.
func (m *Manager) ExportPath(slug string) string {
	if safe, err := util.SafeJoin(m.ExportsDir(), slug+".md"); err == nil {
		return safe
	}
	return filepath.Join(m.ExportsDir(), filepath.Base(slug)+".md")
}
