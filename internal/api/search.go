package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/vavallee/bindery/internal/metadata"
	"github.com/vavallee/bindery/internal/models"
)

type SearchHandler struct {
	meta *metadata.Aggregator
	// series is optional: without it results carry no series information,
	// which is what the search API did before.
	series SeriesPresence
}

// SeriesPresence answers which provider series IDs the library already holds.
type SeriesPresence interface {
	ForeignIDsInLibrary(ctx context.Context, foreignIDs []string) (map[string]bool, error)
}

func NewSearchHandler(meta *metadata.Aggregator) *SearchHandler {
	return &SearchHandler{meta: meta}
}

// WithSeriesPresence lets book search say which series each result belongs to
// and whether that series is already in the library.
func (h *SearchHandler) WithSeriesPresence(series SeriesPresence) *SearchHandler {
	h.series = series
	return h
}

// bookSearchResult is a book as the search UI needs it: the book itself, plus
// the series it belongs to.
//
// models.Book keeps SeriesRefs out of JSON, and this does not change that —
// every other endpoint serializing a Book is unaffected. The fields are
// flattened to the one series that matters for choosing between results, which
// is the primary one; a book's other series memberships are not what
// distinguishes two rows with the same title.
type bookSearchResult struct {
	*models.Book
	SeriesForeignID string `json:"seriesForeignId,omitempty"`
	SeriesTitle     string `json:"seriesTitle,omitempty"`
	SeriesPosition  string `json:"seriesPosition,omitempty"`
	// SeriesInLibrary is only meaningful when SeriesForeignID is set.
	SeriesInLibrary bool `json:"seriesInLibrary,omitempty"`
}

// primarySeriesRef picks the series a book is chiefly part of, preferring the
// one the provider marked primary and otherwise taking the first that names a
// series at all.
func primarySeriesRef(book models.Book) *models.SeriesRef {
	var fallback *models.SeriesRef
	for i := range book.SeriesRefs {
		ref := &book.SeriesRefs[i]
		if strings.TrimSpace(ref.ForeignID) == "" || strings.TrimSpace(ref.Title) == "" {
			continue
		}
		if ref.Primary {
			return ref
		}
		if fallback == nil {
			fallback = ref
		}
	}
	return fallback
}

// withSeries turns the books into search results, marking the series each
// belongs to and whether the library already holds it.
//
// A failure to read the library is not a failure to search: the results are
// still returned, only without the "already have this" mark, since a search
// that answers nothing is worse than one that answers a little less.
func (h *SearchHandler) withSeries(ctx context.Context, books []models.Book) []bookSearchResult {
	out := make([]bookSearchResult, 0, len(books))
	ids := make([]string, 0, len(books))
	for i := range books {
		result := bookSearchResult{Book: &books[i]}
		if ref := primarySeriesRef(books[i]); ref != nil {
			result.SeriesForeignID = ref.ForeignID
			result.SeriesTitle = ref.Title
			result.SeriesPosition = ref.Position
			ids = append(ids, ref.ForeignID)
		}
		out = append(out, result)
	}
	if h.series == nil || len(ids) == 0 {
		return out
	}

	present, err := h.series.ForeignIDsInLibrary(ctx, ids)
	if err != nil {
		slog.Warn("could not tell which searched series are already in the library", "error", err)
		return out
	}
	for i := range out {
		if out[i].SeriesForeignID != "" && present[out[i].SeriesForeignID] {
			out[i].SeriesInLibrary = true
		}
	}
	return out
}

// writeUpstreamError responds with 502 Bad Gateway and a message that makes
// it obvious the failure is on the metadata provider side (OpenLibrary,
// Google Books, Hardcover), not inside Bindery. Using 500 for this conflates
// provider outages with real server bugs and trains users to ignore 500s.
//
// The client-facing message is intentionally generic and never includes the
// underlying err string. Transport errors from the metadata clients wrap a
// *url.Error whose Error() embeds the full upstream request URL, and the
// Google Books URL carries the API key (?key=...) in the query string plus the
// internal DNS resolver IP. Echoing err.Error() back to the caller therefore
// leaked the API key and internal infra (#1144). The full error is still
// logged server-side so operators keep the detail for debugging.
func writeUpstreamError(w http.ResponseWriter, err error) {
	slog.Warn("metadata provider request failed", "error", err)
	writeJSON(w, http.StatusBadGateway, map[string]string{
		"error": "metadata provider unavailable",
	})
}

func (h *SearchHandler) SearchAuthors(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("term")
	if term == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "term parameter required"})
		return
	}

	authors, err := h.meta.SearchAuthors(r.Context(), term)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, authors)
}

func (h *SearchHandler) SearchBooks(w http.ResponseWriter, r *http.Request) {
	term := r.URL.Query().Get("term")
	if term == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "term parameter required"})
		return
	}

	books, err := h.meta.SearchBooks(r.Context(), term)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, h.withSeries(r.Context(), books))
}

// Lookup resolves a single book by a stable identifier passed as a query
// param: either `isbn` (the original behavior) or `asin` (an Audible/audiobook
// identifier). The route is shared (`/book/lookup`) so existing ISBN callers
// keep working unchanged.
func (h *SearchHandler) Lookup(w http.ResponseWriter, r *http.Request) {
	asin := strings.TrimSpace(r.URL.Query().Get("asin"))
	isbn := r.URL.Query().Get("isbn")

	switch {
	case asin != "":
		h.lookupByASIN(w, r, asin)
	case isbn != "":
		h.lookupByISBN(w, r, isbn)
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "isbn or asin parameter required"})
	}
}

func (h *SearchHandler) lookupByISBN(w http.ResponseWriter, r *http.Request, isbn string) {
	book, err := h.meta.GetBookByISBN(r.Context(), isbn)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	if book == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("No book found for ISBN %s. Check the number, or try searching by title instead.", isbn),
		})
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (h *SearchHandler) lookupByASIN(w http.ResponseWriter, r *http.Request, asin string) {
	book, err := h.meta.GetCanonicalBookByASIN(r.Context(), asin)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	if book == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": fmt.Sprintf("No book found for ASIN %s. Check the identifier, or try searching by title instead.", asin),
		})
		return
	}

	// The resolver canonicalizes the ASIN against the primary provider, so the
	// returned book carries the canonical foreignBookId (keep it) but loses the
	// ASIN-origin shape. Re-stamp the ASIN and audiobook media type so the Add
	// Book modal renders it as the audiobook edition the user searched for.
	if book.ASIN == "" {
		book.ASIN = asin
	}
	book.MediaType = models.MediaTypeAudiobook

	writeJSON(w, http.StatusOK, book)
}

// writeServerError logs the underlying error server-side (with request
// context) and returns a generic 500 body, so internal details like SQL
// text or filesystem paths never reach the client.
func writeServerError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Warn("failed to encode JSON response", "status", status, "error", err)
	}
}
