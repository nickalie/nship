// Package job provides core functionality for defining and executing deployment jobs.
package job

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"

	"github.com/nickalie/nship/internal/core/target"
	"github.com/nickalie/nship/internal/util"
)

// HashStorage defines an interface for storing and retrieving step hashes
type HashStorage interface {
	// SaveHash stores a hash for a job step on a specific target
	SaveHash(targetName, jobName string, stepIndex int, hash string) error

	// GetHash retrieves a hash for a job step on a specific target
	GetHash(targetName, jobName string, stepIndex int) (string, error)

	// Clear removes all stored hashes
	Clear() error
}

// StepHasherInterface defines the interface for hash computation
type StepHasherInterface interface {
	ComputeHash(step *Step, tgt *target.Target) (string, error)
}

// StepHasher handles computing hashes for steps
type StepHasher struct{}

// NewStepHasher creates a new StepHasher
func NewStepHasher() StepHasherInterface {
	return &StepHasher{}
}

// ComputeHash generates a hash for a step based on its configuration
// For CopyStep, it also considers the source files
func (h *StepHasher) ComputeHash(step *Step, tgt *target.Target) (string, error) {
	stepData, err := h.prepareStepData(step, tgt)
	if err != nil {
		return "", fmt.Errorf("prepare step data: %w", err)
	}

	if step.Copy != nil {
		hasher := sha256.New()
		hasher.Write(stepData)
		if err := h.processSourcePath(step.Copy, hasher); err != nil {
			return "", fmt.Errorf("process source path: %w", err)
		}
		return hex.EncodeToString(hasher.Sum(nil)), nil
	}

	sum := sha256.Sum256(stepData)
	return hex.EncodeToString(sum[:]), nil
}

// prepareStepData creates a copy of step data with sorted exclude patterns
func (h *StepHasher) prepareStepData(step *Step, tgt *target.Target) ([]byte, error) {
	if step.Copy != nil && len(step.Copy.Exclude) > 0 {
		stepCopy := *step
		copyStepCopy := *step.Copy
		stepCopy.Copy = &copyStepCopy

		// Sort exclude patterns for consistent hashing
		copyStepCopy.Exclude = make([]string, len(step.Copy.Exclude))
		copy(copyStepCopy.Exclude, step.Copy.Exclude)
		sort.Strings(copyStepCopy.Exclude)

		return json.Marshal(struct {
			Step   *Step
			Target *target.Target
		}{
			Step:   &stepCopy,
			Target: tgt,
		})
	}

	return json.Marshal(struct {
		Step   *Step
		Target *target.Target
	}{
		Step:   step,
		Target: tgt,
	})
}

// processSourcePath handles the source path for copy step hashing
func (h *StepHasher) processSourcePath(copyStep *CopyStep, hasher hash.Hash) error {
	// Use filepath.Abs to resolve relative paths
	localPath, err := filepath.Abs(copyStep.Local)
	if err != nil {
		return fmt.Errorf("resolve absolute path: %w", err)
	}

	// Get file info
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	// Add file/directory info to hash
	addFileInfoToHash(localPath, info, hasher)

	if info.IsDir() {
		// For directories, hash the structure recursively
		if err := h.hashDirectory(localPath, copyStep.Exclude, hasher); err != nil {
			return fmt.Errorf("hash directory: %w", err)
		}
	}

	return nil
}

// hashDirectory recursively hashes a directory's structure and file metadata
func (h *StepHasher) hashDirectory(dir string, exclude []string, hasher hash.Hash) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	// Sort entries for consistent hashing
	entryNames := getSortedEntryNames(entries)

	for _, name := range entryNames {
		path := filepath.Join(dir, name)

		if util.IsExcluded(path, exclude) {
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("stat entry: %w", err)
		}

		addFileInfoToHash(path, info, hasher)

		if info.IsDir() {
			if err := h.hashDirectory(path, exclude, hasher); err != nil {
				return err
			}
		}
	}

	return nil
}

// getSortedEntryNames returns a sorted list of entry names
func getSortedEntryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	sort.Strings(names)
	return names
}

// addFileInfoToHash adds file information to the hash
func addFileInfoToHash(path string, info os.FileInfo, hasher hash.Hash) {
	hasher.Write([]byte(path))
	fmt.Fprintf(hasher, "%d", info.Size())
	fmt.Fprintf(hasher, "%d", info.Mode())
	hasher.Write([]byte(info.ModTime().UTC().String()))
}
