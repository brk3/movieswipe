package server

import (
	"net/http"
	"strconv"

	"github.com/brk3/movieswipe/internal/store"
)

func handleSwipe(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)

		var req struct {
			TmdbID int64 `json:"tmdb_id"`
			Liked  bool  `json:"liked"`
		}
		if err := readJSON(r, &req); err != nil || req.TmdbID == 0 {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := st.RecordSwipe(r.Context(), mc.Room.ID, mc.Member.ID, req.TmdbID, req.Liked); err != nil {
			writeError(w, http.StatusInternalServerError, "could not record swipe")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleUndoSwipe(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)

		tmdbID, err := strconv.ParseInt(r.PathValue("tmdbID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid tmdb id")
			return
		}

		if err := st.DeleteSwipe(r.Context(), mc.Member.ID, tmdbID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not undo swipe")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
