package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Store struct {
	Path string
}

func NewStore(path string) *Store {
	return &Store{Path: path}
}

func (s *Store) Load() ([]SessionRecord, error) {
	data, err := os.ReadFile(s.Path) // #nosec G304 -- path is user's own session history file

	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading session history: %w", err)
	}

	var records []SessionRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parsing session history: %w", err)
	}

	// Sort by started_at descending
	sort.Slice(records, func(i, j int) bool {
		return records[i].StartedAt.After(records[j].StartedAt)
	})

	return records, nil
}

func (s *Store) Save(records []SessionRecord) error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating session history dir: %w", err)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session history: %w", err)
	}

	if err := os.WriteFile(s.Path, data, 0o600); err != nil {

		return fmt.Errorf("writing session history: %w", err)
	}
	return nil
}

func (s *Store) AddRecord(record SessionRecord) error {
	records, err := s.Load()
	if err != nil {
		return err
	}
	records = append([]SessionRecord{record}, records...)

	// Keep max 1000 records
	if len(records) > 1000 {
		records = records[:1000]
	}

	return s.Save(records)
}

func (s *Store) DeleteRecord(id string) error {
	records, err := s.Load()
	if err != nil {
		return err
	}

	var updated []SessionRecord
	for _, r := range records {
		if r.ID != id {
			updated = append(updated, r)
		}
	}

	return s.Save(updated)
}

func (s *Store) GetRecordsForPR(prNumber int) []SessionRecord {
	records, err := s.Load()
	if err != nil {
		return nil
	}
	var filtered []SessionRecord
	for _, r := range records {
		if r.PRNumber == prNumber {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
