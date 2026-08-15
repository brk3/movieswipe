package catalog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/brk3/movieswipe/internal/omdb"
	"github.com/brk3/movieswipe/internal/store"
	"github.com/brk3/movieswipe/internal/tmdb"
)

func TestEnrichCards(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	var tmdbCalls int32
	tmdbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tmdbCalls, 1)
		id := strings.TrimPrefix(r.URL.Path, "/movie/")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":` + id + `,"runtime":100,
			"videos":{"results":[{"key":"trailerkey","site":"YouTube","type":"Trailer"}]},
			"external_ids":{"imdb_id":"tt` + id + `"}}`))
	}))
	defer tmdbSrv.Close()

	var omdbCalls int32
	omdbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&omdbCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Response":"True","Ratings":[{"Source":"Internet Movie Database","Value":"7.0/10"}]}`))
	}))
	defer omdbSrv.Close()

	tmdbClient := tmdb.NewWithBaseURL("test-token", tmdbSrv.URL)
	omdbClient := omdb.NewWithBaseURL("test-key", omdbSrv.URL)
	cat := New(tmdbClient, omdbClient, st)

	ids := []int64{1, 2, 3}
	for _, id := range ids {
		if err := st.UpsertMovieBasic(ctx, store.Movie{TmdbID: id, Title: "Movie"}); err != nil {
			t.Fatal(err)
		}
	}
	movies := loadMovies(t, st, ids)

	enriched := cat.EnrichCards(ctx, movies)
	for _, m := range enriched {
		if m.Runtime != 100 || m.ImdbRating != "7.0/10" || m.TrailerKey != "trailerkey" {
			t.Fatalf("movie %d not enriched: %+v", m.TmdbID, m)
		}
	}
	if tmdbCalls != 3 || omdbCalls != 3 {
		t.Fatalf("tmdbCalls=%d omdbCalls=%d, want 3 each", tmdbCalls, omdbCalls)
	}

	reloaded := loadMovies(t, st, ids)
	cat.EnrichCards(ctx, reloaded)
	if tmdbCalls != 3 || omdbCalls != 3 {
		t.Fatalf("re-enrichment should be a no-op, got tmdbCalls=%d omdbCalls=%d", tmdbCalls, omdbCalls)
	}
}

func loadMovies(t *testing.T, st *store.Store, ids []int64) []store.Movie {
	t.Helper()
	movies := make([]store.Movie, len(ids))
	for i, id := range ids {
		m, err := st.MovieByID(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		movies[i] = *m
	}
	return movies
}
