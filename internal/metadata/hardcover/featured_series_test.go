package hardcover

import (
	"encoding/json"
	"testing"
)

// The shape Hardcover's search index actually returns, taken from a live
// response for "Harry Potter and the Order of the Phoenix". featured_series is
// the book's row in the series — its id is that row's — and the series it
// belongs to is nested under "series".
const featuredSeriesFromSearch = `{
  "collection": false,
  "details": "5",
  "featured": true,
  "id": 304420,
  "position": 5.0,
  "series": {
    "books_count": 34,
    "id": 1185,
    "name": "Harry Potter",
    "primary_books_count": 7,
    "slug": "harry-potter"
  },
  "unreleased": false
}`

func decodeAny(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSearchFeaturedSeriesReadsTheNestedSeries(t *testing.T) {
	title, id := searchFeaturedSeries(decodeAny(t, featuredSeriesFromSearch))

	if title != "Harry Potter" {
		t.Errorf("title = %q, want the series name and not the object it lives in", title)
	}
	// 304420 is the book's row in the series; 1185 is the series.
	if id != "1185" {
		t.Errorf("id = %q, want the series id", id)
	}
}

func TestSearchSeriesRefsFromTheSearchShape(t *testing.T) {
	refs := searchSeriesRefs(decodeAny(t, featuredSeriesFromSearch), 304420, 5.0)

	if len(refs) != 1 {
		t.Fatalf("refs = %#v, want one", refs)
	}
	if refs[0].ForeignID != "hc-series:1185" {
		t.Errorf("ForeignID = %q, want the series id", refs[0].ForeignID)
	}
	if refs[0].Title != "Harry Potter" || refs[0].Position != "5" {
		t.Errorf("ref = %#v", refs[0])
	}
}

// A book outside any series carries an empty object rather than null.
func TestSearchFeaturedSeriesIgnoresAnEmptyObject(t *testing.T) {
	title, id := searchFeaturedSeries(decodeAny(t, `{}`))
	if title != "" || id != "" {
		t.Fatalf("empty featured_series produced %q / %q", title, id)
	}
}

// Older index versions put the name at the top level; that shape still works.
func TestSearchFeaturedSeriesStillReadsAFlatObject(t *testing.T) {
	title, id := searchFeaturedSeries(decodeAny(t, `{"id": 1185, "name": "Harry Potter"}`))
	if title != "Harry Potter" || id != "1185" {
		t.Fatalf("flat shape produced %q / %q", title, id)
	}
}
