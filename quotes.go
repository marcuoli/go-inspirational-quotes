// Package quotes provides inspirational quotes from multiple sources.
//
// It fetches a random quote from the zenquotes.io API with automatic
// fallback to a local JSON file or embedded default quotes. Smart/curly
// quotes are normalized to ASCII equivalents.
//
// Usage:
//
//	// Simplest — uses embedded defaults if API is unreachable
//	q := quotes.FetchRandom()
//	fmt.Printf("%q — %s\n", q.Text, q.Author)
//
//	// With a local fallback file
//	q := quotes.FetchRandom(quotes.WithFallbackFile("quotes.json"))
//
//	// With custom timeout
//	q := quotes.FetchRandom(quotes.WithTimeout(3 * time.Second))
//
//	// Individual sources
//	q, err := quotes.FetchFromAPI(5 * time.Second)
//	q, err := quotes.FetchFromFile("quotes.json")
//	q := quotes.FetchFromEmbed()
package quotes

import (
	"embed"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed quotes.json
var embeddedQuotes embed.FS

// DefaultTimeout is the HTTP timeout for the zenquotes.io API.
const DefaultTimeout = 5 * time.Second

// Quote holds an inspirational quote and its author.
type Quote struct {
	Text   string `json:"text"`
	Author string `json:"author"`
}

// config holds options for FetchRandom.
type config struct {
	timeout      time.Duration
	fallbackFile string
}

// Option configures the quote fetcher.
type Option func(*config)

// WithTimeout sets the HTTP client timeout for the API call.
// Default is 5 seconds.
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		c.timeout = d
	}
}

// WithFallbackFile sets a local JSON file path to try before the
// embedded defaults. The file must contain an array of objects with
// "text" and "author" fields.
func WithFallbackFile(path string) Option {
	return func(c *config) {
		c.fallbackFile = path
	}
}

// FetchRandom returns a random inspirational quote using a cascading
// strategy:
//  1. zenquotes.io API (with configurable timeout)
//  2. Local JSON file (if WithFallbackFile was set)
//  3. Embedded default quotes (always available)
//
// This function never returns an error — it always falls back to
// embedded quotes as a last resort.
func FetchRandom(opts ...Option) Quote {
	cfg := &config{timeout: DefaultTimeout}
	for _, o := range opts {
		o(cfg)
	}

	// 1. Try API
	q, err := FetchFromAPI(cfg.timeout)
	if err == nil {
		return q
	}

	// 2. Try local file
	if cfg.fallbackFile != "" {
		q, err = FetchFromFile(cfg.fallbackFile)
		if err == nil {
			return q
		}
	}

	// 3. Embedded fallback (always works)
	return FetchFromEmbed()
}

// zenquotesURL is the API endpoint. It's a variable so tests can
// override it if needed, but normally the exported functions use it
// directly.
var zenquotesURL = "https://zenquotes.io/api/random"

// FetchFromAPI fetches a random quote from the zenquotes.io API.
// The timeout controls how long to wait for the API response.
func FetchFromAPI(timeout time.Duration) (Quote, error) {
	return fetchFromURL(zenquotesURL, timeout)
}

// fetchFromURL fetches a quote from the given URL. This is the internal
// implementation shared by FetchFromAPI and tests.
func fetchFromURL(url string, timeout time.Duration) (Quote, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return Quote{}, fmt.Errorf("zenquotes request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Quote{}, fmt.Errorf("zenquotes returned %d", resp.StatusCode)
	}

	var data []struct {
		Q string `json:"q"`
		A string `json:"a"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return Quote{}, fmt.Errorf("zenquotes decode: %w", err)
	}
	if len(data) == 0 {
		return Quote{}, fmt.Errorf("zenquotes returned empty array")
	}

	return Quote{
		Text:   NormalizeSmartQuotes(data[0].Q),
		Author: data[0].A,
	}, nil
}

// FetchFromFile reads a random quote from a JSON file.
// The file must contain an array of objects with "text" and "author" fields.
func FetchFromFile(path string) (Quote, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Quote{}, fmt.Errorf("read quotes file: %w", err)
	}
	return parseAndPick(raw)
}

// FetchFromEmbed returns a random quote from the embedded default
// collection. This never fails — if the embedded file is somehow
// corrupt, it returns a hardcoded fallback.
func FetchFromEmbed() Quote {
	raw, err := embeddedQuotes.ReadFile("quotes.json")
	if err != nil {
		return hardcodedFallback()
	}
	q, err := parseAndPick(raw)
	if err != nil {
		return hardcodedFallback()
	}
	return q
}

// NormalizeSmartQuotes replaces smart/curly quotes with their ASCII
// equivalents. This is useful because the zenquotes.io API often
// returns Unicode quotation marks.
func NormalizeSmartQuotes(s string) string {
	r := strings.NewReplacer(
		"\u201c", `"`, // Left double quotation mark "
		"\u201d", `"`, // Right double quotation mark "
		"\u2018", "'", // Left single quotation mark '
		"\u2019", "'", // Right single quotation mark '
	)
	return r.Replace(s)
}

// parseAndPick parses a JSON array of quotes and returns one at random.
func parseAndPick(raw []byte) (Quote, error) {
	var items []Quote
	if err := json.Unmarshal(raw, &items); err != nil {
		return Quote{}, fmt.Errorf("parse quotes: %w", err)
	}
	if len(items) == 0 {
		return Quote{}, fmt.Errorf("quotes collection is empty")
	}
	q := items[rand.IntN(len(items))]
	q.Text = NormalizeSmartQuotes(q.Text)
	return q, nil
}

// hardcodedFallback returns a quote that is always available,
// even if all other sources fail.
func hardcodedFallback() Quote {
	return Quote{
		Text:   "The only way to do great work is to love what you do.",
		Author: "Steve Jobs",
	}
}
