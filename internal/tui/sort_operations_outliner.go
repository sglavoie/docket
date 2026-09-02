package tui

import (
	"slices"
)

// manualOrderSectionNames names the sections the freshly rebuilt projection
// holds that the sort deliberately skipped. It reads the rows rather than the
// mode catalog, so the status executeSortOperation reports can never promise an
// exemption the screen does not show.
func (m *OutlinerModel) manualOrderSectionNames() []string {
	if m == nil {
		return nil
	}
	names := make([]string, 0, 4)
	for _, row := range m.rows {
		if !row.IsSectionHeader || !rowKeepsManualOrder(row) || row.SectionName == "" {
			continue
		}
		if !slices.Contains(names, row.SectionName) {
			names = append(names, row.SectionName)
		}
	}
	return names
}
