package server

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/brk3/movieswipe/internal/store"
)

type memberDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	SwipeCount int    `json:"swipe_count"`
}

type roomView struct {
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	Filters       json.RawMessage `json:"filters"`
	Members       []memberDTO     `json:"members"`
	UnseenMatches int             `json:"unseen_matches"`
	YouID         int64           `json:"you_id"`
}

func buildRoomView(st *store.Store, r *http.Request, room *store.Room, viewer *store.Member) (*roomView, error) {
	members, err := st.MembersByRoom(r.Context(), room.ID)
	if err != nil {
		return nil, err
	}
	dtos := make([]memberDTO, len(members))
	for i, m := range members {
		count, err := st.CountSwipes(r.Context(), m.ID)
		if err != nil {
			return nil, err
		}
		dtos[i] = memberDTO{ID: m.ID, Name: m.Name, SwipeCount: count}
	}

	unseen, err := st.UnseenMatchCount(r.Context(), room.ID, viewer.MatchesSeenAt)
	if err != nil {
		return nil, err
	}

	return &roomView{
		Code:          room.Code,
		Name:          room.Name,
		Filters:       json.RawMessage(room.Filters),
		Members:       dtos,
		UnseenMatches: unseen,
		YouID:         viewer.ID,
	}, nil
}

func handleCreateRoom(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name        string          `json:"name"`
			DisplayName string          `json:"display_name"`
			Filters     json.RawMessage `json:"filters"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.DisplayName == "" {
			writeError(w, http.StatusBadRequest, "display_name is required")
			return
		}
		filters := string(req.Filters)
		if filters == "" {
			filters = "{}"
		}

		room, err := st.CreateRoom(r.Context(), req.Name, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create room")
			return
		}
		member, err := st.JoinRoom(r.Context(), room.ID, req.DisplayName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not join room")
			return
		}
		view, err := buildRoomView(st, r, room, member)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load room")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"code":  room.Code,
			"token": member.Token,
			"room":  view,
		})
	}
}

func handleJoinRoom(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			DisplayName string `json:"display_name"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.DisplayName == "" {
			writeError(w, http.StatusBadRequest, "display_name is required")
			return
		}

		room, err := st.RoomByCode(r.Context(), r.PathValue("code"))
		if err != nil {
			writeError(w, http.StatusNotFound, "room not found")
			return
		}
		member, err := st.JoinRoom(r.Context(), room.ID, req.DisplayName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not join room")
			return
		}
		view, err := buildRoomView(st, r, room, member)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load room")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"token": member.Token,
			"room":  view,
		})
	}
}

// handleRemoveMember removes a member from the caller's room, e.g. a
// duplicate created by rejoining under a different name, or the caller
// themselves leaving. Removing a member also drops their swipes, so they
// stop counting toward what "everyone" needs to like for a match.
func handleRemoveMember(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)

		id, err := strconv.ParseInt(r.PathValue("memberID"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid member id")
			return
		}

		target, err := st.MemberByID(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "member not found")
			return
		}
		if target.RoomID != mc.Room.ID {
			writeError(w, http.StatusForbidden, "member does not belong to this room")
			return
		}

		if err := st.DeleteMember(r.Context(), id); err != nil {
			writeError(w, http.StatusInternalServerError, "could not remove member")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleGetRoom(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)
		view, err := buildRoomView(st, r, mc.Room, mc.Member)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not load room")
			return
		}
		writeJSON(w, http.StatusOK, view)
	}
}
