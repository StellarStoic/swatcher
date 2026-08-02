package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbd-wtf/go-nostr/nip19"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestEnsureIdentityGeneratesAndPersistsNostrKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notifications.json")
	initial := Config{NostrEnabled: true, NostrSenderName: "Watcher"}
	b, _ := json.Marshal(initial)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	sender := Sender{Path: path}
	first, err := sender.EnsureIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.NostrSenderNsec, "nsec1") || !strings.HasPrefix(first.NostrSenderNpub, "npub1") {
		t.Fatalf("identity not generated: %+v", first)
	}
	if prefix, _, err := nip19.Decode(first.NostrSenderNsec); err != nil || prefix != "nsec" {
		t.Fatalf("invalid generated nsec: %v", err)
	}
	if !strings.Contains(first.NostrAvatar, "api.dicebear.com/10.x/pixelbot/svg") {
		t.Fatalf("unexpected avatar: %s", first.NostrAvatar)
	}
	second, err := sender.EnsureIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if second.NostrSenderNsec != first.NostrSenderNsec {
		t.Fatal("sender key changed across reload")
	}
}

func TestDisabledNostrPreservesIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notifications.json")
	initial := Config{NostrEnabled: false, NostrSenderNsec: "preserve-me"}
	b, _ := json.Marshal(initial)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	config, err := (Sender{Path: path}).EnsureIdentity()
	if err != nil {
		t.Fatal(err)
	}
	if config.NostrSenderNsec != "preserve-me" {
		t.Fatal("disabled identity was changed")
	}
}

func TestDeliverTestsClearsSuccessfulTelegramPendingState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notifications.json")
	initial := Config{
		TelegramEnabled:     true,
		TelegramToken:       "test-token",
		TelegramChatID:      "1234",
		TelegramTestPending: true,
	}
	b, _ := json.Marshal(initial)
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
	var delivered map[string]string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &delivered); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: make(http.Header)}, nil
	})}
	sender := Sender{Path: path, HTTP: client}
	message := "You receive this message because you enabled Notifications in s-watcher. Consider this a test message."
	updated, err := sender.DeliverTests(context.Background(), initial, message)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TelegramTestPending {
		t.Fatal("successful Telegram test remained pending")
	}
	if delivered["chat_id"] != "1234" || delivered["text"] != message {
		t.Fatalf("unexpected Telegram request: %#v", delivered)
	}
	persisted, err := sender.Load()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.TelegramTestPending {
		t.Fatal("successful Telegram test state was not persisted")
	}
}
