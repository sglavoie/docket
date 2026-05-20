package models

import "testing"

func TestParseTaskRef(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantProj  string
		wantTask  string
		wantCross bool
	}{
		{
			name:      "simple task ID",
			input:     "abc123",
			wantProj:  "",
			wantTask:  "abc123",
			wantCross: false,
		},
		{
			name:      "UUID task ID",
			input:     "550e8400-e29b-41d4-a716-446655440000",
			wantProj:  "",
			wantTask:  "550e8400-e29b-41d4-a716-446655440000",
			wantCross: false,
		},
		{
			name:      "cross-project reference",
			input:     "my-project:task-123",
			wantProj:  "my-project",
			wantTask:  "task-123",
			wantCross: true,
		},
		{
			name:      "cross-project with UUID",
			input:     "work-tasks:550e8400-e29b-41d4-a716-446655440000",
			wantProj:  "work-tasks",
			wantTask:  "550e8400-e29b-41d4-a716-446655440000",
			wantCross: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantProj:  "",
			wantTask:  "",
			wantCross: false,
		},
		{
			// A trailing colon yields a non-empty ProjectID but an empty TaskID.
			// Such a value is not a usable cross-project reference; treating it
			// as one would cause pruneOrphanedRefs to preserve it forever even
			// though no lookup could ever resolve it.
			name:      "malformed cross-project ref with empty task ID",
			input:     "my-project:",
			wantProj:  "my-project",
			wantTask:  "",
			wantCross: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := ParseTaskRef(tt.input)
			if ref.ProjectID != tt.wantProj {
				t.Errorf("ProjectID = %q, want %q", ref.ProjectID, tt.wantProj)
			}
			if ref.TaskID != tt.wantTask {
				t.Errorf("TaskID = %q, want %q", ref.TaskID, tt.wantTask)
			}
			if ref.IsCrossProject() != tt.wantCross {
				t.Errorf("IsCrossProject() = %v, want %v", ref.IsCrossProject(), tt.wantCross)
			}
		})
	}
}

func TestTaskRef_String(t *testing.T) {
	tests := []struct {
		name string
		ref  TaskRef
		want string
	}{
		{
			name: "local reference",
			ref:  TaskRef{TaskID: "abc123"},
			want: "abc123",
		},
		{
			name: "cross-project reference",
			ref:  TaskRef{ProjectID: "my-project", TaskID: "task-123"},
			want: "my-project:task-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskRef_IsEmpty(t *testing.T) {
	tests := []struct {
		name string
		ref  TaskRef
		want bool
	}{
		{
			name: "empty ref",
			ref:  TaskRef{},
			want: true,
		},
		{
			name: "only project ID",
			ref:  TaskRef{ProjectID: "proj"},
			want: true,
		},
		{
			name: "has task ID",
			ref:  TaskRef{TaskID: "task"},
			want: false,
		},
		{
			name: "cross-project with task ID",
			ref:  TaskRef{ProjectID: "proj", TaskID: "task"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
