package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/util"
)

// AttachmentInputKind names the three shapes a pasted attachment value can take.
type AttachmentInputKind int

const (
	// AttachmentInputURL is an absolute http(s) URL or a Mail message: link,
	// stored as metadata only.
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
// A value that names a missing file, or one shaped like a URL that is not an
// absolute http(s) one, is rejected here, so a rejected paste never stages a byte.
func ResolveAttachmentInput(raw string) (ResolvedAttachmentInput, error) {
	input := util.NormalizePastedPathInput(raw)
	if input == "" {
		return ResolvedAttachmentInput{}, fmt.Errorf("path/URL cannot be empty")
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") || IsMessageURL(input) {
		// A prefix is not a URL: the canonical AttachURL validation is what decides
		// here too, so a malformed value is refused before any surface mutates a
		// task rather than persisted for the panel to reject on read.
		validated, err := validateAttachmentURL(input)
		if err != nil {
			return ResolvedAttachmentInput{}, err
		}
		return ResolvedAttachmentInput{Kind: AttachmentInputURL, URL: validated}, nil
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
	// Stat reads metadata alone, so classifying a path never opens the file:
	// a capture preview runs this and must not trigger a download or a
	// privacy prompt. Only the copy at commit reads the bytes.
	info, statErr := os.Stat(absPath)
	if os.IsNotExist(statErr) {
		return ResolvedAttachmentInput{}, fmt.Errorf("file not found: %s", absPath)
	}
	if statErr == nil && !info.Mode().IsRegular() {
		return ResolvedAttachmentInput{}, fmt.Errorf("not a file: %s", absPath)
	}

	return ResolvedAttachmentInput{Kind: AttachmentInputFile, SourcePath: absPath}, nil
}

// ResolveCapturedAttachments classifies every &link value on one capture line
// before the line mutates anything. It is the whole-batch sibling of
// ResolveAttachmentInput and adds the two guards a single paste does not need:
// the destination filename must be a safe child name, and two files on one line
// must not resolve to the same one. Both refuse by name, because the alternative
// is a copy that silently overwrites the first file with the second.
// It reads metadata alone, so a capture preview runs it too and refuses what
// the commit would before Enter is pressed.
func ResolveCapturedAttachments(values []string) ([]ResolvedAttachmentInput, error) {
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
			// OriginalPath carries the URL, matching AttachURL: the panel's
			// original-path action copies that field, so an empty one would copy
			// nothing.
			if err := ref.Task.AddAttachmentAt(input.URL, true, input.URL, now); err != nil {
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
