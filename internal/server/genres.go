package server

import (
	"net/http"

	"github.com/brk3/movieswipe/internal/tmdb"
)

func handleGenres(genres []tmdb.Genre) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, genres)
	}
}
