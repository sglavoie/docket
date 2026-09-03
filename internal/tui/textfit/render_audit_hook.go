package textfit

// RequiredTextTruncationSink is the optional observation seam render tests
// install to see chrome the layout could not fit. Normal application rendering
// leaves it nil, and nothing in production ever writes it. It is the whole
// exported surface of this file: the root shell's test-only withRenderAudit
// swaps it in, collects what truncateRequired reports, and restores it. Keeping
// the recording side unexported is what stops anything but a real truncation
// from producing a record.
var RequiredTextTruncationSink func(context, full, shown string)

func recordRequiredTextTruncation(context, full, shown string) {
	if RequiredTextTruncationSink != nil {
		RequiredTextTruncationSink(context, full, shown)
	}
}
