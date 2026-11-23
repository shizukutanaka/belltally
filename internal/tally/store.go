package tally

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Store is the on-disk tally: a single JSON file in the user's own
// data folder. Nothing ever leaves the machine.
type Store struct {
	Path string // full path of the JSON data file
}

type fileFormat struct {
	Version int   `json:"version"`
	Book    *Book `json:"book"`
}

// DataDir returns BellTally's data directory, honouring
// BELLTALLY_DATA_DIR, then the platform convention via
// os.UserConfigDir (%APPDATA%\BellTally on Windows,
// ~/Library/Application Support/BellTally on macOS,
// $XDG_CONFIG_HOME/BellTally or ~/.config/BellTally elsewhere).
func DataDir() (string, error) {
	if d := os.Getenv("BELLTALLY_DATA_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "BellTally"), nil
}

// OpenStore resolves the store location and ensures it exists.
func OpenStore() (*Store, error) {
	dir, err := DataDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{Path: filepath.Join(dir, "belltally.json")}, nil
}

// Load reads the book. A missing file is an empty book, not an error.
func (s *Store) Load() (*Book, error) {
	raw, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Book{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return &Book{}, nil
	}
	var f fileFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("data file is corrupted: %w", err)
	}
	if f.Book == nil {
		return &Book{}, nil
	}
	return f.Book, nil
}

// Save writes the book atomically (temp file + rename), so a crash or
// a power cut mid-write can never truncate the tally.
func (s *Store) Save(b *Book) error {
	raw, err := json.MarshalIndent(fileFormat{Version: 1, Book: b}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
