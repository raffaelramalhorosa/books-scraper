package storage_test

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"books-scraper/internal/models"
	"books-scraper/internal/storage"
)

func TestWriteCSV(t *testing.T) {
	dir := t.TempDir()
	books := []*models.Book{
		{
			Title:        "A Great Book",
			URL:          "https://example.com/book",
			Rating:       4,
			Price:        12.99,
			Availability: "In stock",
			StockCount:   10,
			AIAnalysis: &models.AIAnalysis{
				Genres:         []string{"Fiction"},
				Themes:         []string{"Love"},
				TargetAudience: "Adults",
				ReadingLevel:   "Intermediate",
				Mood:           "Hopeful",
				SummaryPT:      "Um livro incrível.",
			},
		},
	}

	if err := storage.WriteCSV(dir, books); err != nil {
		t.Fatalf("WriteCSV error: %v", err)
	}

	f, err := os.Open(filepath.Join(dir, "books.csv"))
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 { // header + 1 row
		t.Fatalf("expected 2 records (header+row), got %d", len(records))
	}
	if records[1][0] != "A Great Book" {
		t.Errorf("title mismatch: got %q, want %q", records[1][0], "A Great Book")
	}
}

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	books := []*models.Book{
		{Title: "Test Book", Price: 9.99, Rating: 3},
	}
	if err := storage.WriteJSON(dir, books); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "books.json"))
	if err != nil {
		t.Fatalf("books.json not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("books.json is empty")
	}
}
