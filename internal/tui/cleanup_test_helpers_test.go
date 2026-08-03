package tui

import "github.com/sglavoie/tt/internal/textgrammar"

func captureDestinationNamesForTest() []string {
	definition, ok := textgrammar.DefinitionFor(textgrammar.SurfaceCapture, textgrammar.FieldDestination)
	if !ok {
		return nil
	}
	return definition.Values
}

func pendingChordHintRowForTest(m *AppModel) string {
	if m == nil || m.operationRegistry == nil || len(m.pendingChord) == 0 {
		return ""
	}
	ctx := m.operationCommandContext()
	ctx.View = m.currentView()
	return m.operationRegistry.ChordHintRowForPrefix(InputStrategySimplified, m.pendingChord, ctx)
}

func jumpToOutlinerRootForTest(m *AppModel) {
	if m == nil {
		return
	}
	_, _ = m.handleProjectViewMessages(launchOutlinerMsg{})
}
