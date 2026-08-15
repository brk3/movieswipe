package server

import (
	"encoding/json"
	"net/http"

	"github.com/brk3/movieswipe/internal/store"
)

type memberDTO struct {
	Name       string `json:"name"`
	SwipeCount int    `json:"swipe_count"`
}

type roomView struct {
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	Filters       json.RawMessage `json:"filters"`
	Members       []memberDTO     `json:"members"`
	UnseenMatches int             `json:"unseen_matches"`
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
		dtos[i] = memberDTO{Name: m.Name, SwipeCount: count}
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
