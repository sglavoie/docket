package cmd

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tterrors"
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

func setupAttachmentTest(t *testing.T) (*testDeps, *models.Project) {

	deps, project := setupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := saveTestRuntime(manager, deps.Config)
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
		if !errors.Is(err, tterrors.ErrRequiredField) {
			t.Errorf("expected ErrRequiredField, got %v", err)
		}
		if !strings.Contains(err.Error(), "attachment index") {
			t.Errorf("expected attachment index in error, got %v", err)
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

	t.Run("rejects extra args", func(t *testing.T) {
		deps, _ := setupAttachmentTest(t)
		defer deps.Cleanup()

		err := runAttachmentOpen(attachmentOpenCmd, []string{"1", "0", "extra"})
		if err == nil {
			t.Error("expected error for extra args")
		}
		if !errors.Is(err, tterrors.ErrRequiredField) {
			t.Errorf("expected ErrRequiredField, got %v", err)
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

func TestAttachmentOpenCmdValidatesArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "missing args", args: nil, wantErr: true},
		{name: "single arg", args: []string{"1"}, wantErr: true},
		{name: "exact args", args: []string{"1", "0"}, wantErr: false},
		{name: "extra args", args: []string{"1", "0", "extra"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := attachmentOpenCmd.ValidateArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateArgs(%v) error = %v, wantErr %v", tc.args, err, tc.wantErr)
			}
		})
	}
}
