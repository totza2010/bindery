package db

import (
	"context"
	"testing"

	"github.com/vavallee/bindery/internal/models"
)

// The sequence a user actually performs: make a series by hand, link it to
// Hardcover, then add a book from that series. Before the link was consulted
// this produced a second series with the same name — the linked one and an
// auto-created twin.
func TestCreateOrGetReusesAHandLinkedSeries(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := NewSeriesRepo(database)

	manual, err := repo.CreateManual(ctx, "Harry Potter")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertHardcoverLink(ctx, &models.SeriesHardcoverLink{
		SeriesID:          manual.ID,
		HardcoverSeriesID: "hc-series:1185",
		HardcoverTitle:    "Harry Potter",
		LinkedBy:          "manual",
	}); err != nil {
		t.Fatal(err)
	}

	// Adding a book brings the provider's series with it.
	fromBook := &models.Series{ForeignID: "hc-series:1185", Title: "Harry Potter"}
	if err := repo.CreateOrGet(ctx, fromBook); err != nil {
		t.Fatal(err)
	}

	if fromBook.ID != manual.ID {
		t.Errorf("CreateOrGet returned series %d, want the linked one (%d)", fromBook.ID, manual.ID)
	}
	all, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("library holds %d series, want the one that was linked", len(all))
	}
	// The link is the identity; the series keeps the foreign_id it was made
	// with rather than having it rewritten underneath the user.
	if all[0].ForeignID != manual.ForeignID {
		t.Errorf("foreign_id changed to %q", all[0].ForeignID)
	}
}

// With no link, the provider's series is created as it always was.
func TestCreateOrGetStillCreatesWhenNothingIsLinked(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := NewSeriesRepo(database)

	if _, err := repo.CreateManual(ctx, "Harry Potter"); err != nil {
		t.Fatal(err)
	}
	fromBook := &models.Series{ForeignID: "hc-series:1185", Title: "Harry Potter"}
	if err := repo.CreateOrGet(ctx, fromBook); err != nil {
		t.Fatal(err)
	}

	all, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Two series named alike, because nothing said they were the same one.
	// Matching on the title instead would be a guess.
	if len(all) != 2 {
		t.Fatalf("library holds %d series, want both the unlinked one and the new one", len(all))
	}
}

// A link belonging to another series must not capture an unrelated ID.
func TestCreateOrGetIgnoresALinkForADifferentSeries(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := NewSeriesRepo(database)

	manual, err := repo.CreateManual(ctx, "Harry Potter")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertHardcoverLink(ctx, &models.SeriesHardcoverLink{
		SeriesID:          manual.ID,
		HardcoverSeriesID: "hc-series:1185",
		LinkedBy:          "manual",
	}); err != nil {
		t.Fatal(err)
	}

	other := &models.Series{ForeignID: "hc-series:997", Title: "The Stormlight Archive"}
	if err := repo.CreateOrGet(ctx, other); err != nil {
		t.Fatal(err)
	}
	if other.ID == manual.ID {
		t.Fatal("an unrelated series was folded into the linked one")
	}
}

// Non-Hardcover identifiers never appear in the link table, so they must not
// pay for a lookup or change behaviour.
func TestCreateOrGetLeavesOtherProvidersAlone(t *testing.T) {
	database, err := OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	repo := NewSeriesRepo(database)

	first := &models.Series{ForeignID: "OL123W", Title: "Discworld"}
	if err := repo.CreateOrGet(ctx, first); err != nil {
		t.Fatal(err)
	}
	again := &models.Series{ForeignID: "OL123W", Title: "Discworld"}
	if err := repo.CreateOrGet(ctx, again); err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID {
		t.Errorf("CreateOrGet made a second row for the same foreign_id")
	}
}
