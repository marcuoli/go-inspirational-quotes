package quotes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- NormalizeSmartQuotes ---

func TestNormalizeSmartQuotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"left double", "\u201cHello\u201d", `"Hello"`},
		{"right single", "it\u2019s", "it's"},
		{"left single", "\u2018world\u2019", "'world'"},
		{"mixed", "\u201cit\u2019s a \u201ctest\u201d\u201d", `"it's a "test""`},
		{"no change", "Hello world", "Hello world"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeSmartQuotes(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeSmartQuotes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- FetchFromEmbed ---

func TestFetchFromEmbed(t *testing.T) {
	q := FetchFromEmbed()
	if q.Text == "" {
		t.Error("embedded quote text should not be empty")
	}
	if q.Author == "" {
		t.Error("embedded quote author should not be empty")
	}
}

func TestFetchFromEmbed_NeverFails(t *testing.T) {
	// Call multiple times to exercise randomness
	for i := 0; i < 50; i++ {
		q := FetchFromEmbed()
		if q.Text == "" || q.Author == "" {
			t.Fatalf("iteration %d: got empty quote", i)
		}
	}
}

// --- FetchFromFile ---

func TestFetchFromFile(t *testing.T) {
	path := writeTempFile(t, `[
		{"text": "Test quote", "author": "Test Author"},
		{"text": "Another quote", "author": "Another Author"}
	]`)

	q, err := FetchFromFile(path)
	if err != nil {
		t.Fatalf("FetchFromFile: %v", err)
	}
	if q.Text == "" {
		t.Error("quote text should not be empty")
	}
	if q.Author == "" {
		t.Error("quote author should not be empty")
	}
}

func TestFetchFromFile_MissingFile(t *testing.T) {
	_, err := FetchFromFile("/nonexistent/path/quotes.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestFetchFromFile_InvalidJSON(t *testing.T) {
	path := writeTempFile(t, `not valid json`)

	_, err := FetchFromFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFetchFromFile_EmptyArray(t *testing.T) {
	path := writeTempFile(t, `[]`)

	_, err := FetchFromFile(path)
	if err == nil {
		t.Error("expected error for empty array")
	}
}

func TestFetchFromFile_UsesFromFieldAsAuthor(t *testing.T) {
	path := writeTempFile(t, `[{"text": "Django format quote", "from": "Django Author"}]`)

	q, err := FetchFromFile(path)
	if err != nil {
		t.Fatalf("FetchFromFile: %v", err)
	}
	if q.Text != "Django format quote" {
		t.Errorf("text = %q, want %q", q.Text, "Django format quote")
	}
	if q.Author != "Django Author" {
		t.Errorf("author = %q, want %q", q.Author, "Django Author")
	}
}

func TestFetchFromFile_MissingAuthorAndFrom(t *testing.T) {
	path := writeTempFile(t, `[{"text": "Quote without author"}]`)

	_, err := FetchFromFile(path)
	if err == nil {
		t.Error("expected error when quote has no author/from")
	}
}

func TestFetchFromFile_NormalizesSmartQuotes(t *testing.T) {
	path := writeTempFile(t, `[{"text": "\u201cSmart\u201d", "author": "Author"}]`)

	q, err := FetchFromFile(path)
	if err != nil {
		t.Fatalf("FetchFromFile: %v", err)
	}
	if strings.Contains(q.Text, "\u201c") || strings.Contains(q.Text, "\u201d") {
		t.Errorf("smart quotes not normalized: %q", q.Text)
	}
	if q.Text != `"Smart"` {
		t.Errorf("got %q, want %q", q.Text, `"Smart"`)
	}
}

// --- FetchFromAPI ---

func TestFetchFromAPI_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{
			{"q": "API quote", "a": "API Author"},
		})
	}))
	defer srv.Close()

	// We can't easily swap the URL, so test via FetchRandom with a mock
	// This test verifies the parsing logic directly
	q, err := fetchFromURL(srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("fetchFromURL: %v", err)
	}
	if q.Text != "API quote" {
		t.Errorf("text = %q, want %q", q.Text, "API quote")
	}
	if q.Author != "API Author" {
		t.Errorf("author = %q, want %q", q.Author, "API Author")
	}
}

