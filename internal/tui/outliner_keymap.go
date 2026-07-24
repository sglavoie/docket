package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for OutlinerModel.
func (m *OutlinerModel) buildKeyMap() *keymap.KeyMap {
	return &keymap.KeyMap{
		ViewName: "Outliner",
		Entries:  newOutlinerKeymapBuilder(m).entries(),
	}
}

type outlinerKeymapBuilder struct {
	model *OutlinerModel
}

func newOutlinerKeymapBuilder(m *OutlinerModel) *outlinerKeymapBuilder {
	return &outlinerKeymapBuilder{
		model: m,
	}
}

func (b *outlinerKeymapBuilder) entries() []keymap.Entry {
	var entries []keymap.Entry

	entries = append(entries, b.navigationEntries()...)
	entries = append(entries, b.primaryViewEntries()...)
	entries = append(entries, b.gotoEntries()...)
	entries = append(entries, b.modeEntries()...)
	entries = append(entries, b.foldEntries()...)
	entries = append(entries, b.statusEntries()...)
	entries = append(entries, b.priorityEntries()...)
	entries = append(entries, b.dueDateEntries()...)
	entries = append(entries, b.snoozeEntries()...)
	entries = append(entries, b.moveEntries()...)
	entries = append(entries, b.linkedTaskEntries()...)
	entries = append(entries, b.taskPrefixEntries()...)
	entries = append(entries, b.blockerEntries()...)
	entries = append(entries, b.filterEntries()...)
	entries = append(entries, b.sortEntries()...)
	entries = append(entries, b.typeChangeEntries()...)
	entries = append(entries, b.perspectiveCycleEntries()...)
	entries = append(entries, b.sectionNavigationEntries()...)
	entries = append(entries, b.actionEntries()...)
	entries = append(entries, b.movementEntries()...)
	entries = append(entries, b.commonTaskActionEntries()...)
	entries = append(entries, b.taskViewEntries()...)
	entries = append(entries, b.utilityEntries()...)

	return entries
}

func (b *outlinerKeymapBuilder) currentRow() *outlinerRow {
	m := b.model
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}

	return &m.rows[m.cursor]
}

func (b *outlinerKeymapBuilder) cursorTask() *models.Task {
	row := b.currentRow()
	if row == nil || !row.isTaskRow() {
		return nil
	}

	return row.task
}

func (b *outlinerKeymapBuilder) hasTask() bool {
	return b.cursorTask() != nil
}

func (b *outlinerKeymapBuilder) hasTaskActionTarget() bool {
	return b.hasTask() || len(b.model.getSelectedTaskProjectRefs()) > 0
}

func (b *outlinerKeymapBuilder) onHeader() bool {
	row := b.currentRow()
	return row != nil && !row.isTaskRow()
}

func (b *outlinerKeymapBuilder) gateOnTask(entries []keymap.Entry) []keymap.Entry {
	if len(entries) > 0 && entries[0].PrefixNode != nil {
		entries[0].PrefixNode.Gate = b.hasTaskActionTarget
	}

	return entries
}

// handleOutlinerFilterSubKey handles filter sub-prefix keys for the outliner.
func (m *OutlinerModel) handleOutlinerFilterSubKey(prefix, key string) (tea.Model, tea.Cmd) {
	if dispatchFilterSubPrefix(prefix, key, &m.filter.CommonFilterState, m.runtime.Machine(), func(p string) {
		if m.Dispatcher != nil {
			m.Dispatcher.SetPendingPrefix(p)
		} else {
			m.pendingPrefix = p
		}
	}) {
		m.rebuildRows()
	}
	return m, nil
}
