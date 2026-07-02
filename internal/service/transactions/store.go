package transactions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/exposure"
)

// FileStore reads pending transactions from a JSON test data file on demand.
type FileStore struct {
	path string
}

// NewFileStore configures a file-backed transaction source.
func NewFileStore(path string) (*FileStore, error) {
	if path == "" {
		return nil, fmt.Errorf("test data path must not be empty")
	}
	return &FileStore{path: path}, nil
}

// ListPending loads the latest pending transactions from the configured file path.
func (s *FileStore) ListPending(_ context.Context) ([]exposure.PendingTransaction, error) {
	rawData, err := os.ReadFile(s.path)
	if err != nil {
		return nil, fmt.Errorf("read test data: %w", err)
	}

	var transactions []exposure.PendingTransaction
	if err := json.Unmarshal(rawData, &transactions); err != nil {
		return nil, fmt.Errorf("decode test data: %w", err)
	}
	return transactions, nil
}
