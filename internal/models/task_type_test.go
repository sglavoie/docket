package models

import "testing"

func TestTaskTypeValid(t *testing.T) {
	tests := []struct {
		taskType TaskType
		valid    bool
	}{
		{TaskTypeEpic, true},
		{TaskTypeStory, true},
		{TaskTypeTask, true},
		{TaskTypeBug, true},
		{TaskTypeIdea, true},
		{TaskTypeAction, true},
		{TaskType("invalid"), false},
		{TaskType(""), false},
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.IsValid() != tc.valid {
				t.Errorf("%s.IsValid() = %v, want %v", tc.taskType, tc.taskType.IsValid(), tc.valid)
			}
		})
	}
}

func TestTaskTypeAllowedChildren(t *testing.T) {
	tests := []struct {
		taskType TaskType
		expected []TaskType
	}{
		{TaskTypeEpic, []TaskType{TaskTypeStory, TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction}},
		{TaskTypeStory, []TaskType{TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction}},
		{TaskTypeTask, []TaskType{}},
		{TaskTypeBug, []TaskType{}},
		{TaskTypeIdea, []TaskType{}},
		{TaskTypeAction, []TaskType{}},
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			allowed := tc.taskType.AllowedChildren()
			if len(allowed) != len(tc.expected) {
				t.Errorf("%s.AllowedChildren() has %d items, want %d", tc.taskType, len(allowed), len(tc.expected))
				return
			}
			for i, a := range allowed {
				if a != tc.expected[i] {
					t.Errorf("%s.AllowedChildren()[%d] = %s, want %s", tc.taskType, i, a, tc.expected[i])
				}
			}
		})
	}
}

func TestTaskTypeCanHaveChildren(t *testing.T) {
	tests := []struct {
		taskType TaskType
		expected bool
	}{
		{TaskTypeEpic, true},
		{TaskTypeStory, true},
		{TaskTypeTask, false},
		{TaskTypeBug, false},
		{TaskTypeIdea, false},
		{TaskTypeAction, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.CanHaveChildren() != tc.expected {
				t.Errorf("%s.CanHaveChildren() = %v, want %v", tc.taskType, tc.taskType.CanHaveChildren(), tc.expected)
			}
		})
	}
}

func TestTaskTypeCanBeChildOf(t *testing.T) {
	tests := []struct {
		child  TaskType
		parent TaskType
		can    bool
	}{
		// Epic cannot be child of anything
		{TaskTypeEpic, TaskTypeEpic, false},
		{TaskTypeEpic, TaskTypeStory, false},
		{TaskTypeEpic, TaskTypeTask, false},

		// Story can only be child of Epic
		{TaskTypeStory, TaskTypeEpic, true},
		{TaskTypeStory, TaskTypeStory, false},
		{TaskTypeStory, TaskTypeTask, false},

		// Task can be child of Epic or Story
		{TaskTypeTask, TaskTypeEpic, true},
		{TaskTypeTask, TaskTypeStory, true},
		{TaskTypeTask, TaskTypeTask, false},
		{TaskTypeTask, TaskTypeBug, false},

		// Bug can be child of Epic or Story
		{TaskTypeBug, TaskTypeEpic, true},
		{TaskTypeBug, TaskTypeStory, true},
		{TaskTypeBug, TaskTypeTask, false},

		// Idea can be child of Epic or Story
		{TaskTypeIdea, TaskTypeEpic, true},
		{TaskTypeIdea, TaskTypeStory, true},
		{TaskTypeIdea, TaskTypeTask, false},

		// Action can be child of Epic or Story
		{TaskTypeAction, TaskTypeEpic, true},
		{TaskTypeAction, TaskTypeStory, true},
		{TaskTypeAction, TaskTypeTask, false},
		{TaskTypeAction, TaskTypeAction, false},
	}

	for _, tc := range tests {
		name := string(tc.child) + "_under_" + string(tc.parent)
		t.Run(name, func(t *testing.T) {
			if tc.child.CanBeChildOf(tc.parent) != tc.can {
				t.Errorf("%s.CanBeChildOf(%s) = %v, want %v", tc.child, tc.parent, tc.child.CanBeChildOf(tc.parent), tc.can)
			}
		})
	}
}

func TestTaskTypeIcon(t *testing.T) {
	tests := []struct {
		taskType TaskType
		icon     string
	}{
		{TaskTypeEpic, "E"},
		{TaskTypeStory, "S"},
		{TaskTypeTask, "T"},
		{TaskTypeBug, "B"},
		{TaskTypeIdea, "I"},
		{TaskTypeAction, "A"},
		{TaskType("invalid"), "?"},
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.Icon() != tc.icon {
				t.Errorf("%s.Icon() = %s, want %s", tc.taskType, tc.taskType.Icon(), tc.icon)
			}
		})
	}
}

func TestTaskTypeHeadingPrefix(t *testing.T) {
	tests := []struct {
		taskType TaskType
		prefix   string
	}{
		{TaskTypeEpic, "Epic"},
		{TaskTypeStory, "Story"},
		{TaskTypeTask, "Task"},
		{TaskTypeBug, "Bug"},
		{TaskTypeIdea, "Idea"},
		{TaskTypeAction, "Action"},
		{TaskType("invalid"), "Task"},
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.HeadingPrefix() != tc.prefix {
				t.Errorf("%s.HeadingPrefix() = %s, want %s", tc.taskType, tc.taskType.HeadingPrefix(), tc.prefix)
			}
		})
	}
}

