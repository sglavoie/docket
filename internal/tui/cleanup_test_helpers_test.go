package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/textgrammar"
	"github.com/sglavoie/tt/internal/tui/operation"
)

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
	hints := m.operationRegistry.DecoratedChordHintsForPrefix(
		operation.InputStrategySimplified,
		m.pendingChord,
		ctx,
		m.crossOutliner.decoratePendingChordHint,
	)
	parts := []string{strings.Join(m.pendingChord, "") + "…"}
	for _, hint := range hints {
		parts = append(parts, hint.Key+" "+hint.Label)
	}
	return strings.Join(parts, " · ")
}

func jumpToOutlinerRootForTest(m *AppModel) {
	if m == nil {
		return
	}
	_, _ = m.handleProjectViewMessages(launchOutlinerMsg{})
}