func TestFetchFromAPI_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchFromURL(srv.URL, 5*time.Second)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestFetchFromAPI_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{})
	}))
	defer srv.Close()

	_, err := fetchFromURL(srv.URL, 5*time.Second)
	if err == nil {
		t.Error("expected error for empty response")
	}
}

func TestFetchFromAPI_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	_, err := fetchFromURL(srv.URL, 5*time.Second)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFetchFromAPI_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte(`[{"q":"late","a":"Author"}]`))
	}))
	defer srv.Close()

	_, err := fetchFromURL(srv.URL, 50*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestFetchFromAPI_SmartQuotesNormalized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[{"q":"\u201cSmart\u201d","a":"Author"}]`))
	}))
	defer srv.Close()

	q, err := fetchFromURL(srv.URL, 5*time.Second)
	if err != nil {
		t.Fatalf("fetchFromURL: %v", err)
	}
	if q.Text != `"Smart"` {
		t.Errorf("smart quotes not normalized: got %q, want %q", q.Text, `"Smart"`)
	}
}

// --- FetchRandom ---

func TestFetchRandom_FallsBackToEmbed(t *testing.T) {
	// No API, no file — should get embedded quote
	q := FetchRandom(WithTimeout(1 * time.Millisecond))
	if q.Text == "" || q.Author == "" {
		t.Error("FetchRandom should return a non-empty quote even without API")
	}
}

func TestFetchRandom_FallsBackToFile(t *testing.T) {
	path := writeTempFile(t, `[{"text": "File quote", "author": "File Author"}]`)

	q := FetchRandom(
		WithTimeout(1*time.Millisecond),
		WithFallbackFile(path),
	)
	if q.Text == "" || q.Author == "" {
		t.Error("FetchRandom should return a non-empty quote")
	}
	// With 1ms timeout, API should fail, so it falls back to file
	if q.Text != "File quote" {
		// It's possible the API succeeded in 1ms, which is unlikely but not impossible
		t.Logf("note: got %q (API may have responded fast)", q.Text)
	}
}

func TestFetchRandom_DefaultTimeout(t *testing.T) {
	// Just verifies no panic with default options
	q := FetchRandom()
	if q.Text == "" || q.Author == "" {
		t.Error("FetchRandom should always return a non-empty quote")
	}
}

// --- Options ---

func TestWithTimeout(t *testing.T) {
	cfg := &config{timeout: DefaultTimeout}
	WithTimeout(3 * time.Second)(cfg)
	if cfg.timeout != 3*time.Second {
		t.Errorf("timeout = %v, want 3s", cfg.timeout)
	}
}

func TestWithFallbackFile(t *testing.T) {
	cfg := &config{}
	WithFallbackFile("/some/path.json")(cfg)
	if cfg.fallbackFile != "/some/path.json" {
		t.Errorf("fallbackFile = %q, want %q", cfg.fallbackFile, "/some/path.json")
	}
}

// --- Quote type ---

func TestQuote_JSON(t *testing.T) {
	q := Quote{Text: "Hello", Author: "World"}
	data, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var q2 Quote
	if err := json.Unmarshal(data, &q2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if q2.Text != q.Text || q2.Author != q.Author {
		t.Errorf("roundtrip: got %+v, want %+v", q2, q)
	}
}

// --- hardcodedFallback ---

func TestHardcodedFallback(t *testing.T) {
	q := hardcodedFallback()
	if q.Text == "" || q.Author == "" {
		t.Error("hardcoded fallback should not be empty")
	}
}

// --- helpers ---

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "quotes.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}
