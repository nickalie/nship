// Package fs provides functionality for file system operations and copying files.
package fs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/nickalie/nship/internal/core/job"
)

const (
	// DefaultHashDir is the default directory for storing step hashes
	DefaultHashDir = ".nship/hashes"
)

// StepHash represents hash data for a specific job step
type StepHash struct {
	TargetName string `json:"target"`
	JobName    string `json:"job"`
	StepIndex  int    `json:"step"`
	Hash       string `json:"hash"`
}

// FileHashStorage implements HashStorage using the file system
type FileHashStorage struct {
	baseDir    string
	fileSystem FileSystem
	mu         sync.RWMutex
	hashes     map[string]StepHash
	loaded     bool
}

// NewFileHashStorage creates a new FileHashStorage with the default directory
func NewFileHashStorage() *FileHashStorage {
	return NewFileHashStorageWithPath(DefaultHashDir)
}

// NewFileHashStorageWithPath creates a new FileHashStorage with a custom directory
func NewFileHashStorageWithPath(baseDir string) *FileHashStorage {
	return &FileHashStorage{
		baseDir:    baseDir,
		fileSystem: NewFileSystem(),
		hashes:     make(map[string]StepHash),
	}
}

// SaveHash stores a hash for a job step on a specific target
func (s *FileHashStorage) SaveHash(targetName, jobName string, stepIndex int, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureLoaded(); err != nil {
		return err
	}

	// Create the key for this hash
	key := makeHashKey(targetName, jobName, stepIndex)

	// Store the hash in memory
	s.hashes[key] = StepHash{
		TargetName: targetName,
		JobName:    jobName,
		StepIndex:  stepIndex,
		Hash:       hash,
	}

	// Persist to disk
	return s.persist()
}

// GetHash retrieves a hash for a job step on a specific target
func (s *FileHashStorage) GetHash(targetName, jobName string, stepIndex int) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.ensureLoaded(); err != nil {
		return "", err
	}

	key := makeHashKey(targetName, jobName, stepIndex)
	if hash, ok := s.hashes[key]; ok {
		return hash.Hash, nil
	}

	return "", nil // No hash found is not an error
}

// Clear removes all stored hashes
func (s *FileHashStorage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hashes = make(map[string]StepHash)

	// Remove hash file if it exists
	hashFile := s.getHashFilePath()
	err := s.fileSystem.RemoveAll(hashFile)

	// Ignore errors if file doesn't exist
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove hash file: %w", err)
	}

	return nil
}

// ensureLoaded makes sure the hashes are loaded from disk
func (s *FileHashStorage) ensureLoaded() error {
	if s.loaded {
		return nil
	}

	hashFile := s.getHashFilePath()
	data, err := s.fileSystem.ReadFile(hashFile)

	if os.IsNotExist(err) {
		// File doesn't exist yet, that's fine
		s.loaded = true
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to read hash file: %w", err)
	}

	var hashList []StepHash
	if err := json.Unmarshal(data, &hashList); err != nil {
		return fmt.Errorf("failed to parse hash file: %w", err)
	}

	// Convert to map for faster lookups
	s.hashes = make(map[string]StepHash, len(hashList))
	for _, hash := range hashList {
		key := makeHashKey(hash.TargetName, hash.JobName, hash.StepIndex)
		s.hashes[key] = hash
	}

	s.loaded = true
	return nil
}

// persist saves the hashes to disk
func (s *FileHashStorage) persist() error {
	// Convert map to slice for storage
	hashList := make([]StepHash, 0, len(s.hashes))
	for _, hash := range s.hashes {
		hashList = append(hashList, hash)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(hashList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hashes: %w", err)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(s.getHashFilePath())
	if err := s.fileSystem.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create hash directory: %w", err)
	}

	// Write to file
	if err := s.fileSystem.WriteFile(s.getHashFilePath(), data, 0644); err != nil {
		return fmt.Errorf("failed to write hash file: %w", err)
	}

	return nil
}

// getHashFilePath returns the path to the hash file
func (s *FileHashStorage) getHashFilePath() string {
	return filepath.Join(s.baseDir, "step_hashes.json")
}

// makeHashKey creates a unique key for a hash
func makeHashKey(targetName, jobName string, stepIndex int) string {
	return fmt.Sprintf("%s:%s:%d", targetName, jobName, stepIndex)
}

// Ensure FileHashStorage implements the HashStorage interface
var _ job.HashStorage = (*FileHashStorage)(nil)
