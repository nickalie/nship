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
	ComputeHash(step *Step, tgt *target.Target, fs FileSystemInterface) (string, error)
}

// StepHasher handles computing hashes for steps
type StepHasher struct{}

// NewStepHasher creates a new StepHasher
func NewStepHasher() StepHasherInterface {
	return &StepHasher{}
}

// FileSystemInterface is needed for accessing source files
type FileSystemInterface interface {
	Stat(name string) (os.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
}

// ComputeHash generates a hash for a step based on its configuration
// For CopyStep, it also considers the source files
func (h *StepHasher) ComputeHash(step *Step, tgt *target.Target, fs FileSystemInterface) (string, error) {
	// Create a combined structure with step and target info
	type combinedData struct {
		Step   *Step          `json:"step"`
		Target *target.Target `json:"target"`
	}

	combined := combinedData{
		Step:   step,
		Target: tgt,
	}

	// Special handling for CopyStep
	if step.GetType() == CopyStepType && fs != nil {
		return h.computeCopyStepHash(step, tgt, fs)
	}

	// For other steps, hash the combined configuration
	data, err := json.Marshal(combined)
	if err != nil {
		return "", fmt.Errorf("failed to marshal step and target: %w", err)
	}

	// Compute SHA-256
	hashSum := sha256.Sum256(data)
	return hex.EncodeToString(hashSum[:]), nil
}

// prepareStepData creates a copy of step data with sorted exclude patterns
func (h *StepHasher) prepareStepData(step *Step, tgt *target.Target) ([]byte, error) {
	if step.Copy == nil {
		return nil, fmt.Errorf("nil CopyStep")
	}

	// Make a deep copy of the step to ensure we don't modify the original
	stepCopy := *step
	copyStepcopy := *step.Copy

	// Sort exclude patterns if they exist
	if len(copyStepcopy.Exclude) > 0 {
		sortedExcludes := make([]string, len(copyStepcopy.Exclude))
		copy(sortedExcludes, copyStepcopy.Exclude)
		sort.Strings(sortedExcludes)
		copyStepcopy.Exclude = sortedExcludes
	}

	stepCopy.Copy = &copyStepcopy
	combined := struct {
		Step   *Step          `json:"step"`
		Target *target.Target `json:"target"`
	}{
		Step:   &stepCopy,
		Target: tgt,
	}

	return json.Marshal(combined)
}

// processSourcePath handles the source path for copy step hashing
func (h *StepHasher) processSourcePath(copyStep *CopyStep, fs FileSystemInterface, hasher hash.Hash) error {
	localInfo, err := fs.Stat(copyStep.Local)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Source doesn't exist, hash will only include step config
		}
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if localInfo.IsDir() {
		if err := h.hashDirectory(copyStep.Local, copyStep.Exclude, fs, hasher); err != nil {
			return fmt.Errorf("failed to hash directory: %w", err)
		}
	} else {
		addFileInfoToHash(copyStep.Local, localInfo, hasher)
	}

	return nil
}

// computeCopyStepHash generates a hash for a CopyStep that includes source file information
func (h *StepHasher) computeCopyStepHash(step *Step, tgt *target.Target, fs FileSystemInterface) (string, error) {
	stepData, err := h.prepareStepData(step, tgt)
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	hasher.Write(stepData)

	if err := h.processSourcePath(step.Copy, fs, hasher); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// hashDirectory recursively hashes a directory's structure and file metadata
func (h *StepHasher) hashDirectory(dir string, exclude []string, fileSystem FileSystemInterface, hasher hash.Hash) error {
	entries, err := fileSystem.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	entryNames := getSortedEntryNames(entries)

	for _, name := range entryNames {
		path := filepath.Join(dir, name)

		// Use the utility package's IsExcluded function
		if util.IsExcluded(path, name, exclude) {
			continue
		}

		info, err := fileSystem.Stat(path)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", path, err)
		}

		addFileInfoToHash(path, info, hasher)

		if info.IsDir() {
			if err := h.hashDirectory(path, exclude, fileSystem, hasher); err != nil {
				return err
			}
		}
	}

	return nil
}

func getSortedEntryNames(entries []os.DirEntry) []string {
	entryNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryNames = append(entryNames, entry.Name())
	}
	sort.Strings(entryNames)
	return entryNames
}

// Remove the local isExcluded function as we're now using the unified version from fs package

func addFileInfoToHash(path string, info os.FileInfo, hasher hash.Hash) {
	fileData := fmt.Sprintf("%s:%s:%d:%d", path, info.ModTime().String(), info.Size(), info.Mode())
	hasher.Write([]byte(fileData))
}
