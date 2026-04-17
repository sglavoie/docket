package config

import "path/filepath"

// ExportsDir returns the directory where project export files are stored (~/.tt/exports/).
func (m *Manager) ExportsDir() string {
	return filepath.Join(filepath.Dir(m.configPath), "exports")
}

// ExportPath returns the full path for a project export file given a project slug.
func (m *Manager) ExportPath(slug string) string {
	return filepath.Join(m.ExportsDir(), slug+".md")
}
