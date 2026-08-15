package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/brk3/movieswipe/internal/store"
	"github.com/brk3/movieswipe/internal/tmdb"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func discoverPage(page int, ids ...int) string {
	results := make([]map[string]any, len(ids))
	for i, id := range ids {
		results[i] = map[string]any{
			"id":           id,
			"title":        fmt.Sprintf("Movie %d", id),
			"overview":     "an overview",
			"release_date": "2020-01-01",
			"vote_average": 7.5,
			"vote_count":   100,
			"genre_ids":    []int{28},
		}
	}
	body, _ := json.Marshal(map[string]any{
		"page":        page,
		"total_pages": 2,
		"results":     results,
	})
	return string(body)
}

func TestEnsureCardsBuildsDeck(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			w.Write([]byte(discoverPage(1, 1, 2, 3)))
		case "2":
			w.Write([]byte(discoverPage(2, 4, 5)))
		default:
			t.Fatalf("unexpected page %q", page)
		}
	}))
	defer srv.Close()

	tmdbClient := tmdb.NewWithBaseURL("test-token", srv.URL)
	cat := New(tmdbClient, nil, st)

	room, err := st.CreateRoom(ctx, "test", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	member, err := st.JoinRoom(ctx, room.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}

	if err := cat.EnsureCards(ctx, room, member.ID, 5); err != nil {
		t.Fatal(err)
	}

	cards, err := st.CardsForMember(ctx, room.ID, member.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 5 {
		t.Fatalf("len(cards) = %d, want 5", len(cards))
	}
	if cards[0].TmdbID != 1 || cards[0].Title != "Movie 1" {
		t.Fatalf("unexpected first card: %+v", cards[0])
	}

	got, err := st.RoomByCode(ctx, room.Code)
	if err != nil {
		t.Fatal(err)
	}
	if got.PageCursor != 2 {
		t.Fatalf("PageCursor = %d, want 2", got.PageCursor)
	}
}

func TestEnsureCardsExcludesSwiped(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(discoverPage(1, 1, 2, 3)))
	}))
	defer srv.Close()

	tmdbClient := tmdb.NewWithBaseURL("test-token", srv.URL)
	cat := New(tmdbClient, nil, st)

	room, err := st.CreateRoom(ctx, "test", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	member, err := st.JoinRoom(ctx, room.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}

	if err := cat.EnsureCards(ctx, room, member.ID, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordSwipe(ctx, room.ID, member.ID, 1, true); err != nil {
		t.Fatal(err)
	}

	cards, err := st.CardsForMember(ctx, room.ID, member.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("len(cards) = %d, want 2", len(cards))
	}
	for _, c := range cards {
		if c.TmdbID == 1 {
			t.Fatal("swiped movie should not be in cards")
		}
	}
}

func TestEnsureCardsAppliesDefaultMinVotes(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	var gotMinVotes string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMinVotes = r.URL.Query().Get("vote_count.gte")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(discoverPage(1, 1, 2, 3)))
	}))
	defer srv.Close()

	tmdbClient := tmdb.NewWithBaseURL("test-token", srv.URL)
	cat := New(tmdbClient, nil, st)

	room, err := st.CreateRoom(ctx, "test", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	member, err := st.JoinRoom(ctx, room.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}

	if err := cat.EnsureCards(ctx, room, member.ID, 3); err != nil {
		t.Fatal(err)
	}
	if gotMinVotes != "50" {
		t.Fatalf("vote_count.gte = %q, want %q", gotMinVotes, "50")
	}
}

func TestEnsureCardsPreservesExplicitMinVotes(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	var gotMinVotes string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMinVotes = r.URL.Query().Get("vote_count.gte")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(discoverPage(1, 1, 2, 3)))
	}))
	defer srv.Close()

	tmdbClient := tmdb.NewWithBaseURL("test-token", srv.URL)
	cat := New(tmdbClient, nil, st)

	room, err := st.CreateRoom(ctx, "test", `{"min_votes": 200}`)
	if err != nil {
		t.Fatal(err)
	}
	member, err := st.JoinRoom(ctx, room.ID, "Alice")
	if err != nil {
		t.Fatal(err)
	}

	if err := cat.EnsureCards(ctx, room, member.ID, 3); err != nil {
		t.Fatal(err)
	}
	if gotMinVotes != "200" {
		t.Fatalf("vote_count.gte = %q, want %q", gotMinVotes, "200")
	}
}
