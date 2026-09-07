package hardcover

import (
	"strings"
	"testing"

	"github.com/vavallee/bindery/internal/metadata"
)

// Hardcover records a translation as its own book sharing the original's
// position, so a slot arrives holding one entry per language. The shape and
// the reader counts here are those the API returns for position 1 of Harry
// Potter (series 1185).
func TestCollapseSeriesPositionsKeepsTheMostHeldEntry(t *testing.T) {
	books := []metadata.SeriesCatalogBook{
		{ProviderID: "2456202", Title: "Droga królów", Position: "1", UsersCount: 12},
		{ProviderID: "328491", Title: "Harry Potter and the Philosopher's Stone", Position: "1", UsersCount: 17351},
		{ProviderID: "1945721", Title: "Гарри Поттер и философский камень", Position: "1", UsersCount: 40},
		{ProviderID: "429306", Title: "Harry Potter and the Chamber of Secrets", Position: "2", UsersCount: 13542},
	}

	got := collapseSeriesPositions(books)

	if len(got) != 2 {
		t.Fatalf("collapseSeriesPositions() kept %d books, want one per position", len(got))
	}
	if got[0].ProviderID != "328491" {
		t.Errorf("position 1 = %q (%s), want the English novel", got[0].Title, got[0].ProviderID)
	}
	if got[1].ProviderID != "429306" {
		t.Errorf("position 2 = %q (%s)", got[1].Title, got[1].ProviderID)
	}
}

// A book with no position is an extra the series accumulated, not a duplicate
// of a volume, so several of them must all survive.
func TestCollapseSeriesPositionsLeavesUnpositionedBooksAlone(t *testing.T) {
	books := []metadata.SeriesCatalogBook{
		{ProviderID: "1", Title: "Companion", Position: "", UsersCount: 5},
		{ProviderID: "2", Title: "Artbook", Position: "  ", UsersCount: 3},
		{ProviderID: "3", Title: "Volume One", Position: "1", UsersCount: 10},
	}

	got := collapseSeriesPositions(books)

	if len(got) != 3 {
		t.Fatalf("collapseSeriesPositions() kept %d books, want all three", len(got))
	}
}

// Fractional positions are their own slots: a 2.5 novella is not a competing
// edition of volume 2.
func TestCollapseSeriesPositionsTreatsFractionalPositionsSeparately(t *testing.T) {
	books := []metadata.SeriesCatalogBook{
		{ProviderID: "1", Title: "Words of Radiance", Position: "2", UsersCount: 6088},
		{ProviderID: "2", Title: "Edgedancer", Position: "2.5", UsersCount: 3315},
	}

	got := collapseSeriesPositions(books)

	if len(got) != 2 {
		t.Fatalf("collapseSeriesPositions() kept %d books, want both", len(got))
	}
}

// Order is the caller's; collapsing must not reshuffle what it keeps.
func TestCollapseSeriesPositionsPreservesOrder(t *testing.T) {
	books := []metadata.SeriesCatalogBook{
		{ProviderID: "a", Title: "One", Position: "1", UsersCount: 1},
		{ProviderID: "b", Title: "Two", Position: "2", UsersCount: 1},
		{ProviderID: "c", Title: "One, translated", Position: "1", UsersCount: 99},
		{ProviderID: "d", Title: "Three", Position: "3", UsersCount: 1},
	}

	got := collapseSeriesPositions(books)

	want := []string{"c", "b", "d"}
	if len(got) != len(want) {
		t.Fatalf("collapseSeriesPositions() kept %d books, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ProviderID != id {
			t.Errorf("book %d = %q, want %q", i, got[i].ProviderID, id)
		}
	}
}

// Box sets are dropped by the query rather than in Go, because Hardcover files
// them under the position of the first book they contain and there is no way
// to tell one from a volume once it has arrived.
func TestSeriesCatalogQueryExcludesCompilations(t *testing.T) {
	if !strings.Contains(seriesCatalogQuery, "compilation: {_eq: false}") {
		t.Error("GetBooksBySeries no longer excludes compilations")
	}
	if !strings.Contains(seriesCatalogQuery, "users_count: desc_nulls_last") {
		t.Error("GetBooksBySeries no longer orders competing entries by reader count")
	}
}
