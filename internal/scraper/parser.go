package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"books-scraper/internal/models"
)

var (
	stockRegex = regexp.MustCompile(`\((\d+) available\)`)
	priceRegex = regexp.MustCompile(`[^\d.]`)
)

// ParseRating converts a CSS class like "star-rating Three" to an int.
func ParseRating(class string) int {
	for _, part := range strings.Fields(class) {
		if n, ok := models.RatingWords[part]; ok {
			return n
		}
	}
	return 0
}

// ParsePrice strips the £ symbol and returns a float64.
func ParsePrice(raw string) (float64, error) {
	clean := priceRegex.ReplaceAllString(strings.TrimSpace(raw), "")
	if clean == "" {
		return 0, fmt.Errorf("empty price string")
	}
	return strconv.ParseFloat(clean, 64)
}

// ParseStockCount extracts the integer from "In stock (22 available)".
// Returns 0 if out of stock or count not found.
func ParseStockCount(raw string) int {
	matches := stockRegex.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return 0
	}
	n, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return n
}

// ParseAvailability normalises the availability string.
func ParseAvailability(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.Contains(trimmed, "In stock") {
		return "In stock"
	}
	return "Out of stock"
}

// AbsoluteURL turns a relative href into a full URL using the base.
func AbsoluteURL(base, href string) string {
	if strings.HasPrefix(href, "http") {
		return href
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(href, "/")
}
