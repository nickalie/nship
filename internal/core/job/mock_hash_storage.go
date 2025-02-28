package job

// MockHashStorage implements the HashStorage interface for testing
type MockHashStorage struct {
	GetHashFunc  func(targetName, jobName string, stepIndex int) (string, error)
	SaveHashFunc func(targetName, jobName string, stepIndex int, hash string) error
	ClearFunc    func() error
}

// GetHash retrieves a hash for a job step on a specific target
func (m *MockHashStorage) GetHash(targetName, jobName string, stepIndex int) (string, error) {
	if m.GetHashFunc != nil {
		return m.GetHashFunc(targetName, jobName, stepIndex)
	}
	return "", nil
}

// SaveHash stores a hash for a job step on a specific target
func (m *MockHashStorage) SaveHash(targetName, jobName string, stepIndex int, hash string) error {
	if m.SaveHashFunc != nil {
		return m.SaveHashFunc(targetName, jobName, stepIndex, hash)
	}
	return nil
}

// Clear removes all stored hashes
func (m *MockHashStorage) Clear() error {
	if m.ClearFunc != nil {
		return m.ClearFunc()
	}
	return nil
}

// Ensure MockHashStorage implements the HashStorage interface
var _ HashStorage = (*MockHashStorage)(nil)
