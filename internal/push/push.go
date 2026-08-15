// Package push sends Web Push notifications to subscribed browsers.
package push

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Gone reports whether the push service says a subscription is no longer
// valid, so callers know to delete it.
type Gone struct{ StatusCode int }

func (e *Gone) Error() string {
	return fmt.Sprintf("push subscription gone (status %d)", e.StatusCode)
}

type Notifier struct {
	publicKey  string
	privateKey string
	subject    string
}

func New(publicKey, privateKey, subject string) *Notifier {
	return &Notifier{publicKey: publicKey, privateKey: privateKey, subject: subject}
}

func (n *Notifier) Enabled() bool {
	return n != nil && n.publicKey != "" && n.privateKey != ""
}

func (n *Notifier) PublicKey() string {
	if n == nil {
		return ""
	}
	return n.publicKey
}

type Subscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

func (n *Notifier) Send(ctx context.Context, sub Subscription, title, body, url string) error {
	payload, err := json.Marshal(map[string]string{"title": title, "body": body, "url": url})
	if err != nil {
		return err
	}

	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		Subscriber:      n.subject,
		VAPIDPublicKey:  n.publicKey,
		VAPIDPrivateKey: n.privateKey,
		TTL:             60,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return &Gone{StatusCode: resp.StatusCode}
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("push service returned status %d", resp.StatusCode)
	}
	return nil
}
