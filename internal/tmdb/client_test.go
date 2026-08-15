package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testClient(t *testing.T, path string, response string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing bearer token, got %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != path {
			t.Errorf("path = %q, want %q", r.URL.Path, path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)

	c := New("test-token")
	c.baseURL = srv.URL
	return c
}

func TestGenres(t *testing.T) {
	c := testClient(t, "/genre/movie/list", `{"genres":[{"id":28,"name":"Action"},{"id":35,"name":"Comedy"}]}`)

	genres, err := c.Genres(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(genres) != 2 || genres[0].Name != "Action" {
		t.Fatalf("unexpected genres: %+v", genres)
	}
}

func TestDiscover(t *testing.T) {
	c := testClient(t, "/discover/movie", `{"page":1,"total_pages":500,"results":[{"id":42,"title":"Foo","genre_ids":[28,35]}]}`)

	results, totalPages, err := c.Discover(context.Background(), DiscoverFilters{Page: 1, GenreIDs: []int{28, 35}, MinRating: 6.5})
	if err != nil {
		t.Fatal(err)
	}
	if totalPages != 500 {
		t.Fatalf("totalPages = %d, want 500", totalPages)
	}
	if len(results) != 1 || results[0].ID != 42 {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestMovieDetailTrailerKey(t *testing.T) {
	c := testClient(t, "/movie/42", `{"id":42,"title":"Foo","runtime":120,
		"videos":{"results":[{"key":"abc123","site":"YouTube","type":"Trailer"},{"key":"xyz","site":"YouTube","type":"Teaser"}]},
		"external_ids":{"imdb_id":"tt0000042"}}`)

	d, err := c.MovieDetail(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if d.Runtime != 120 || d.ExternalIDs.IMDbID != "tt0000042" {
		t.Fatalf("unexpected detail: %+v", d)
	}
	if key := d.TrailerKey(); key != "abc123" {
		t.Fatalf("TrailerKey() = %q, want abc123", key)
	}
}
