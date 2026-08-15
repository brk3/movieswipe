package omdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRatings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("i"); got != "tt0000042" {
			t.Errorf("imdb id = %q, want tt0000042", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Response":"True","Ratings":[
			{"Source":"Internet Movie Database","Value":"8.1/10"},
			{"Source":"Rotten Tomatoes","Value":"91%"},
			{"Source":"Metacritic","Value":"80/100"}
		]}`))
	}))
	defer srv.Close()

	c := NewWithBaseURL("test-key", srv.URL)
	r, err := c.Ratings(context.Background(), "tt0000042")
	if err != nil {
		t.Fatal(err)
	}
	if r.IMDb != "8.1/10" || r.RottenTomatoes != "91%" {
		t.Fatalf("unexpected ratings: %+v", r)
	}
}

func TestRatingsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Response":"False","Error":"Incorrect IMDb ID."}`))
	}))
	defer srv.Close()

	c := NewWithBaseURL("test-key", srv.URL)
	r, err := c.Ratings(context.Background(), "ttbad")
	if err != nil {
		t.Fatal(err)
	}
	if r.IMDb != "" || r.RottenTomatoes != "" {
		t.Fatalf("expected empty ratings, got %+v", r)
	}
}
