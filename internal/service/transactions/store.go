package transactions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/tejasvi-mehra/currency-hedge-calculator/internal/service/exposure"
)

// FileStore keeps pending transactions loaded from a JSON seed file.
type FileStore struct {
	mu           sync.RWMutex
	transactions []exposure.PendingTransaction
}

// NewFileStore loads pending transactions from a JSON file once at startup.
func NewFileStore(path string) (*FileStore, error) {
	rawData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read seed data: %w", err)
	}

	var transactions []exposure.PendingTransaction
	if err := json.Unmarshal(rawData, &transactions); err != nil {
		return nil, fmt.Errorf("decode seed data: %w", err)
	}

	return &FileStore{transactions: transactions}, nil
}

// ListPending returns a copy of pending transactions from in-memory state.
func (s *FileStore) ListPending(_ context.Context) ([]exposure.PendingTransaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]exposure.PendingTransaction, len(s.transactions))
	copy(result, s.transactions)
	return result, nil
}
