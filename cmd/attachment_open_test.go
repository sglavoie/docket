package cmd

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestBuildAttachmentEditorCmd verifies that flagged attachment_editor strings
// are split correctly so the flags are not treated as the binary name.
func TestBuildAttachmentEditorCmd(t *testing.T) {
	cases := []struct {
		editor   string
		target   string
		wantExe  string
		wantArgs []string
	}{
		{
			editor:   "vlc",
			target:   "/path/to/file.mp4",
			wantExe:  "vlc",
			wantArgs: []string{"vlc", "/path/to/file.mp4"},
		},
		{
			editor:   "vlc --fullscreen",
			target:   "/path/to/file.mp4",
			wantExe:  "vlc",
			wantArgs: []string{"vlc", "--fullscreen", "/path/to/file.mp4"},
		},
		{
			editor:   "mpv --loop=inf --mute",
			target:   "/path/to/audio.mp3",
			wantExe:  "mpv",
			wantArgs: []string{"mpv", "--loop=inf", "--mute", "/path/to/audio.mp3"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.editor, func(t *testing.T) {
			cmd := buildAttachmentEditorCmd(tc.editor, tc.target)
			// cmd.Args[0] is the resolved binary path; compare base name for portability.
			if len(cmd.Args) != len(tc.wantArgs) {
				t.Fatalf("buildAttachmentEditorCmd(%q, %q) args = %v, want %v",
					tc.editor, tc.target, cmd.Args, tc.wantArgs)
			}
			// args[1:] should match exactly.
			for i := 1; i < len(tc.wantArgs); i++ {
				if cmd.Args[i] != tc.wantArgs[i] {
					t.Errorf("args[%d] = %q, want %q (full args: %v)",
						i, cmd.Args[i], tc.wantArgs[i], cmd.Args)
				}
			}
			// Target must always be the last argument.
			last := cmd.Args[len(cmd.Args)-1]
			if last != tc.target {
				t.Errorf("last arg = %q, want target %q (full args: %v)",
					last, tc.target, cmd.Args)
			}
		})
	}
}

func setupAttachmentTest(t *testing.T) (*testutil.TestDeps, *models.Project) {

	deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := manager.Save(deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	return deps, project
}

func TestRunAttachmentOpen(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		deps, _ := setupAttachmentTest(t)
		defer deps.Cleanup()

		err := runAttachmentOpen(attachmentOpenCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects single arg", func(t *testing.T) {
		deps, _ := setupAttachmentTest(t)
		defer deps.Cleanup()

		err := runAttachmentOpen(attachmentOpenCmd, []string{"1"})
		if err == nil {
			t.Error("expected error for single arg")
		}
	})

	t.Run("task has no attachments", func(t *testing.T) {
		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		taskID := fmt.Sprintf("%d", project.Tasks[0].ShortID)
		err := runAttachmentOpen(attachmentOpenCmd, []string{taskID, "0"})
		if err == nil {
			t.Error("expected error for task with no attachments")
		}
	})

	t.Run("invalid attachment index", func(t *testing.T) {
		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		task := project.Tasks[0]
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename: "test.txt",
		})

		taskID := fmt.Sprintf("%d", task.ShortID)
		err := runAttachmentOpen(attachmentOpenCmd, []string{taskID, "abc"})
		if err == nil {
			t.Error("expected error for invalid attachment index")
		}
	})

	t.Run("attachment index out of range", func(t *testing.T) {
		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		task := project.Tasks[0]
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename: "test.txt",
		})

		taskID := fmt.Sprintf("%d", task.ShortID)
		err := runAttachmentOpen(attachmentOpenCmd, []string{taskID, "5"})
		if err == nil {
			t.Error("expected error for attachment index out of range")
		}
	})
}
