package tui

func (m *OutlinerModel) HandledKeys() []string {
	if m.operationRegistry == nil {
		return nil
	}
	return m.operationRegistry.HandledKeysForInputStrategy(InputStrategySimplified)
}

func (m *OutlinerModel) GetHelpItems() []HelpItem {
	if m.operationRegistry == nil {
		return nil
	}
	return m.operationRegistry.HelpItemsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
	)
}

func (m *OutlinerModel) ContextualFooter() (StatusBarFooterSpec, bool) {
	if m.operationRegistry == nil {
		return StatusBarFooterSpec{}, false
	}
	hints := fitFooterHintLine(m.operationRegistry.FooterHintPairsForInputStrategy(
		InputStrategySimplified,
		m.operationCommandContext(),
	), m.Width())
	if hints == "" {
		return StatusBarFooterSpec{}, false
	}
	return StatusBarFooterSpec{Left: hints}, true
}
