package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
)

func convertLegacyV4TaskFile(path string, location *time.Location) (format.V5TaskConversionResult, error) {
	data, err := readV5DryRunSource(path)
	if err != nil {
		return format.V5TaskConversionResult{}, err
	}
	document, err := format.DecodeLegacyV4TaskDocument(string(data), location)
	if err != nil {
		return format.V5TaskConversionResult{}, err
	}
	return format.ConvertLegacyV4TaskDocument(document, location)
}

func legacyV4ProjectArchives(
	dataDir string,
	projectDir string,
	location *time.Location,
) ([]storage.V5ArchiveTaskSource, string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, "", err
	}
	var archives []storage.V5ArchiveTaskSource
	var indexPath string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(projectDir, entry.Name())
		switch {
		case legacyV4ArchiveFileName.MatchString(entry.Name()):
			conversion, err := convertLegacyV4TaskFile(path, location)
			if err != nil {
				return nil, "", err
			}
			sourceID, err := filepath.Rel(dataDir, path)
			if err != nil {
				return nil, "", err
			}
			archives = append(archives, storage.V5ArchiveTaskSource{
				SourceID:   filepath.ToSlash(sourceID),
				Path:       path,
				Conversion: conversion,
			})
		case entry.Name() == legacyV4ArchiveIndexFileName:
			if _, err := readV5DryRunSource(path); err != nil {
				return nil, "", err
			}
			indexPath = path
		}
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].SourceID < archives[j].SourceID })
	return archives, indexPath, nil
}

func readOptionalV5MigrationFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("inspect v5 migration source %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("v5 migration source is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, fmt.Errorf("read v5 migration source %s: %w", path, err)
	}
	return data, true, nil
}

func reconcileV5MigrationPlanning(
	domains *V5DomainCandidates,
	projects []storage.V5ProjectTaskCandidate,
) error {
	if domains == nil {
		return fmt.Errorf("v5 migration domains are required")
	}
	type taskIdentity struct {
		projectID string
		taskID    string
	}
	states := make(map[taskIdentity]string)
	var nextInDocumentOrder []V5PlanningTaskRef
	for _, project := range projects {
		for _, task := range project.Tasks {
			identity := taskIdentity{projectID: project.ProjectID, taskID: task.ID}
			states[identity] = task.State
			if task.State == "next" {
				nextInDocumentOrder = append(nextInDocumentOrder, V5PlanningTaskRef{
					ProjectID: project.ProjectID,
					TaskID:    task.ID,
				})
			}
		}
	}

	var today []V5PlanningTaskRef
	for _, ref := range domains.Planning.Today {
		state, exists := states[taskIdentity{projectID: ref.ProjectID, taskID: ref.TaskID}]
		if !exists || state == "inbox" || state == "next" || state == "done" || state == "canceled" {
			continue
		}
		today = append(today, ref)
	}
	domains.Planning.Today = today

	seenNext := make(map[taskIdentity]bool)
	var nextOrder []V5PlanningTaskRef
	for _, ref := range domains.Planning.NextOrder {
		identity := taskIdentity{projectID: ref.ProjectID, taskID: ref.TaskID}
		if states[identity] != "next" || seenNext[identity] {
			continue
		}
		seenNext[identity] = true
		nextOrder = append(nextOrder, ref)
	}
	for _, ref := range nextInDocumentOrder {
		identity := taskIdentity{projectID: ref.ProjectID, taskID: ref.TaskID}
		if seenNext[identity] {
			continue
		}
		seenNext[identity] = true
		nextOrder = append(nextOrder, ref)
	}
	domains.Planning.NextOrder = nextOrder
	return domains.Validate()
}
