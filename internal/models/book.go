package models

// AIAnalysis holds structured data extracted by Gemini Flash from a book description.
type AIAnalysis struct {
	Genres         []string `json:"genres"`
	Themes         []string `json:"themes"`
	TargetAudience string   `json:"target_audience"`
	ReadingLevel   string   `json:"reading_level"`
	Mood           string   `json:"mood"`
	SummaryPT      string   `json:"summary_pt"`
}

// Book represents all data collected for a single book from books.toscrape.com.
type Book struct {
	Title        string      `json:"title"`
	URL          string      `json:"url"`
	ImageURL     string      `json:"image_url"`
	Category     string      `json:"category"`
	UPC          string      `json:"upc"`
	Rating       int         `json:"rating"`
	Price        float64     `json:"price"`
	PriceExclTax float64     `json:"price_excl_tax"`
	PriceInclTax float64     `json:"price_incl_tax"`
	Tax          float64     `json:"tax"`
	Availability string      `json:"availability"`
	StockCount   int         `json:"stock_count"`
	NumReviews   int         `json:"num_reviews"`
	Description  string      `json:"description"`
	AIAnalysis   *AIAnalysis `json:"ai_analysis,omitempty"`
}

// RatingWords maps the CSS class word to its numeric equivalent.
var RatingWords = map[string]int{
	"One":   1,
	"Two":   2,
	"Three": 3,
	"Four":  4,
	"Five":  5,
}
