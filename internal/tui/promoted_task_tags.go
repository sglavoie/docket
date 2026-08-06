package tui

// promotedTaskTag is a code-owned workflow tag with first-class Outliner
// behavior. It remains an ordinary durable task tag: the registry only adds
// presentation, direct-key, and picker affordances.
type promotedTaskTag struct {
	Tag   string
	Label string
}

const reviewTaskTag = "review"

var promotedTaskTags = []promotedTaskTag{
	{Tag: reviewTaskTag, Label: "Review"},
}
