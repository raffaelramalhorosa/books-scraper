package storage

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"books-scraper/internal/models"
)

var csvHeaders = []string{
	"title", "url", "image_url", "category", "upc",
	"rating", "price", "price_excl_tax", "price_incl_tax", "tax",
	"availability", "stock_count", "num_reviews", "description",
	"ai_genres", "ai_themes", "ai_target_audience", "ai_reading_level", "ai_mood", "ai_summary_pt",
}

// WriteCSV writes books to <dir>/books.csv.
func WriteCSV(dir string, books []*models.Book) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	path := filepath.Join(dir, "books.csv")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(csvHeaders); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for _, b := range books {
		row := bookToRow(b)
		if err := w.Write(row); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}
	}
	return w.Error()
}

func bookToRow(b *models.Book) []string {
	genres, themes, audience, level, mood, summaryPT := "", "", "", "", "", ""
	if b.AIAnalysis != nil {
		genres = strings.Join(b.AIAnalysis.Genres, "; ")
		themes = strings.Join(b.AIAnalysis.Themes, "; ")
		audience = b.AIAnalysis.TargetAudience
		level = b.AIAnalysis.ReadingLevel
		mood = b.AIAnalysis.Mood
		summaryPT = b.AIAnalysis.SummaryPT
	}
	return []string{
		b.Title,
		b.URL,
		b.ImageURL,
		b.Category,
		b.UPC,
		strconv.Itoa(b.Rating),
		strconv.FormatFloat(b.Price, 'f', 2, 64),
		strconv.FormatFloat(b.PriceExclTax, 'f', 2, 64),
		strconv.FormatFloat(b.PriceInclTax, 'f', 2, 64),
		strconv.FormatFloat(b.Tax, 'f', 2, 64),
		b.Availability,
		strconv.Itoa(b.StockCount),
		strconv.Itoa(b.NumReviews),
		b.Description,
		genres, themes, audience, level, mood, summaryPT,
	}
}
