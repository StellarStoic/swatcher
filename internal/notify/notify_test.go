// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbd-wtf/go-nostr/nip19"
)

func TestAuthRequiredRecognizesRelayPublishErrors(t *testing.T) {
	for _, message := range []string{
		"auth-required: sign in",
		"msg: auth-required: sign in",
	} {
		if !isAuthRequired(errors.New(message)) {
			t.Fatalf("expected %q to require NIP-42 authentication", message)
		}
	}
	for _, err := range []error{nil, errors.New("msg: restricted: denied")} {
		if isAuthRequired(err) {
			t.Fatalf("did not expect %v to require NIP-42 authentication", err)
		}
	}
}

func TestUniqueRelayURLs(t *testing.T) {
	got := uniqueRelayURLs([]string{" wss://relay.example ", "", "wss://relay.example", "ws://relay2.example"})
	if len(got) != 2 || got[0] != "wss://relay.example" || got[1] != "ws://relay2.example" {
		t.Fatalf("unexpected relay URLs: %#v", got)
	}
}

func TestDisplayRelayURLRemovesCredentialsAndQuery(t *testing.T) {
	got := displayRelayURL("wss://user:secret@relay.example/path?token=secret#fragment")
	if got != "wss://relay.example/path" {
		t.Fatalf("unexpected display URL: %q", got)
	}
}

func TestConfigureTorSOCKSRejectsInvalidAddress(t *testing.T) {
	if err := ConfigureTorSOCKS("not-a-host-port"); err == nil {
		t.Fatal("expected invalid Tor SOCKS address to be rejected")
	}
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
