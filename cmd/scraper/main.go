package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"books-scraper/internal/ai"
	"books-scraper/internal/models"
	"books-scraper/internal/scraper"
	"books-scraper/internal/storage"
)

func main() {
	pages := flag.Int("pages", 50, "maximum listing pages to scrape (0 = unlimited)")
	delay := flag.Duration("delay", 300*time.Millisecond, "delay between requests per domain")
	workers := flag.Int("workers", 2, "number of parallel detail-page workers")
	output := flag.String("output", "./output", "directory for output files")
	formats := flag.String("formats", "json,csv", "comma-separated output formats: json,csv,postgres")
	dbURL := flag.String("db-url", "", "PostgreSQL DSN (required when 'postgres' is in formats)")
	geminiKey   := flag.String("gemini-key", "", "Gemini API key (overrides GEMINI_API_KEY env var)")
	geminiModel := flag.String("gemini-model", "gemini-1.5-flash", "Gemini model to use for AI analysis")
	useGemini   := flag.Bool("gemini", false, "enable AI analysis via Gemini Flash")
	flag.Parse()

	// Resolve output to absolute path to avoid CWD ambiguity with `go run`.
	absOutput, err := filepath.Abs(*output)
	if err != nil {
		slog.Error("resolve output path", "err", err)
		os.Exit(1)
	}
	output = &absOutput

	// Resolve Gemini API key: flag > env var.
	apiKey := *geminiKey
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if *useGemini && apiKey == "" {
		slog.Error("--gemini enabled but no API key provided (use --gemini-key or GEMINI_API_KEY)")
		os.Exit(1)
	}

	ctx := context.Background()

	// Initialise Gemini client once and reuse across goroutines.
	var geminiClient *ai.Client
	if *useGemini {
		var err error
		geminiClient, err = ai.NewClient(ctx, apiKey, *geminiModel)
		if err != nil {
			slog.Error("failed to init gemini", "err", err)
			os.Exit(1)
		}
		defer geminiClient.Close()
	}

	// Initialise PostgreSQL connection if needed.
	var db *storage.DB
	formatSet := parseFormats(*formats)
	if formatSet["postgres"] {
		if *dbURL == "" {
			slog.Error("postgres format requested but --db-url is empty")
			os.Exit(1)
		}
		var err error
		db, err = storage.NewDB(ctx, *dbURL)
		if err != nil {
			slog.Error("failed to connect to postgres", "err", err)
			os.Exit(1)
		}
		defer db.Close()
	}

	// Collect books from the scraper via a channel for safe concurrent writes.
	var (
		mu    sync.Mutex
		books []*models.Book
	)

	onBook := func(b *models.Book) {
		// Enrich with Gemini analysis if enabled.
		if geminiClient != nil && b.Description != "" {
			analysis, err := geminiClient.AnalyzeDescription(ctx, b.Description)
			if err != nil {
				slog.Warn("gemini analysis failed", "title", b.Title, "err", err)
			} else {
				b.AIAnalysis = analysis
			}
		}
		mu.Lock()
		books = append(books, b)
		mu.Unlock()
	}

	slog.Info("starting scraper",
		"pages", *pages,
		"delay", *delay,
		"workers", *workers,
		"formats", *formats,
		"gemini", *useGemini,
	)

	if err := scraper.Run(scraper.Config{
		MaxPages: *pages,
		Delay:    *delay,
		Workers:  *workers,
		OnBook:   onBook,
	}); err != nil {
		slog.Error("scraper failed", "err", err)
		os.Exit(1)
	}

	slog.Info("scraping complete", "total_books", len(books))

	// Write outputs.
	if formatSet["json"] {
		if err := storage.WriteJSON(*output, books); err != nil {
			slog.Error("write json failed", "err", err)
		} else {
			slog.Info("json written", "path", *output+"/books.json")
		}
	}

	if formatSet["csv"] {
		if err := storage.WriteCSV(*output, books); err != nil {
			slog.Error("write csv failed", "err", err)
		} else {
			slog.Info("csv written", "path", *output+"/books.csv")
		}
	}

	if formatSet["postgres"] && db != nil {
		if err := db.BulkInsert(ctx, books); err != nil {
			slog.Error("postgres insert failed", "err", err)
		} else {
			slog.Info("postgres insert complete", "rows", len(books))
		}
	}

	slog.Info("done")
}

// parseFormats converts "json,csv,postgres" into a lookup set.
// Uses SplitSeq (Go 1.24+) to avoid allocating an intermediate slice.
func parseFormats(raw string) map[string]bool {
	set := make(map[string]bool)
	for f := range strings.SplitSeq(raw, ",") {
		set[strings.TrimSpace(strings.ToLower(f))] = true
	}
	return set
}
