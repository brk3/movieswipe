package server

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/brk3/movieswipe/internal/push"
	"github.com/brk3/movieswipe/internal/store"
)

func handleSwipe(st *store.Store, p *push.Notifier, logger *slog.Logger) http.HandlerFunc {
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

		matched, err := st.RecordSwipe(r.Context(), mc.Room.ID, mc.Member.ID, req.TmdbID, req.Liked)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not record swipe")
			return
		}
		if matched && p.Enabled() {
			go notifyRoommatesOfMatch(st, p, logger, mc.Room.ID, mc.Member.ID, mc.Member.Name)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// notifyRoommatesOfMatch pushes a notification to every other member of the
// room letting them know they matched with matcherName. Runs detached from
// the request so a slow or failing push provider never delays the swipe.
func notifyRoommatesOfMatch(st *store.Store, p *push.Notifier, logger *slog.Logger, roomID, memberID int64, matcherName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	subs, err := st.PushSubscriptionsForRoomExcept(ctx, roomID, memberID)
	if err != nil {
		logger.Error("load push subscriptions", "err", err)
		return
	}

	title := "New match!"
	body := "You and " + matcherName + " have new matches"
	for _, sub := range subs {
		err := p.Send(ctx, push.Subscription{
			Endpoint: sub.Endpoint,
			P256dh:   sub.P256dh,
			Auth:     sub.Auth,
		}, title, body, "/?tab=lists")
		if err == nil {
			continue
		}
		if _, gone := err.(*push.Gone); gone {
			if delErr := st.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint); delErr != nil {
				logger.Error("delete stale push subscription", "err", delErr)
			}
			continue
		}
		logger.Error("send push notification", "err", err)
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
