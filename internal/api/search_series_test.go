package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vavallee/bindery/internal/models"
)

type stubSeriesPresence struct {
	present map[string]bool
	asked   []string
	err     error
}

func (s *stubSeriesPresence) ForeignIDsInLibrary(_ context.Context, ids []string) (map[string]bool, error) {
	s.asked = append(s.asked, ids...)
	if s.err != nil {
		return nil, s.err
	}
	return s.present, nil
}

func bookWithSeries(title string, refs ...models.SeriesRef) models.Book {
	return models.Book{ForeignID: "hc:" + title, Title: title, SeriesRefs: refs}
}

func TestWithSeriesFlattensThePrimaryRefAndMarksTheLibrary(t *testing.T) {
	presence := &stubSeriesPresence{present: map[string]bool{"hc-series:1185": true}}
	h := NewSearchHandler(nil).WithSeriesPresence(presence)

	got := h.withSeries(context.Background(), []models.Book{
		bookWithSeries("Order of the Phoenix",
			models.SeriesRef{ForeignID: "hc-series:9", Title: "Wizarding World"},
			models.SeriesRef{ForeignID: "hc-series:1185", Title: "Harry Potter", Position: "5", Primary: true},
		),
		bookWithSeries("Some Standalone"),
	})

	if len(got) != 2 {
		t.Fatalf("withSeries returned %d results, want 2", len(got))
	}
	if got[0].SeriesForeignID != "hc-series:1185" || got[0].SeriesTitle != "Harry Potter" || got[0].SeriesPosition != "5" {
		t.Errorf("primary ref not used: %+v", got[0])
	}
	if !got[0].SeriesInLibrary {
		t.Error("series held by the library was not marked")
	}
	if got[1].SeriesForeignID != "" || got[1].SeriesInLibrary {
		t.Errorf("standalone book gained a series: %+v", got[1])
	}
}

// A ref with no primary flag still names a series, and is better than saying
// the book belongs to none.
func TestWithSeriesFallsBackToTheFirstNamedRef(t *testing.T) {
	h := NewSearchHandler(nil)

	got := h.withSeries(context.Background(), []models.Book{
		bookWithSeries("Book",
			models.SeriesRef{ForeignID: "", Title: "Nameless id"},
			models.SeriesRef{ForeignID: "hc-series:7", Title: "Only Option"},
		),
	})

	if got[0].SeriesForeignID != "hc-series:7" {
		t.Fatalf("fallback ref = %+v", got[0])
	}
	// Without a presence source there is nothing to check against.
	if got[0].SeriesInLibrary {
		t.Error("marked as held with no library to ask")
	}
}

// Searching must survive a database that will not answer.
func TestWithSeriesKeepsResultsWhenTheLibraryLookupFails(t *testing.T) {
	presence := &stubSeriesPresence{err: errors.New("database is gone")}
	h := NewSearchHandler(nil).WithSeriesPresence(presence)

	got := h.withSeries(context.Background(), []models.Book{
		bookWithSeries("Book", models.SeriesRef{ForeignID: "hc-series:1", Title: "S", Primary: true}),
	})

	if len(got) != 1 || got[0].SeriesTitle != "S" {
		t.Fatalf("results lost on lookup failure: %+v", got)
	}
	if got[0].SeriesInLibrary {
		t.Error("claimed the library holds a series it could not read")
	}
}

// The response embeds the book, so every field the UI already relied on has to
// survive the wrapper.
func TestSearchResultStillCarriesTheBook(t *testing.T) {
	h := NewSearchHandler(nil)
	results := h.withSeries(context.Background(), []models.Book{
		bookWithSeries("Dune", models.SeriesRef{ForeignID: "hc-series:2", Title: "Dune", Position: "1", Primary: true}),
	})

	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, results)

	var decoded []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded[0]["title"] != "Dune" {
		t.Errorf("book fields lost: %v", decoded[0])
	}
	if decoded[0]["seriesTitle"] != "Dune" || decoded[0]["seriesPosition"] != "1" {
		t.Errorf("series fields missing: %v", decoded[0])
	}
	if _, leaked := decoded[0]["SeriesRefs"]; leaked {
		t.Error("SeriesRefs leaked into JSON; it is meant to stay internal")
	}
}
