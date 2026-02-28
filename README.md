# go-inspirational-quotes

A Go package that provides inspirational quotes from multiple sources with automatic fallback.

## Features

- **zenquotes.io API** — fetches random quotes from the public API
- **Local JSON fallback** — reads from a configurable JSON file if the API is unavailable
- **Embedded defaults** — 30 built-in quotes that always work, even offline
- **Smart quote normalization** — converts curly/Unicode quotes to ASCII equivalents
- **Zero dependencies** — uses only the Go standard library
- **Functional options** — clean, composable configuration

## Installation

```bash
go get github.com/marcuoli/go-inspirational-quotes
```

## Quick Start

```go
package main

import (
    "fmt"
    quotes "github.com/marcuoli/go-inspirational-quotes"
)

func main() {
    q := quotes.FetchRandom()
    fmt.Printf("%q — %s\n", q.Text, q.Author)
}
```

## API

### Types

```go
type Quote struct {
    Text   string `json:"text"`
    Author string `json:"author"`
}
```

### Functions

#### `FetchRandom(opts ...Option) Quote`

Returns a random quote using a cascading strategy:
1. zenquotes.io API (with configurable timeout)
2. Local JSON file (if `WithFallbackFile` was set)
3. Embedded default quotes (always available)

This function **never fails** — it always returns a valid quote.

```go
// Simple usage (API → embedded fallback)
q := quotes.FetchRandom()

// With local file fallback
q := quotes.FetchRandom(quotes.WithFallbackFile("my-quotes.json"))

// With custom timeout
q := quotes.FetchRandom(quotes.WithTimeout(3 * time.Second))

// Both options
q := quotes.FetchRandom(
    quotes.WithTimeout(3 * time.Second),
    quotes.WithFallbackFile("my-quotes.json"),
)
```

#### `FetchFromAPI(timeout time.Duration) (Quote, error)`

Fetches a random quote directly from the zenquotes.io API.

```go
q, err := quotes.FetchFromAPI(5 * time.Second)
if err != nil {
    log.Printf("API error: %v", err)
}
```

#### `FetchFromFile(path string) (Quote, error)`

Reads a random quote from a local JSON file. The file must contain an array of objects with `text` and `author` fields.

```go
q, err := quotes.FetchFromFile("quotes.json")
```

#### `FetchFromEmbed() Quote`

Returns a random quote from the embedded default collection. This never fails.

```go
q := quotes.FetchFromEmbed()
```

#### `NormalizeSmartQuotes(s string) string`

Replaces Unicode smart/curly quotes with ASCII equivalents:
- `"` (U+201C) → `"`
- `"` (U+201D) → `"`
- `'` (U+2018) → `'`
- `'` (U+2019) → `'`

```go
text := quotes.NormalizeSmartQuotes("\u201cHello\u201d")
// text == `"Hello"`
```

### Options

| Option | Description | Default |
|---|---|---|
| `WithTimeout(d)` | HTTP client timeout for API calls | 5 seconds |
| `WithFallbackFile(path)` | Local JSON file to try before embedded defaults | none |

## JSON File Format

Both local files and the embedded collection use the same format:

```json
[
  {"text": "The only way to do great work is to love what you do.", "author": "Steve Jobs"},
  {"text": "Stay hungry, stay foolish.", "author": "Steve Jobs"}
]
```

## Fallback Strategy

```
FetchRandom()
    │
    ├─ 1. zenquotes.io API ───────── success → return quote
    │                                  fail ↓
    ├─ 2. Local JSON file ────────── success → return quote
    │   (if WithFallbackFile set)      fail ↓
    └─ 3. Embedded defaults ─────── always succeeds → return quote
```

## Testing

```bash
go test ./... -v
```

21 tests covering:
- Smart quote normalization (6 sub-tests)
- Embedded quotes (2 tests)
- File loading (5 tests: success, missing, invalid JSON, empty, normalization)
- API fetching (6 tests: success, server error, empty, invalid JSON, timeout, normalization)
- FetchRandom fallback chain (3 tests)
- Options (2 tests)
- JSON roundtrip (1 test)
- Hardcoded fallback (1 test)

## License

MIT
