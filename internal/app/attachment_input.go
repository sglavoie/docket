package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// AttachmentInputKind names the three shapes a pasted attachment value can take.
type AttachmentInputKind int

const (
	// AttachmentInputURL is an absolute http(s) URL, stored as metadata only.
	AttachmentInputURL AttachmentInputKind = iota
	// AttachmentInputCodeReference is a file:line reference, stored as metadata only.
	AttachmentInputCodeReference
	// AttachmentInputFile is a local file whose bytes are copied into the task.
	AttachmentInputFile
)

// ResolvedAttachmentInput is the classification of one pasted attachment value:
// which of the three attach paths it belongs to, plus the already-resolved value
// that path needs. It carries no presentation state, so every surface that lets a
// user paste an attachment classifies identically.
type ResolvedAttachmentInput struct {
	// Kind selects the attach path.
	Kind AttachmentInputKind
	// URL is set for AttachmentInputURL.
	URL string
	// CodePath and CodeLine are set for AttachmentInputCodeReference.
	CodePath string
	CodeLine int
	// SourcePath is the ~-expanded absolute path set for AttachmentInputFile.
	SourcePath string
	// SourceMissing reports a code reference whose file is not on disk. It is not
	// an error — a code reference to a not-yet-created file is legal — so the
	// caller decides whether to say so.
	SourceMissing bool
}

// ResolveAttachmentInput classifies a pasted attachment value across the URL,
// code-reference, and file paths, normalizing the paste and resolving a file path
// (~-expansion, then absolute, then existence) before any caller mutates anything.
// A value that names a missing file is rejected here, so a rejected paste never
// stages a byte.
func ResolveAttachmentInput(raw string) (ResolvedAttachmentInput, error) {
	input := util.NormalizePastedPathInput(raw)
	if input == "" {
		return ResolvedAttachmentInput{}, fmt.Errorf("path/URL cannot be empty")
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return ResolvedAttachmentInput{Kind: AttachmentInputURL, URL: input}, nil
	}

	if codePath, lineNum, isCodeRef := storage.ParseCodeReference(input); isCodeRef {
		return ResolvedAttachmentInput{
			Kind:          AttachmentInputCodeReference,
			CodePath:      codePath,
			CodeLine:      lineNum,
			SourceMissing: !storage.CodeReferenceFileExists(codePath),
		}, nil
	}

	path := input
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ResolvedAttachmentInput{}, fmt.Errorf("failed to expand home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return ResolvedAttachmentInput{}, fmt.Errorf("failed to resolve path: %w", err)
	}
	if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
		return ResolvedAttachmentInput{}, fmt.Errorf("file not found: %s", absPath)
	}

	return ResolvedAttachmentInput{Kind: AttachmentInputFile, SourcePath: absPath}, nil
}

// resolveCapturedAttachments classifies every &link value on one capture line
// before the line mutates anything. It is the whole-batch sibling of
// ResolveAttachmentInput and adds the two guards a single paste does not need:
// the destination filename must be a safe child name, and two files on one line
// must not resolve to the same one. Both refuse by name, because the alternative
// is a copy that silently overwrites the first file with the second.
func resolveCapturedAttachments(values []string) ([]ResolvedAttachmentInput, error) {
	if len(values) == 0 {
		return nil, nil
	}
	resolved := make([]ResolvedAttachmentInput, 0, len(values))
	claimed := make(map[string]string, len(values))
	for _, value := range values {
		input, err := ResolveAttachmentInput(value)
		if err != nil {
			return nil, invalidInput("attachment", fmt.Sprintf("%s: %s", value, err))
		}
		if input.Kind == AttachmentInputFile {
			filename := filepath.Base(input.SourcePath)
			if nameErr := storage.SafeChildName(filename); nameErr != nil {
				return nil, invalidInput("attachment", fmt.Sprintf("%s: %s", value, nameErr))
			}
			if first, taken := claimed[filename]; taken {
				return nil, invalidInput("attachment", fmt.Sprintf(
					"%s and %s would both be stored as %s", first, value, filename))
			}
			claimed[filename] = value
		}
		resolved = append(resolved, input)
	}
	return resolved, nil
}

// applyCapturedAttachments records each resolved attachment on the task and
// returns the copy side effects the caller hands to the same ChangeSet as the
// task write. Metadata is appended before any op is staged, the attachFile order,
// so a rejected attachment aborts the capture without staging a byte.
func applyCapturedAttachments(ref TaskRef, inputs []ResolvedAttachmentInput, now time.Time) ([]storage.AttachmentOp, error) {
	var ops []storage.AttachmentOp
	for _, input := range inputs {
		switch input.Kind {
		case AttachmentInputURL:
			if err := ref.Task.AddAttachmentAt(input.URL, true, "", now); err != nil {
				return nil, err
			}
		case AttachmentInputCodeReference:
			if err := ref.Task.AddCodeReferenceAt(input.CodePath, input.CodeLine, now); err != nil {
				return nil, err
			}
		case AttachmentInputFile:
			filename := filepath.Base(input.SourcePath)
			if err := ref.Task.AddAttachmentAt(filename, false, input.SourcePath, now); err != nil {
				return nil, err
			}
			ops = append(ops, storage.AttachmentOp{
				Kind:        storage.AttachmentCopy,
				Task:        ref.Task,
				DestProject: ref.Project,
				SourcePath:  input.SourcePath,
				Filename:    filename,
			})
		}
	}
	return ops, nil
}
