package scraper

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"

	"books-scraper/internal/models"
)

const (
	baseURL    = "https://books.toscrape.com"
	catalogURL = "https://books.toscrape.com/catalogue/"
)

// Config holds runtime options passed in from CLI flags.
type Config struct {
	MaxPages   int
	Delay      time.Duration
	Workers    int
	OnBook     func(*models.Book)
}

// Run starts the scraping process and calls cfg.OnBook for every book collected.
func Run(cfg Config) error {
	// detailCollector visits individual book pages.
	detail := colly.NewCollector(
		colly.AllowedDomains("books.toscrape.com"),
		colly.UserAgent("books-scraper/1.0 (educational; github.com/user/books-scraper)"),
	)
	if err := detail.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: cfg.Workers,
		Delay:       cfg.Delay,
	}); err != nil {
		return fmt.Errorf("detail limit rule: %w", err)
	}
	extensions.RandomUserAgent(detail)

	detail.OnHTML("div.col-sm-6.product_main", func(e *colly.HTMLElement) {
		book, ok := e.Request.Ctx.GetAny("book").(*models.Book)
		if !ok {
			return
		}

		book.Title = strings.TrimSpace(e.ChildText("h1"))
		ratingClass := e.ChildAttr("p.star-rating", "class")
		book.Rating = ParseRating(ratingClass)

		priceRaw := e.ChildText("p.price_color")
		if p, err := ParsePrice(priceRaw); err == nil {
			book.Price = p
		}

		availRaw := e.ChildText("p.availability")
		book.Availability = ParseAvailability(availRaw)
		book.StockCount = ParseStockCount(availRaw)
	})

	detail.OnHTML("div#product_description ~ p", func(e *colly.HTMLElement) {
		book, ok := e.Request.Ctx.GetAny("book").(*models.Book)
		if !ok {
			return
		}
		if book.Description == "" {
			book.Description = strings.TrimSpace(e.Text)
		}
	})

	detail.OnHTML("table.table-striped tr", func(e *colly.HTMLElement) {
		book, ok := e.Request.Ctx.GetAny("book").(*models.Book)
		if !ok {
			return
		}
		key := strings.TrimSpace(e.ChildText("th"))
		val := strings.TrimSpace(e.ChildText("td"))
		switch key {
		case "UPC":
			book.UPC = val
		case "Price (excl. tax)":
			if p, err := ParsePrice(val); err == nil {
				book.PriceExclTax = p
			}
		case "Price (incl. tax)":
			if p, err := ParsePrice(val); err == nil {
				book.PriceInclTax = p
			}
		case "Tax":
			if p, err := ParsePrice(val); err == nil {
				book.Tax = p
			}
		case "Number of reviews":
			var n int
			if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
				book.NumReviews = n
			}
		}
	})

	// Category comes from the breadcrumb on the detail page.
	detail.OnHTML("ul.breadcrumb li:nth-child(3) a", func(e *colly.HTMLElement) {
		book, ok := e.Request.Ctx.GetAny("book").(*models.Book)
		if !ok {
			return
		}
		book.Category = strings.TrimSpace(e.Text)
	})

	detail.OnScraped(func(r *colly.Response) {
		book, ok := r.Ctx.GetAny("book").(*models.Book)
		if !ok {
			return
		}
		cfg.OnBook(book)
		slog.Info("scraped", "title", book.Title, "url", book.URL)
	})

	detail.OnError(func(r *colly.Response, err error) {
		slog.Error("detail request failed", "url", r.Request.URL.String(), "err", err)
	})

	// listingCollector visits catalogue listing pages and pagination.
	listing := colly.NewCollector(
		colly.AllowedDomains("books.toscrape.com"),
		colly.UserAgent("books-scraper/1.0 (educational; github.com/user/books-scraper)"),
	)
	if err := listing.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1, // one listing page at a time to preserve page order
		Delay:       cfg.Delay,
	}); err != nil {
		return fmt.Errorf("listing limit rule: %w", err)
	}

	pageCount := 0

	listing.OnHTML("article.product_pod", func(e *colly.HTMLElement) {
		href := e.ChildAttr("h3 a", "href")
		// href is relative to the catalogue dir, e.g. "../../../a-light-in-the.../index.html"
		// Colly's e.Request.AbsoluteURL resolves it correctly.
		detailURL := e.Request.AbsoluteURL(href)
		if detailURL == "" {
			return
		}

		imageHref := e.ChildAttr("div.image_container img", "src")
		imageURL := e.Request.AbsoluteURL(imageHref)

		book := &models.Book{
			URL:      detailURL,
			ImageURL: imageURL,
		}
		ctx := colly.NewContext()
		ctx.Put("book", book)
		if err := detail.Request("GET", detailURL, nil, ctx, nil); err != nil {
			slog.Error("queuing detail failed", "url", detailURL, "err", err)
		}
	})

	listing.OnHTML("li.next a", func(e *colly.HTMLElement) {
		if cfg.MaxPages > 0 && pageCount >= cfg.MaxPages {
			return
		}
		nextURL := e.Request.AbsoluteURL(e.Attr("href"))
		if err := listing.Visit(nextURL); err != nil {
			slog.Error("pagination visit failed", "url", nextURL, "err", err)
		}
	})

	listing.OnRequest(func(r *colly.Request) {
		pageCount++
		slog.Info("visiting listing page", "page", pageCount, "url", r.URL.String())
	})

	listing.OnError(func(r *colly.Response, err error) {
		slog.Error("listing request failed", "url", r.Request.URL.String(), "err", err)
	})

	if err := listing.Visit(baseURL); err != nil {
		return fmt.Errorf("initial visit: %w", err)
	}

	listing.Wait()
	detail.Wait()
	return nil
}
