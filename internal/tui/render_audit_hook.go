package tui

// requiredTextTruncationSink is an optional, package-private observation seam
// used by render tests. Normal application rendering leaves it nil.
var requiredTextTruncationSink func(context, full, shown string)

func recordRequiredTextTruncation(context, full, shown string) {
	if requiredTextTruncationSink != nil {
		requiredTextTruncationSink(context, full, shown)
	}
}
