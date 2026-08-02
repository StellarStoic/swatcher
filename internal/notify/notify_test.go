package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbd-wtf/go-nostr/nip19"
)

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