func TestParseTaskType(t *testing.T) {
	tests := []struct {
		input string
		want  TaskType
		valid bool
	}{
		{"epic", TaskTypeEpic, true},
		{"story", TaskTypeStory, true},
		{"task", TaskTypeTask, true},
		{"bug", TaskTypeBug, true},
		{"idea", TaskTypeIdea, true},
		{"action", TaskTypeAction, true},
		{"invalid", TaskType("invalid"), false},
		{"", TaskType(""), false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, valid := ParseTaskType(tc.input)
			if got != tc.want {
				t.Errorf("ParseTaskType(%s) = %s, want %s", tc.input, got, tc.want)
			}
			if valid != tc.valid {
				t.Errorf("ParseTaskType(%s) valid = %v, want %v", tc.input, valid, tc.valid)
			}
		})
	}
}

func TestAllTaskTypes(t *testing.T) {
	types := AllTaskTypes()
	expected := []TaskType{TaskTypeEpic, TaskTypeStory, TaskTypeTask, TaskTypeBug, TaskTypeIdea, TaskTypeAction}

	if len(types) != len(expected) {
		t.Errorf("AllTaskTypes() has %d items, want %d", len(types), len(expected))
		return
	}

	for i, tt := range types {
		if tt != expected[i] {
			t.Errorf("AllTaskTypes()[%d] = %s, want %s", i, tt, expected[i])
		}
	}
}

func TestTaskTypeIsLeafType(t *testing.T) {
	tests := []struct {
		taskType TaskType
		isLeaf   bool
	}{
		{TaskTypeEpic, false},
		{TaskTypeStory, false},
		{TaskTypeTask, true},
		{TaskTypeBug, true},
		{TaskTypeIdea, true},
		{TaskTypeAction, true},
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.IsLeafType() != tc.isLeaf {
				t.Errorf("%s.IsLeafType() = %v, want %v", tc.taskType, tc.taskType.IsLeafType(), tc.isLeaf)
			}
		})
	}
}

func TestTaskTypePromote(t *testing.T) {
	tests := []struct {
		taskType TaskType
		promoted TaskType
	}{
		{TaskTypeEpic, TaskTypeEpic},    // Already at top, stays Epic
		{TaskTypeStory, TaskTypeEpic},   // Story promotes to Epic
		{TaskTypeTask, TaskTypeStory},   // Task promotes to Story
		{TaskTypeBug, TaskTypeStory},    // Bug promotes to Story
		{TaskTypeIdea, TaskTypeStory},   // Idea promotes to Story
		{TaskTypeAction, TaskTypeStory}, // Action promotes to Story
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.Promote() != tc.promoted {
				t.Errorf("%s.Promote() = %s, want %s", tc.taskType, tc.taskType.Promote(), tc.promoted)
			}
		})
	}
}

func TestTaskTypeDemote(t *testing.T) {
	tests := []struct {
		taskType TaskType
		demoted  TaskType
	}{
		{TaskTypeEpic, TaskTypeStory},    // Epic demotes to Story
		{TaskTypeStory, TaskTypeTask},    // Story demotes to Task (default leaf)
		{TaskTypeTask, TaskTypeTask},     // Already leaf, stays Task
		{TaskTypeBug, TaskTypeBug},       // Already leaf, stays Bug
		{TaskTypeIdea, TaskTypeIdea},     // Already leaf, stays Idea
		{TaskTypeAction, TaskTypeAction}, // Already leaf, stays Action
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.Demote() != tc.demoted {
				t.Errorf("%s.Demote() = %s, want %s", tc.taskType, tc.taskType.Demote(), tc.demoted)
			}
		})
	}
}

func TestTaskTypeCycleLeafLeft(t *testing.T) {
	tests := []struct {
		taskType TaskType
		cycled   TaskType
	}{
		{TaskTypeTask, TaskTypeIdea},     // Task cycles left to Idea
		{TaskTypeBug, TaskTypeTask},      // Bug cycles left to Task
		{TaskTypeIdea, TaskTypeBug},      // Idea cycles left to Bug
		{TaskTypeEpic, TaskTypeEpic},     // Non-leaf stays same
		{TaskTypeStory, TaskTypeStory},   // Non-leaf stays same
		{TaskTypeAction, TaskTypeAction}, // Action doesn't cycle
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.CycleLeafLeft() != tc.cycled {
				t.Errorf("%s.CycleLeafLeft() = %s, want %s", tc.taskType, tc.taskType.CycleLeafLeft(), tc.cycled)
			}
		})
	}
}

func TestTaskTypeCycleLeafRight(t *testing.T) {
	tests := []struct {
		taskType TaskType
		cycled   TaskType
	}{
		{TaskTypeTask, TaskTypeBug},      // Task cycles right to Bug
		{TaskTypeBug, TaskTypeIdea},      // Bug cycles right to Idea
		{TaskTypeIdea, TaskTypeTask},     // Idea cycles right to Task
		{TaskTypeEpic, TaskTypeEpic},     // Non-leaf stays same
		{TaskTypeStory, TaskTypeStory},   // Non-leaf stays same
		{TaskTypeAction, TaskTypeAction}, // Action doesn't cycle
	}

	for _, tc := range tests {
		t.Run(string(tc.taskType), func(t *testing.T) {
			if tc.taskType.CycleLeafRight() != tc.cycled {
				t.Errorf("%s.CycleLeafRight() = %s, want %s", tc.taskType, tc.taskType.CycleLeafRight(), tc.cycled)
			}
		})
	}
}
