package server

import (
	"net/http"

	"github.com/brk3/movieswipe/internal/push"
	"github.com/brk3/movieswipe/internal/store"
)

func handleVAPIDPublicKey(p *push.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.Enabled() {
			writeJSON(w, http.StatusOK, map[string]string{"public_key": ""})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"public_key": p.PublicKey()})
	}
}

func handlePushSubscribe(st *store.Store, p *push.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !p.Enabled() {
			writeError(w, http.StatusServiceUnavailable, "push notifications are not configured")
			return
		}
		mc := memberFromContext(r)

		var req struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
		}
		if err := readJSON(r, &req); err != nil || req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
			writeError(w, http.StatusBadRequest, "invalid subscription")
			return
		}

		if err := st.SavePushSubscription(r.Context(), mc.Member.ID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth); err != nil {
			writeError(w, http.StatusInternalServerError, "could not save subscription")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handlePushUnsubscribe(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mc := memberFromContext(r)

		var req struct {
			Endpoint string `json:"endpoint"`
		}
		if err := readJSON(r, &req); err != nil || req.Endpoint == "" {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := st.DeletePushSubscription(r.Context(), mc.Member.ID, req.Endpoint); err != nil {
			writeError(w, http.StatusInternalServerError, "could not remove subscription")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
