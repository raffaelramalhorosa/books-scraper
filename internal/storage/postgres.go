package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"books-scraper/internal/models"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS books (
	id              SERIAL PRIMARY KEY,
	title           TEXT        NOT NULL,
	url             TEXT        UNIQUE NOT NULL,
	image_url       TEXT,
	category        TEXT,
	upc             TEXT,
	rating          SMALLINT,
	price           NUMERIC(10,2),
	price_excl_tax  NUMERIC(10,2),
	price_incl_tax  NUMERIC(10,2),
	tax             NUMERIC(10,2),
	availability    TEXT,
	stock_count     INT,
	num_reviews     INT,
	description     TEXT,
	ai_genres       TEXT[],
	ai_themes       TEXT[],
	ai_target_audience TEXT,
	ai_reading_level   TEXT,
	ai_mood            TEXT,
	ai_summary_pt      TEXT,
	created_at      TIMESTAMPTZ DEFAULT now()
);`

// DB wraps a pgxpool connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// NewDB opens a connection pool and creates the books table if it doesn't exist.
func NewDB(ctx context.Context, dsn string) (*DB, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Close releases pool resources.
func (d *DB) Close() {
	d.pool.Close()
}

// BulkInsert upserts all books using a single COPY + ON CONFLICT strategy.
// On conflict (same URL) it updates all fields.
func (d *DB) BulkInsert(ctx context.Context, books []*models.Book) error {
	if len(books) == 0 {
		return nil
	}

	// pgx CopyFrom is the fastest bulk-insert path available.
	rows := make([][]any, 0, len(books))
	for _, b := range books {
		var genres, themes []string
		var audience, level, mood, summaryPT string
		if b.AIAnalysis != nil {
			genres = b.AIAnalysis.Genres
			themes = b.AIAnalysis.Themes
			audience = b.AIAnalysis.TargetAudience
			level = b.AIAnalysis.ReadingLevel
			mood = b.AIAnalysis.Mood
			summaryPT = b.AIAnalysis.SummaryPT
		}
		rows = append(rows, []any{
			b.Title, b.URL, b.ImageURL, b.Category, b.UPC,
			b.Rating, b.Price, b.PriceExclTax, b.PriceInclTax, b.Tax,
			b.Availability, b.StockCount, b.NumReviews, b.Description,
			genres, themes, audience, level, mood, summaryPT,
		})
	}

	// Temporary staging table → upsert into books avoids COPY's lack of ON CONFLICT.
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE books_staging (LIKE books INCLUDING ALL) ON COMMIT DROP
	`); err != nil {
		return fmt.Errorf("create staging table: %w", err)
	}

	cols := []string{
		"title", "url", "image_url", "category", "upc",
		"rating", "price", "price_excl_tax", "price_incl_tax", "tax",
		"availability", "stock_count", "num_reviews", "description",
		"ai_genres", "ai_themes", "ai_target_audience", "ai_reading_level", "ai_mood", "ai_summary_pt",
	}

	_, err = tx.CopyFrom(
		ctx,
		pgx.Identifier{"books_staging"},
		cols,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("copy from: %w", err)
	}

	setClauses := make([]string, 0, len(cols)-1)
	for _, c := range cols {
		if c == "url" {
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = EXCLUDED.%s", c, c))
	}

	upsertSQL := fmt.Sprintf(`
		INSERT INTO books (%s)
		SELECT %s FROM books_staging
		ON CONFLICT (url) DO UPDATE SET %s
	`, strings.Join(cols, ","), strings.Join(cols, ","), strings.Join(setClauses, ","))

	if _, err := tx.Exec(ctx, upsertSQL); err != nil {
		return fmt.Errorf("upsert: %w", err)
	}

	return tx.Commit(ctx)
}
