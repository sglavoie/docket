package tui

import (
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for ProjectTreeModel.
func (m *ProjectTreeModel) buildKeyMap() *keymap.KeyMap {
	return &keymap.KeyMap{
		ViewName: "Project Tree",
		Entries:  newProjectTreeKeymapBuilder(m).entries(),
	}
}

type projectTreeKeymapBuilder struct {
	model  *ProjectTreeModel
	stages []string
}

func newProjectTreeKeymapBuilder(m *ProjectTreeModel) *projectTreeKeymapBuilder {
	var stages []string
	if m.config != nil {
		stages = m.config.Stages
	}

	return &projectTreeKeymapBuilder{
		model:  m,
		stages: stages,
	}
}

func (b *projectTreeKeymapBuilder) entries() []keymap.Entry {
	var entries []keymap.Entry

	entries = append(entries, b.primaryViewEntries()...)
	entries = append(entries, b.navigationEntries()...)
	entries = append(entries, b.gotoEntries()...)
	entries = append(entries, b.foldEntries()...)
	entries = append(entries, b.statusEntries()...)
	entries = append(entries, b.priorityEntries()...)
	entries = append(entries, b.dueDateEntries()...)
	entries = append(entries, b.snoozeEntries()...)
	entries = append(entries, b.moveEntries()...)
	entries = append(entries, b.linkedTaskEntries()...)
	entries = append(entries, b.taskPrefixEntries()...)
	entries = append(entries, b.stageEntries()...)
	entries = append(entries, b.blockerEntries()...)
	entries = append(entries, b.filterEntries()...)
	entries = append(entries, b.typeChangeEntries()...)
	entries = append(entries, b.selectionEntries()...)
	entries = append(entries, b.movementEntries()...)
	entries = append(entries, b.commonTaskActionEntries()...)
	entries = append(entries, b.actionEntries()...)
	entries = append(entries, b.sortEntries()...)

	return entries
}

func (b *projectTreeKeymapBuilder) visible() []*TreeNode {
	return b.model.visibleNodes()
}

func (b *projectTreeKeymapBuilder) currentNode() *TreeNode {
	visible := b.visible()
	if CursorInBounds(b.model.cursor, len(visible)) {
		return visible[b.model.cursor]
	}

	return nil
}

func (b *projectTreeKeymapBuilder) cursorTask() *models.Task {
	node := b.currentNode()
	if node == nil {
		return nil
	}

	return node.Task
}

func (b *projectTreeKeymapBuilder) hasTask() bool {
	return b.cursorTask() != nil
}

func (b *projectTreeKeymapBuilder) gateOnTask(entries []keymap.Entry) []keymap.Entry {
	if len(entries) > 0 && entries[0].PrefixNode != nil {
		entries[0].PrefixNode.Gate = b.hasTask
	}

	return entries
}
