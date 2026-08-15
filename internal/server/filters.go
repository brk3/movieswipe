package server

import (
	"encoding/json"
	"net/http"

	"github.com/brk3/movieswipe/internal/catalog"
	"github.com/brk3/movieswipe/internal/store"
)

func handleUpdateFilters(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)

		var filters catalog.Filters
		if err := readJSON(r, &filters); err != nil {
			writeError(w, http.StatusBadRequest, "invalid filters")
			return
		}

		body, err := json.Marshal(filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not encode filters")
			return
		}

		if err := st.SetRoomFilters(r.Context(), mc.Room.ID, string(body)); err != nil {
			writeError(w, http.StatusInternalServerError, "could not update filters")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
