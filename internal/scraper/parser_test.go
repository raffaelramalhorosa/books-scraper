package scraper_test

import (
	"testing"

	"books-scraper/internal/scraper"
)

func TestParseRating(t *testing.T) {
	cases := []struct {
		class string
		want  int
	}{
		{"star-rating One", 1},
		{"star-rating Two", 2},
		{"star-rating Three", 3},
		{"star-rating Four", 4},
		{"star-rating Five", 5},
		{"", 0},
		{"star-rating Unknown", 0},
	}
	for _, tc := range cases {
		got := scraper.ParseRating(tc.class)
		if got != tc.want {
			t.Errorf("ParseRating(%q) = %d, want %d", tc.class, got, tc.want)
		}
	}
}

func TestParsePrice(t *testing.T) {
	cases := []struct {
		raw     string
		want    float64
		wantErr bool
	}{
		{"£12.99", 12.99, false},
		{"£ 9.99", 9.99, false},
		{"£100.00", 100.00, false},
		{"", 0, true},
	}
	for _, tc := range cases {
		got, err := scraper.ParsePrice(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParsePrice(%q): expected error, got nil", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParsePrice(%q): unexpected error: %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParsePrice(%q) = %.2f, want %.2f", tc.raw, got, tc.want)
		}
	}
}

func TestParseStockCount(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"In stock (22 available)", 22},
		{"In stock (1 available)", 1},
		{"In stock", 0},
		{"Out of stock", 0},
		{"", 0},
	}
	for _, tc := range cases {
		got := scraper.ParseStockCount(tc.raw)
		if got != tc.want {
			t.Errorf("ParseStockCount(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}

func TestParseAvailability(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"  In stock (22 available)  ", "In stock"},
		{"In stock", "In stock"},
		{"Out of stock", "Out of stock"},
		{"", "Out of stock"},
	}
	for _, tc := range cases {
		got := scraper.ParseAvailability(tc.raw)
		if got != tc.want {
			t.Errorf("ParseAvailability(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestAbsoluteURL(t *testing.T) {
	cases := []struct {
		base string
		href string
		want string
	}{
		{"https://books.toscrape.com/catalogue", "../a-book/index.html", "https://books.toscrape.com/catalogue/../a-book/index.html"},
		{"https://books.toscrape.com", "https://other.com/page", "https://other.com/page"},
	}
	for _, tc := range cases {
		got := scraper.AbsoluteURL(tc.base, tc.href)
		if got != tc.want {
			t.Errorf("AbsoluteURL(%q, %q) = %q, want %q", tc.base, tc.href, got, tc.want)
		}
	}
}
