package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"books-scraper/internal/models"
)

// WriteJSON serialises books to <dir>/books.json with human-readable indentation.
func WriteJSON(dir string, books []*models.Book) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	path := filepath.Join(dir, "books.json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create json file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(books); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}
