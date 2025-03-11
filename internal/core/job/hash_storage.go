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

// computeCopyStepHash generates a hash for a CopyStep that includes source file information
func (h *StepHasher) computeCopyStepHash(step *Step, tgt *target.Target, fs FileSystemInterface) (string, error) {
	copyStep := step.Copy
	if copyStep == nil {
		return "", fmt.Errorf("nil CopyStep")
	}

	// Create a combined structure with step and target info
	type combinedData struct {
		Step   *Step          `json:"step"`
		Target *target.Target `json:"target"`
	}

	combined := combinedData{
		Step:   step,
		Target: tgt,
	}

	// Marshal the combined data
	stepData, err := json.Marshal(combined)
	if err != nil {
		return "", fmt.Errorf("failed to marshal step and target: %w", err)
	}

	// Create a hasher that we'll update with all the relevant data
	hasher := sha256.New()

	// Add the step configuration
	hasher.Write(stepData)

	// Get source info
	localInfo, err := fs.Stat(copyStep.Local)
	if err != nil {
		if os.IsNotExist(err) {
			// If source doesn't exist, just use the step config
			hashSum := sha256.Sum256(stepData)
			return hex.EncodeToString(hashSum[:]), nil
		}
		return "", fmt.Errorf("failed to stat source: %w", err)
	}

	// If source is a directory, process all files in it
	if localInfo.IsDir() {
		err = h.hashDirectory(copyStep.Local, copyStep.Exclude, fs, hasher)
		if err != nil {
			return "", fmt.Errorf("failed to hash directory: %w", err)
		}
	} else {
		// For a single file, add its modification time and size
		fileData := fmt.Sprintf("%s:%d", localInfo.ModTime().String(), localInfo.Size())
		hasher.Write([]byte(fileData))
	}

	// Return the final hash
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// hashDirectory recursively hashes a directory's structure and file metadata
func (h *StepHasher) hashDirectory(dir string, exclude []string, fs FileSystemInterface, hasher hash.Hash) error {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	entryNames := getSortedEntryNames(entries)

	for _, name := range entryNames {
		path := filepath.Join(dir, name)

		if isExcluded(path, name, exclude) {
			continue
		}

		info, err := fs.Stat(path)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", path, err)
		}

		addFileInfoToHash(path, info, hasher)

		if info.IsDir() {
			if err := h.hashDirectory(path, exclude, fs, hasher); err != nil {
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

func isExcluded(path, name string, exclude []string) bool {
	for _, pattern := range exclude {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

func addFileInfoToHash(path string, info os.FileInfo, hasher hash.Hash) {
	fileData := fmt.Sprintf("%s:%s:%d:%d", path, info.ModTime().String(), info.Size(), info.Mode())
	hasher.Write([]byte(fileData))
}
