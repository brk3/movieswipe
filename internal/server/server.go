package server

import (
	"embed"
	"log/slog"
	"net/http"

	"github.com/brk3/movieswipe/internal/catalog"
	"github.com/brk3/movieswipe/internal/push"
	"github.com/brk3/movieswipe/internal/store"
	"github.com/brk3/movieswipe/internal/tmdb"
)

type Deps struct {
	Logger  *slog.Logger
	WebFS   embed.FS
	Store   *store.Store
	Catalog *catalog.Catalog
	Genres  []tmdb.Genre
	Push    *push.Notifier
}

func New(d Deps) http.Handler {
	genresByID := buildGenreIndex(d.Genres)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /api/genres", handleGenres(d.Genres))
	mux.HandleFunc("POST /api/rooms", handleCreateRoom(d.Store))
	mux.HandleFunc("POST /api/rooms/{code}/join", handleJoinRoom(d.Store))
	mux.HandleFunc("GET /api/rooms/{code}", requireMember(d.Store, handleGetRoom(d.Store)))
	mux.HandleFunc("PATCH /api/rooms/{code}/filters", requireMember(d.Store, handleUpdateFilters(d.Store)))
	mux.HandleFunc("GET /api/rooms/{code}/cards", requireMember(d.Store, handleCards(d.Store, d.Catalog, genresByID)))
	mux.HandleFunc("POST /api/rooms/{code}/swipes", requireMember(d.Store, handleSwipe(d.Store, d.Push, d.Logger)))
	mux.HandleFunc("DELETE /api/rooms/{code}/swipes/{tmdbID}", requireMember(d.Store, handleUndoSwipe(d.Store)))
	mux.HandleFunc("GET /api/rooms/{code}/lists", requireMember(d.Store, handleLists(d.Store, genresByID)))
	mux.HandleFunc("POST /api/rooms/{code}/lists/seen", requireMember(d.Store, handleMarkMatchesSeen(d.Store)))
	mux.HandleFunc("GET /api/push/vapid-public-key", handleVAPIDPublicKey(d.Push))
	mux.HandleFunc("POST /api/rooms/{code}/push/subscribe", requireMember(d.Store, handlePushSubscribe(d.Store, d.Push)))
	mux.HandleFunc("DELETE /api/rooms/{code}/push/subscribe", requireMember(d.Store, handlePushUnsubscribe(d.Store)))
	mux.Handle("GET /", webHandler(d.WebFS))

	return withMiddleware(mux, d.Logger)
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
