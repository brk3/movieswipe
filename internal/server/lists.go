package server

import (
	"net/http"

	"github.com/brk3/movieswipe/internal/store"
)

type memberLikesDTO struct {
	ID    int64     `json:"id"`
	Name  string    `json:"name"`
	Likes []cardDTO `json:"likes"`
}

type listsResponse struct {
	Members []memberLikesDTO `json:"members"`
	Matches []cardDTO        `json:"matches"`
}

func handleLists(st *store.Store, genresByID map[int]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)

		members, err := st.MembersByRoom(r.Context(), mc.Room.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load room")
			return
		}

		resp := listsResponse{Members: make([]memberLikesDTO, len(members))}
		for i, m := range members {
			likes, err := st.LikesByMember(r.Context(), m.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "could not load likes")
				return
			}
			resp.Members[i] = memberLikesDTO{ID: m.ID, Name: m.Name, Likes: cardsFromMovies(likes, genresByID)}
		}

		matches, err := st.Matches(r.Context(), mc.Room.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load matches")
			return
		}
		resp.Matches = cardsFromMovies(matches, genresByID)

		writeJSON(w, http.StatusOK, resp)
	}
}

func handleMarkMatchesSeen(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)
		if err := st.StampMatchesSeen(r.Context(), mc.Member.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "could not update")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func cardsFromMovies(movies []store.Movie, genresByID map[int]string) []cardDTO {
	cards := make([]cardDTO, len(movies))
	for i, m := range movies {
		cards[i] = cardFromMovie(m, genresByID)
	}
	return cards
}
