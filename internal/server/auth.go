package server

import (
	"context"
	"net/http"

	"github.com/brk3/movieswipe/internal/store"
)

type memberContext struct {
	Member *store.Member
	Room   *store.Room
}

type ctxKey int

const memberCtxKey ctxKey = 0

func requireMember(st *store.Store, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Member-Token")
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing X-Member-Token")
			return
		}

		member, err := st.MemberByToken(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		room, err := st.RoomByCode(r.Context(), r.PathValue("code"))
		if err != nil {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}
		if room.ID != member.RoomID {
			writeError(w, http.StatusForbidden, "token does not match room")
			return
		}

		ctx := context.WithValue(r.Context(), memberCtxKey, &memberContext{Member: member, Room: room})
		next(w, r.WithContext(ctx))
	}
}

func memberFromContext(r *http.Request) *memberContext {
	mc, _ := r.Context().Value(memberCtxKey).(*memberContext)
	return mc
}
