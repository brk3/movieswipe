package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/brk3/movieswipe/internal/catalog"
	"github.com/brk3/movieswipe/internal/store"
	"github.com/brk3/movieswipe/internal/tmdb"
)

const defaultCardLimit = 10
const maxCardLimit = 50

type cardDTO struct {
	TmdbID     int64    `json:"tmdb_id"`
	Title      string   `json:"title"`
	Year       int      `json:"year,omitempty"`
	Overview   string   `json:"overview"`
	PosterPath string   `json:"poster_path,omitempty"`
	Runtime    int      `json:"runtime,omitempty"`
	Genres     []string `json:"genres,omitempty"`
	TmdbRating float64  `json:"tmdb_rating,omitempty"`
	ImdbRating string   `json:"imdb_rating,omitempty"`
	RtRating   string   `json:"rt_rating,omitempty"`
	TrailerKey string   `json:"trailer_key,omitempty"`
}

func handleCards(st *store.Store, cat *catalog.Catalog, genresByID map[int]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)

		limit := defaultCardLimit
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= maxCardLimit {
				limit = n
			}
		}

		if err := cat.EnsureCards(r.Context(), mc.Room, mc.Member.ID, limit); err != nil {
			writeError(w, http.StatusBadGateway, "could not build deck: "+err.Error())
			return
		}

		movies, err := st.CardsForMember(r.Context(), mc.Room.ID, mc.Member.ID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load cards")
			return
		}
		movies = cat.EnrichCards(r.Context(), movies)

		writeJSON(w, http.StatusOK, cardsFromMovies(movies, genresByID))
	}
}

func cardFromMovie(m store.Movie, genresByID map[int]string) cardDTO {
	return cardDTO{
		TmdbID:     m.TmdbID,
		Title:      m.Title,
		Year:       m.Year,
		Overview:   m.Overview,
		PosterPath: m.PosterPath,
		Runtime:    m.Runtime,
		Genres:     genreNames(m.Genres, genresByID),
		TmdbRating: m.TmdbRating,
		ImdbRating: m.ImdbRating,
		RtRating:   m.RtRating,
		TrailerKey: m.TrailerKey,
	}
}

func genreNames(genresJSON string, genresByID map[int]string) []string {
	if genresJSON == "" {
		return nil
	}
	var ids []int
	if err := json.Unmarshal([]byte(genresJSON), &ids); err != nil {
		return nil
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := genresByID[id]; ok {
			names = append(names, name)
		}
	}
	return names
}

func buildGenreIndex(genres []tmdb.Genre) map[int]string {
	m := make(map[int]string, len(genres))
	for _, g := range genres {
		m[g.ID] = g.Name
	}
	return m
}
