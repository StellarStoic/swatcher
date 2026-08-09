// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/s-watcher/s-watcher/internal/bitcoin"
	"github.com/s-watcher/s-watcher/internal/electrum"
	"github.com/s-watcher/s-watcher/internal/notify"
	"github.com/s-watcher/s-watcher/internal/webauth"
)

func multipartWatchRequest(t *testing.T, values url.Values) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, entries := range values {
		for _, value := range entries {
			if err := writer.WriteField(name, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/watches", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	return request
}

func TestDirection(t *testing.T) {
	tests := []struct {
		effect electrum.Effect
		want   string
	}{
		{electrum.Effect{Received: 1}, "received"},
		{electrum.Effect{Sent: 1}, "sent"},
		{electrum.Effect{Received: 2, Sent: 1}, "self-transfer"},
		{electrum.Effect{}, "activity"},
	}
	for _, test := range tests {
		if got := direction(test.effect); got != test.want {
			t.Fatalf("direction(%+v) = %q, want %q", test.effect, got, test.want)
		}
	}
}

func TestMovementSignal(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{Event{Received: 2, Sent: 1}, "received"},
		{Event{Received: 1, Sent: 2}, "sent"},
		{Event{Received: 1, Sent: 1}, "idle"},
		{Event{}, "idle"},
	}
	for _, test := range tests {
		if got := movementSignal(test.event); got != test.want {
			t.Fatalf("movementSignal(%+v) = %q, want %q", test.event, got, test.want)
		}
	}
}

func TestNotificationRules(t *testing.T) {
	tests := []struct {
		name     string
		group    WatchGroup
		event    Event
		tip      int64
		eligible bool
		waiting  bool
	}{
		{name: "default mempool", event: Event{Direction: "received", Received: 1}, eligible: true},
		{name: "incoming excludes sent", group: WatchGroup{NotifyMode: "incoming"}, event: Event{Direction: "sent", Sent: 20}},
		{name: "minimum", group: WatchGroup{NotifyMinimum: 10_000}, event: Event{Direction: "received", Received: 9_999}},
		{name: "waits for confirmation", group: WatchGroup{NotifyAfter: 1}, event: Event{Direction: "received", Received: 10}, waiting: true},
		{name: "waits for three", group: WatchGroup{NotifyAfter: 3}, event: Event{Direction: "received", Received: 10, Height: 100}, tip: 101, waiting: true},
		{name: "three confirmations", group: WatchGroup{NotifyAfter: 3}, event: Event{Direction: "received", Received: 10, Height: 100}, tip: 102, eligible: true},
		{name: "disabled", group: WatchGroup{NotifyMode: "off"}, event: Event{Direction: "received", Received: 10}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			eligible, waiting := notificationEligible(test.group, test.event, test.tip)
			if eligible != test.eligible || waiting != test.waiting {
				t.Fatalf("notificationEligible() = %v, %v; want %v, %v", eligible, waiting, test.eligible, test.waiting)
			}
		})
	}
}

func TestNotificationScheduleHelpers(t *testing.T) {
	c := notify.Config{QuietStart: 22, QuietEnd: 7, UTCOffset: 2}
	if !notificationQuietNow(c, time.Date(2026, 8, 4, 21, 0, 0, 0, time.UTC)) {
		t.Fatal("wrapped quiet hours did not include local 23:00")
	}
	if notificationQuietNow(c, time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("quiet hours included local noon")
	}
	message := digestMessage(
		[]Event{{GroupID: "g", Direction: "received", Received: 42, Net: 42, TxID: "txid"}},
		map[string]string{"g": "donations / website"},
		map[string]notificationBalance{"g": {Confirmed: 1234}},
		0,
		"",
	)
	if !strings.Contains(message, "**s/watcher daily digest**") || !strings.Contains(message, "**Activities:** 1") || !strings.Contains(message, "donations / website") || !strings.Contains(message, "42 sat") {
		t.Fatalf("unexpected digest: %s", message)
	}
}

func TestActivityMessageIncludesTrackedDetailsAndOnionTransactionLink(t *testing.T) {
	event := Event{
		Direction:    "received",
		Received:     123_456_789,
		Net:          123_456_789,
		TxID:         strings.Repeat("a", 64),
		Height:       900,
		Replaceable:  true,
		Runestone:    true,
		Inscriptions: 2,
		OPReturn:     []string{"thank you", "invoice 42"},
		SeenAt:       time.Date(2026, 8, 6, 12, 30, 0, 0, time.UTC),
	}
	message := activityMessage(event, "donations / website", notificationBalance{Confirmed: 255_000_000, Unconfirmed: 2_000_000}, 902, "http://privateexplorer.onion/")
	for _, expected := range []string{
		"**Watch:** donations / website",
		"**Activity:** Incoming",
		"**Received:** 1.23456789 BTC",
		"**Sent:** 0 sat",
		"**Net change:** +1.23456789 BTC",
		"block 900 · 3 confirmations",
		"**Current balance:** 2.55 BTC · +0.02 BTC pending",
		"**Runes:** Runestone detected",
		"**Inscriptions:** 2 inscription envelopes detected",
		"**OP_RETURN:**\n• thank you\n• invoice 42",
		"**Detected:** 2026-08-06 12:30 UTC",
		"**Transaction:** http://privateexplorer.onion/tx/" + strings.Repeat("a", 64),
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("rich activity message is missing %q:\n%s", expected, message)
		}
	}
	if strings.Contains(message, "**RBF:**") {
		t.Fatalf("confirmed transaction must not show an RBF warning: %s", message)
	}
}

func TestBitcoinAmountFormatting(t *testing.T) {
	tests := []struct {
		name string
		sats uint64
		want string
	}{
		{name: "zero", sats: 0, want: "0 sat"},
		{name: "below threshold", sats: 999_999, want: "999999 sat"},
		{name: "threshold", sats: 1_000_000, want: "0.01 BTC"},
		{name: "eight decimals", sats: 1_234_567, want: "0.01234567 BTC"},
		{name: "whole bitcoin", sats: 100_000_000, want: "1 BTC"},
		{name: "trimmed decimals", sats: 123_450_000, want: "1.2345 BTC"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatBitcoinAmount(test.sats); got != test.want {
				t.Fatalf("formatBitcoinAmount(%d) = %q; want %q", test.sats, got, test.want)
			}
		})
	}
	if got := formatSignedBitcoinAmount(-2_000_000, false); got != "-0.02 BTC" {
		t.Fatalf("unexpected negative amount: %q", got)
	}
	if got := formatSignedBitcoinAmount(42, true); got != "+42 sat" {
		t.Fatalf("unexpected signed sat amount: %q", got)
	}
}

func TestEventAmountUsesSharedUnits(t *testing.T) {
	if got := eventAmount(Event{Direction: "received", Received: 1_000_000}); got != "+0.01 BTC" {
		t.Fatalf("unexpected received amount: %q", got)
	}
	if got := eventAmount(Event{Direction: "sent", Sent: 999_999}); got != "-999999 sat" {
		t.Fatalf("unexpected sent amount: %q", got)
	}
	if got := eventAmount(Event{Direction: "self-transfer", Received: 2_000_000, Sent: 1_000_000, Net: 1_000_000}); got != "net 0.01 BTC · in 0.02 BTC / out 0.01 BTC" {
		t.Fatalf("unexpected self-transfer amount: %q", got)
	}
}

func TestActivityMessageUsesPlainTxIDWithoutOnionAndWarnsForRBF(t *testing.T) {
	event := Event{Direction: "received", Received: 900, Net: 900, TxID: "plain-txid", Replaceable: true}
	message := activityMessage(event, "cold / reserve", notificationBalance{Confirmed: 100}, 0, "")
	for _, expected := range []string{"**Activity:** Incoming", "**State:** Unconfirmed · mempool", "**RBF:** Replaceable", "**Transaction:** `plain-txid`"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("plain activity message is missing %q: %s", expected, message)
		}
	}
	if strings.Contains(message, "/tx/") || strings.Contains(message, "http://") || strings.Contains(message, "https://") {
		t.Fatalf("transaction became linkable without an onion interface: %s", message)
	}
}

func TestActivityMessageEscapesUntrustedMarkdown(t *testing.T) {
	event := Event{Direction: "received", TxID: "tx_id", OPReturn: []string{"**fake label** [link](url) `code` ~~gone~~"}}
	message := activityMessage(event, "cold_wallet *mine*", notificationBalance{}, 0, "")
	for _, expected := range []string{
		"**Watch:** cold\\_wallet \\*mine\\*",
		"• \\*\\*fake label\\*\\* \\[link\\]\\(url\\) \\`code\\` \\~\\~gone\\~\\~",
		"**Transaction:** `tx_id`",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("escaped message is missing %q:\n%s", expected, message)
		}
	}
}

func TestMempoolOnionBaseIgnoresLANAndClearnetURLs(t *testing.T) {
	urls := []string{"http://mempool.local", "https://mempool.example", "http://privateexplorer.onion/"}
	if got := mempoolOnionBase(urls); got != "http://privateexplorer.onion" {
		t.Fatalf("unexpected onion base %q", got)
	}
	if got := mempoolOnionBase(urls[:2]); got != "" {
		t.Fatalf("non-onion URL was selected for notifications: %q", got)
	}
}

func TestWatchSortModes(t *testing.T) {
	older := time.Unix(100, 0)
	newer := time.Unix(200, 0)
	tests := map[string]string{
		"balance":  "b",
		"name":     "b",
		"group":    "b",
		"date":     "b",
		"activity": "a",
		"type":     "b",
	}
	for mode, want := range tests {
		groups := []groupView{
			{WatchGroup: WatchGroup{ID: "a", Label: "beta", Category: "zeta", ScriptType: "legacy", CreatedAt: older}, Confirmed: 1, LastActivity: newer},
			{WatchGroup: WatchGroup{ID: "b", Label: "alpha", Category: "alpha", ScriptType: "address", CreatedAt: newer}, Confirmed: 2, LastActivity: older},
		}
		sortGroups(groups, mode)
		if groups[0].ID != want {
			t.Fatalf("sort %s placed %s first, want %s", mode, groups[0].ID, want)
		}
	}
}

func TestTransactionSortModes(t *testing.T) {
	older := time.Unix(100, 0)
	newer := time.Unix(200, 0)
	tests := []struct {
		mode   string
		events []eventView
		want   string
	}{
		{mode: "newest", events: []eventView{{Event: Event{TxID: "old", SeenAt: older}}, {Event: Event{TxID: "new", SeenAt: newer}}}, want: "new"},
		{mode: "oldest", events: []eventView{{Event: Event{TxID: "old", SeenAt: older}}, {Event: Event{TxID: "new", SeenAt: newer}}}, want: "old"},
		{mode: "largest", events: []eventView{{Event: Event{TxID: "small", Received: 10}}, {Event: Event{TxID: "large", Sent: 20}}}, want: "large"},
		{mode: "smallest", events: []eventView{{Event: Event{TxID: "small", Received: 10}}, {Event: Event{TxID: "large", Sent: 20}}}, want: "small"},
		{mode: "incoming", events: []eventView{{Event: Event{TxID: "sent", Direction: "sent"}}, {Event: Event{TxID: "received", Direction: "received"}}}, want: "received"},
		{mode: "outgoing", events: []eventView{{Event: Event{TxID: "received", Direction: "received"}}, {Event: Event{TxID: "sent", Direction: "sent"}}}, want: "sent"},
		{mode: "mempool", events: []eventView{{Event: Event{TxID: "confirmed", Height: 10}}, {Event: Event{TxID: "mempool"}}}, want: "mempool"},
		{mode: "confirmed", events: []eventView{{Event: Event{TxID: "mempool"}}, {Event: Event{TxID: "confirmed", Height: 10}}}, want: "confirmed"},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			sortTransactionEvents(test.events, test.mode)
			if test.events[0].TxID != test.want {
				t.Fatalf("sort %s placed %s first; want %s", test.mode, test.events[0].TxID, test.want)
			}
		})
	}
}

func TestTransactionHistoryPaginatesOneHundredEvents(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	a.mempoolURLs = []string{"http://mempool.local"}
	a.state.Groups = []WatchGroup{{ID: "group", Label: "savings", Category: "cold", Source: "bulk-address-list", ScriptType: "collection", Count: 1}}
	a.state.Watches = []Watch{{ID: "watch", GroupID: "group", Confirmed: 200_000_000, Initialized: true, LastTxID: "tx-204", LastTxAt: time.Unix(204, 0).UTC()}}
	for index := 0; index < 205; index++ {
		a.state.Events = append(a.state.Events, Event{
			WatchID:              "watch",
			GroupID:              "group",
			TxID:                 fmt.Sprintf("tx-%03d", index),
			Direction:            "received",
			Received:             uint64(index + 1),
			Net:                  int64(index + 1),
			Height:               int64(index + 1),
			Runestone:            index == 100,
			Inscriptions:         map[bool]int{true: 2}[index == 100],
			OPReturn:             map[bool][]string{true: {"history note"}}[index == 100],
			SpentAddresses:       map[bool][]string{true: {"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"}}[index == 100],
			DestinationAddresses: map[bool][]string{true: {"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy"}}[index == 100],
			SeenAt:               time.Unix(int64(index), 0).UTC(),
		})
	}
	indexResponse := httptest.NewRecorder()
	a.index(indexResponse, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(indexResponse.Body.String(), `href="/groups/group/transactions">Show all transactions`) {
		t.Fatal("watch row did not render its transaction-history link")
	}
	request := httptest.NewRequest(http.MethodGet, "/groups/group/transactions?sort=oldest&page=2", nil)
	request.SetPathValue("id", "group")
	response := httptest.NewRecorder()
	a.groupTransactions(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, body)
	}
	for _, expected := range []string{"205</strong> transactions", "page 2 of 3", "data-txid=\"tx-100\"", "tx-199", "Previous 100", "Next 100", "Runes · runestone detected", "2 inscription envelopes", "history note", "From watched address", "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", "To address", "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy", "http://mempool.local", "2 BTC"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("history page is missing %q", expected)
		}
	}
	for _, excluded := range []string{"tx-099", "tx-200"} {
		if strings.Contains(body, excluded) {
			t.Fatalf("history page included event outside page two: %s", excluded)
		}
	}
	if count := strings.Count(body, "class=\"transaction-card\""); count != 100 {
		t.Fatalf("history page rendered %d transactions; want 100", count)
	}
}

func TestTransactionHistoryPrivacyModeMasksAmountsAndTxIDs(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	txID := strings.Repeat("a", 64)
	receivedAddress := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	destinationAddress := "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy"
	a.state = state{
		PrivacyMode: true,
		Groups:      []WatchGroup{{ID: "group", Label: "savings", Category: "cold", Source: "address", ScriptType: "address", Count: 1}},
		Watches:     []Watch{{ID: "watch", GroupID: "group", Confirmed: 200_000_000, Initialized: true}},
		Events:      []Event{{WatchID: "watch", GroupID: "group", TxID: txID, Direction: "self-transfer", Received: 200_000_000, Net: 200_000_000, ReceivedAddresses: []string{receivedAddress}, SpentAddresses: []string{receivedAddress}, DestinationAddresses: []string{destinationAddress}, SeenAt: time.Now()}},
	}
	request := httptest.NewRequest(http.MethodGet, "/groups/group/transactions", nil)
	request.SetPathValue("id", "group")
	response := httptest.NewRecorder()
	a.groupTransactions(response, request)
	body := response.Body.String()
	for _, secret := range []string{txID, receivedAddress, destinationAddress, "2 BTC", "200000000"} {
		if strings.Contains(body, secret) {
			t.Fatalf("privacy history exposed %q", secret)
		}
	}
	if !strings.Contains(body, maskIdentifier(txID)) || !strings.Contains(body, maskIdentifier(receivedAddress)) || !strings.Contains(body, maskIdentifier(destinationAddress)) || !strings.Contains(body, "masked") {
		t.Fatal("privacy history did not render masked values")
	}
}

func TestTransactionHistoryListsPendingElectrsDetails(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	txID := strings.Repeat("b", 64)
	a.state = state{
		Groups:  []WatchGroup{{ID: "group", Label: "archive", Category: "old", Source: "address", ScriptType: "address", Count: 1}},
		Watches: []Watch{{ID: "watch", GroupID: "group", Initialized: true}},
		Events:  []Event{{WatchID: "watch", GroupID: "group", TxID: txID, Height: 800_000, Direction: "activity", DetailsPending: true, Historical: true, SeenAt: time.Now()}},
	}
	request := httptest.NewRequest(http.MethodGet, "/groups/group/transactions", nil)
	request.SetPathValue("id", "group")
	response := httptest.NewRecorder()
	a.groupTransactions(response, request)
	body := response.Body.String()
	for _, expected := range []string{"1</strong> transaction", txID, "Indexed transaction", "Details pending", "Full details will be retried"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("pending history is missing %q: %s", expected, body)
		}
	}
	if strings.Contains(body, "No transactions were returned") {
		t.Fatal("pending Electrs transaction was rendered as an empty history")
	}
}

func TestHistoryImportNotificationClassification(t *testing.T) {
	for _, test := range []struct {
		name        string
		initialized bool
		known       bool
		want        bool
	}{
		{name: "initial scan", initialized: false, known: false, want: false},
		{name: "upgrade backfill", initialized: true, known: true, want: false},
		{name: "new transaction", initialized: true, known: false, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldNotifyHistoryEvent(test.initialized, test.known); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestPrivacyModeMasksRenderedSensitiveValues(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	address := "1BoatSLRHtKNngkdXEeobR76b53LETtpyT"
	a.state = state{
		PrivacyMode: true,
		Groups:      []WatchGroup{{ID: "group", Label: "Savings", Category: "Cold", Notes: "hardware wallet stored upstairs", Source: address, ScriptType: "address", Count: 1}},
		Watches:     []Watch{{ID: "watch", GroupID: "group", Label: "Savings", Address: address, Confirmed: 123456, Initialized: true}},
		Events:      []Event{{WatchID: "watch", GroupID: "group", TxID: strings.Repeat("a", 64), Direction: "received", Received: 5000, SeenAt: time.Now()}},
	}
	response := httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, secret := range []string{address, "123456 sat", "5000 sat", strings.Repeat("a", 64), "hardware wallet stored upstairs"} {
		if strings.Contains(body, secret) {
			t.Fatalf("privacy response exposed %q", secret)
		}
	}
	if !strings.Contains(body, maskIdentifier(address)) || !strings.Contains(body, maskIdentifier(strings.Repeat("a", 64))) {
		t.Fatalf("privacy response did not retain four-character edges: %s", body)
	}
	if !strings.ContainsAny(body, "🬀🬁🬂🬃🬄🬅🬆🬇🬈🬉🬊🬋🬌🬍🬎🬏🬐🬑🬒🬓🬔🬕🬖🬗🬘🬙🬚🬛🬜🬝🬞🬟") {
		t.Fatalf("privacy response did not use legacy-computing masks: %s", body)
	}
	if !strings.Contains(body, "signal-received") {
		t.Fatalf("latest received activity did not render a green signal: %s", body)
	}
}

func TestTemplatesEscapeStoredValues(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	payload := `<script>alert("xss")</script>`
	a.state.Groups = []WatchGroup{{ID: "group", Label: payload, Category: payload, Notes: payload, Source: payload, ScriptType: "address"}}
	a.state.Events = []Event{
		{GroupID: "group", TxID: strings.Repeat("a", 64), Direction: "received", OPReturn: []string{payload}, Replaceable: true, Runestone: true, Inscriptions: 2, SeenAt: time.Now()},
		{GroupID: "group", TxID: strings.Repeat("b", 64), Height: 1, Direction: "received", Replaceable: true, SeenAt: time.Now()},
	}
	response := httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	if strings.Contains(body, payload) || !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "op-return") || !strings.Contains(body, "OP_RETURN") || !strings.Contains(body, "Runes · runestone detected") || !strings.Contains(body, "Ordinals · 2 inscription envelopes detected") || strings.Count(body, "Replaceable — do not treat as final until confirmed.") != 1 {
		t.Fatalf("stored values were not safely HTML-escaped: %s", body)
	}
}

func TestSecurityHeadersAndCrossSitePostProtection(t *testing.T) {
	called := false
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if cspNonce(r) == "" {
			t.Fatal("request context did not receive a CSP nonce")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	csp := response.Header().Get("Content-Security-Policy")
	if response.Code != http.StatusNoContent || !strings.Contains(csp, "script-src 'nonce-") || strings.Contains(csp, "script-src 'unsafe-inline'") || response.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("unexpected security headers: %v", response.Header())
	}
	called = false
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("password=12345"))
	request.Header.Set("Origin", "https://attacker.example")
	request.Host = "s-watcher.local"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || called {
		t.Fatalf("cross-site POST was not rejected: status=%d called=%v", response.Code, called)
	}

	called = false
	request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("password=12345"))
	request.Header.Set("Origin", "http://examplehiddenservice.onion")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	request.Host = "s-watcher.internal"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !called {
		t.Fatalf("same-origin Tor proxy POST was rejected: status=%d called=%v", response.Code, called)
	}

	called = false
	request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader("password=12345"))
	request.Header.Set("Origin", "https://attacker.example")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	request.Host = "s-watcher.internal"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || called {
		t.Fatalf("fetch-metadata cross-site POST was not rejected: status=%d called=%v", response.Code, called)
	}
}

func TestSetPrivacyModeRequiresPasswordConfigurationAndVerifiesDisable(t *testing.T) {
	dataDir := t.TempDir()
	if err := SetPrivacyMode(dataDir, true, ""); err == nil || !strings.Contains(err.Error(), "Web Password") {
		t.Fatalf("enabling without a password should fail clearly: %v", err)
	}
	if err := webauth.SetPassword(filepath.Join(dataDir, "auth.json"), "12345"); err != nil {
		t.Fatal(err)
	}
	if err := SetPrivacyMode(dataDir, true, ""); err != nil {
		t.Fatalf("enable privacy mode: %v", err)
	}
	if err := SetPrivacyMode(dataDir, false, "wrong"); err == nil || !strings.Contains(err.Error(), "incorrect") {
		t.Fatalf("disabling with the wrong password should fail: %v", err)
	}
	if err := SetPrivacyMode(dataDir, false, "12345"); err != nil {
		t.Fatalf("disable privacy mode: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dataDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved state
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.PrivacyMode {
		t.Fatal("privacy mode remained enabled after password verification")
	}
}

func TestSetDiscoveryGapPersistsValidatedValue(t *testing.T) {
	dataDir := t.TempDir()
	if err := SetDiscoveryGap(dataDir, 0); err == nil {
		t.Fatal("accepted a zero discovery gap")
	}
	if err := SetDiscoveryGap(dataDir, 37); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dataDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved state
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.DiscoveryGap != 37 {
		t.Fatalf("saved discovery gap %d, want 37", saved.DiscoveryGap)
	}
}

func TestSetPrivacyIndicatorsPersistsSettings(t *testing.T) {
	dataDir := t.TempDir()
	if err := SetPrivacyIndicators(dataDir, 0, true, true, true); err == nil {
		t.Fatal("accepted a zero small-deposit threshold")
	}
	if err := SetPrivacyIndicators(dataDir, 2_500, true, false, true); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dataDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved state
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.PrivacyIndicatorsConfigured || !saved.AddressReuseIndicators || saved.SmallDepositIndicators || !saved.CombinedWalletIndicators || saved.SmallDepositThreshold != 2_500 {
		t.Fatalf("unexpected privacy indicator settings: %+v", saved)
	}
}

func TestPrivacyIndicatorsRenderAsInformationalBadges(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	a.state.Groups = []WatchGroup{{ID: "g1", Label: "one"}, {ID: "g2", Label: "two"}}
	a.state.Events = []Event{
		{GroupID: "g1", TxID: "first", Direction: "received", Received: 500, ReceivedAddresses: []string{"bc1qreuse"}, SeenAt: time.Now()},
		{GroupID: "g1", TxID: "shared", Direction: "received", Received: 600, ReceivedAddresses: []string{"bc1qreuse"}, SeenAt: time.Now()},
		{GroupID: "g2", TxID: "shared", Direction: "sent", Sent: 600, SeenAt: time.Now()},
	}
	response := httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, text := range []string{"Reused address · 2 observed receipts", "Small deposit · below 1000 sat", "Combined funds from 2 watched wallets"} {
		if !strings.Contains(body, text) {
			t.Fatalf("missing privacy indicator %q: %s", text, body)
		}
	}
}

func TestSmartDiscoveryReportsUnsatisfiedSafetyLimit(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	a.state.DiscoveryGap = 20
	a.state.Groups = []WatchGroup{{ID: "group", Source: "unused", ScriptType: "native-segwit", Count: 500}}
	a.state.Watches = []Watch{{ID: "watch", GroupID: "group", Path: "m/0/499", Address: "address", KnownTx: []string{"used"}}}
	if err := a.expandDiscoveryLocked(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.state.Groups[0].DiscoveryError, "500-address limit") {
		t.Fatalf("missing discovery-limit warning: %q", a.state.Groups[0].DiscoveryError)
	}
}

func TestLoginCreatesAuthenticatedSession(t *testing.T) {
	dataDir := t.TempDir()
	a, err := New(dataDir, "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := webauth.SetPassword(a.authPath, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	loginPageResponse := httptest.NewRecorder()
	a.loginPage(loginPageResponse, httptest.NewRequest(http.MethodGet, "/login", nil))
	if body := loginPageResponse.Body.String(); !strings.Contains(body, "Forgot password?") || !strings.Contains(body, "Set Web Password") {
		t.Fatalf("login page is missing recovery guidance: %s", body)
	}
	form := url.Values{"password": {"correct horse battery staple"}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	a.login(response, request)
	result := response.Result()
	if result.StatusCode != http.StatusSeeOther || len(result.Cookies()) != 1 || !result.Cookies()[0].HttpOnly || result.Cookies()[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected login response: status=%d cookies=%+v", result.StatusCode, result.Cookies())
	}
	protectedResponse := httptest.NewRecorder()
	protectedRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	protectedRequest.AddCookie(result.Cookies()[0])
	a.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(protectedResponse, protectedRequest)
	if protectedResponse.Code != http.StatusNoContent {
		t.Fatalf("authenticated request returned %d", protectedResponse.Code)
	}
}

func TestAddExtendedKeyGroupAndRender(t *testing.T) {
	master, err := hdkeychain.NewMaster([]byte("s-watcher app import test"), &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	public, err := master.Neuter()
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{
		"label":          {"Test Wallet_1"},
		"category":       {"Cold Storage"},
		"notes":          {"Long-term savings; verify annually."},
		"source":         {public.String()},
		"script_type":    {"native-segwit"},
		"include_change": {"on"},
	}
	request := httptest.NewRequest(http.MethodPost, "/watches", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	a.addWatch(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("add status %d: %s", response.Code, response.Body.String())
	}
	if len(a.state.Groups) != 1 || a.state.Groups[0].Count != defaultDiscoveryGap || a.state.Groups[0].Notes != "Long-term savings; verify annually." || len(a.state.Watches) != defaultDiscoveryGap*2 {
		t.Fatalf("unexpected imported state: %d groups, %d watches", len(a.state.Groups), len(a.state.Watches))
	}
	for i := range a.state.Watches {
		if a.state.Watches[i].Path == "m/0/19" {
			a.state.Watches[i].KnownTx = []string{"used"}
		}
	}
	if err := a.expandDiscoveryLocked(); err != nil {
		t.Fatal(err)
	}
	if a.state.Groups[0].Count != 40 || len(a.state.Watches) != 80 {
		t.Fatalf("smart discovery did not extend both branches: count=%d watches=%d", a.state.Groups[0].Count, len(a.state.Watches))
	}
	a.state.DiscoveryGap = 5
	if err := a.expandDiscoveryLocked(); err != nil {
		t.Fatal(err)
	}
	if a.state.Groups[0].Count != 40 || len(a.state.Watches) != 80 {
		t.Fatalf("reducing the gap removed discovered coverage: count=%d watches=%d", a.state.Groups[0].Count, len(a.state.Watches))
	}
	a.state.DiscoveryGap = defaultDiscoveryGap

	response = httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "test wallet_1") || !strings.Contains(body, "cold storage") || !strings.Contains(body, "Long-term savings; verify annually.") || !strings.Contains(body, "theme-bitcoin-night") || !strings.Contains(body, "monitoring with notifications") || !strings.Contains(body, "njump.me/npub1qqqqqqz7") || !strings.Contains(body, "Sort by") || !strings.Contains(body, ">Edit<") || !strings.Contains(body, "[hidden]{display:none!important}") || !strings.Contains(body, "focus-watches") || !strings.Contains(body, "block:'center'") || !strings.Contains(body, "toLocaleLowerCase()") || !strings.Contains(body, "72% 78%") || !strings.Contains(body, "classList.add('metadata-tag')") || !strings.Contains(body, "smart gap 20") || !strings.Contains(body, "notify_minimum") || !strings.Contains(body, "3 confirmations") || !strings.Contains(body, "new URLSearchParams(new FormData(addForm))") {
		t.Fatalf("render status %d: %s", response.Code, response.Body.String())
	}
}

func TestAddWatchAcceptsBrowserMultipartForSingleAndBulk(t *testing.T) {
	tests := []struct {
		name        string
		form        url.Values
		wantBulk    bool
		wantWatches int
	}{
		{
			name: "single address",
			form: url.Values{
				"label":    {"donations"},
				"category": {"public"},
				"source":   {"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"},
			},
			wantWatches: 1,
		},
		{
			name: "bulk addresses",
			form: url.Values{
				"label":        {"archive"},
				"category":     {"old"},
				"bulk_sources": {"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy\nbc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"},
			},
			wantBulk:    true,
			wantWatches: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			a.addWatch(response, multipartWatchRequest(t, test.form))
			if response.Code != http.StatusCreated {
				t.Fatalf("multipart add status %d: %s", response.Code, response.Body.String())
			}
			if len(a.state.Groups) != 1 || a.state.Groups[0].Bulk != test.wantBulk || len(a.state.Watches) != test.wantWatches {
				t.Fatalf("unexpected multipart state: groups=%+v watches=%d", a.state.Groups, len(a.state.Watches))
			}
		})
	}
}

func TestAddWatchAcceptsProxySafeURLEncodingThroughSecurityMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		form        url.Values
		wantWatches int
	}{
		{
			name: "single address",
			form: url.Values{
				"label":    {"donations"},
				"category": {"public"},
				"source":   {"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"},
			},
			wantWatches: 1,
		},
		{
			name: "bulk addresses",
			form: url.Values{
				"label":        {"archive"},
				"category":     {"old"},
				"bulk_sources": {"3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy\nbc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"},
			},
			wantWatches: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodPost, "/watches", strings.NewReader(test.form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
			request.Header.Set("Accept", "application/json")
			request.Header.Set("Sec-Fetch-Site", "same-origin")
			response := httptest.NewRecorder()
			securityHeaders(http.HandlerFunc(a.addWatch)).ServeHTTP(response, request)
			if response.Code != http.StatusCreated {
				t.Fatalf("URL-encoded add status %d: %s", response.Code, response.Body.String())
			}
			if len(a.state.Groups) != 1 || len(a.state.Watches) != test.wantWatches {
				t.Fatalf("unexpected URL-encoded state: groups=%+v watches=%d", a.state.Groups, len(a.state.Watches))
			}
		})
	}
}

func TestDuplicateWatchReturnsJSONConflict(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{
		"label":    {"donationaddress"},
		"category": {"donations"},
		"source":   {"1BoatSLRHtKNngkdXEeobR76b53LETtpyT"},
	}
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/watches", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("Accept", "application/json")
		response := httptest.NewRecorder()
		a.addWatch(response, request)
		if attempt == 0 && (response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"id"`)) {
			t.Fatalf("first add status %d: %s", response.Code, response.Body.String())
		}
		if attempt == 1 {
			if response.Code != http.StatusConflict {
				t.Fatalf("duplicate status %d: %s", response.Code, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), "already being watched") || !strings.Contains(response.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("unexpected duplicate response: %s", response.Body.String())
			}
		}
	}
}

func TestBulkAddressImportCreatesOneDeduplicatedGroup(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	legacy := "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"
	segwit := "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
	scriptHash := "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy"
	form := url.Values{
		"label":        {"Archive Addresses"},
		"category":     {"Donations"},
		"notes":        {"Imported public addresses."},
		"bulk_sources": {legacy + "\n" + segwit + ", " + legacy + ";" + scriptHash},
	}
	request := httptest.NewRequest(http.MethodPost, "/watches", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	a.addWatch(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("bulk add status %d: %s", response.Code, response.Body.String())
	}
	if len(a.state.Groups) != 1 || !a.state.Groups[0].Bulk || a.state.Groups[0].Count != 3 || a.state.Groups[0].ScriptType != "bulk" || len(a.state.Watches) != 3 {
		t.Fatalf("unexpected bulk state: groups=%+v watches=%d", a.state.Groups, len(a.state.Watches))
	}
	response = httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, expected := range []string{"archive addresses", "3 addresses pasted in bulk", "bulk address list", "Mixed mainnet address types", "multiple types", "Paste in bulk"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("bulk render is missing %q: %s", expected, body)
		}
	}

	second := url.Values{"label": {"duplicate list"}, "category": {"archive"}, "bulk_sources": {legacy + "\n" + segwit}}
	request = httptest.NewRequest(http.MethodPost, "/watches", strings.NewReader(second.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response = httptest.NewRecorder()
	a.addWatch(response, request)
	if response.Code != http.StatusConflict || len(a.state.Groups) != 1 || len(a.state.Watches) != 3 || !strings.Contains(response.Body.String(), "Nothing was added") {
		t.Fatalf("overlapping bulk import was not atomic: status=%d body=%s groups=%d watches=%d", response.Code, response.Body.String(), len(a.state.Groups), len(a.state.Watches))
	}
}

func TestBulkAddressImportRejectsInvalidEntryWithoutEchoingIt(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	invalid := "not-a-public-address"
	form := url.Values{"label": {"archive"}, "category": {"old"}, "bulk_sources": {"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa\n" + invalid}}
	request := httptest.NewRequest(http.MethodPost, "/watches", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	a.addWatch(response, request)
	if response.Code != http.StatusBadRequest || len(a.state.Groups) != 0 || strings.Contains(response.Body.String(), invalid) || !strings.Contains(response.Body.String(), "Entry 2") {
		t.Fatalf("invalid bulk response: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCombineGroupsMergesHistoryAndPreservesNotes(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	a.state.Groups = []WatchGroup{
		{ID: "one", Label: "first", Category: "archive", Notes: "first context", Source: "address-one", ScriptType: "address", Count: 1, NotifyMode: "incoming", NotifyMinimum: 100, NotifyAfter: 1},
		{ID: "two", Label: "second", Category: "archive", Notes: "second context", Source: "address-two", ScriptType: "address", Count: 1, NotifyMode: "incoming", NotifyMinimum: 100, NotifyAfter: 1},
	}
	a.state.Watches = []Watch{
		{ID: "watch-one", GroupID: "one", Label: "first", Address: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa"},
		{ID: "watch-two", GroupID: "two", Label: "second", Address: "3J98t1WpEZ73CNmQviecrnyiWrnqRhWNLy"},
	}
	a.state.Events = []Event{
		{WatchID: "watch-one", GroupID: "one", TxID: "shared", Direction: "received", Received: 10, Net: 10, OPReturn: []string{"hello"}, ReceivedAddresses: []string{"address-one"}, Historical: true, DetailsVersion: 1},
		{WatchID: "watch-two", GroupID: "two", TxID: "shared", Direction: "sent", Sent: 3, Net: -3, Runestone: true, ReceivedAddresses: []string{"address-two"}, SpentAddresses: []string{"address-three"}, DestinationAddresses: []string{"address-four"}, DetailsVersion: 1},
	}
	form := url.Values{"group_ids": {"one", "two"}, "label": {"Combined Archive"}, "category": {"Long Term"}}
	request := httptest.NewRequest(http.MethodPost, "/groups/combine", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	a.combineGroups(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("combine status %d: %s", response.Code, response.Body.String())
	}
	if len(a.state.Groups) != 1 || !a.state.Groups[0].Combined || a.state.Groups[0].Count != 2 || a.state.Groups[0].Label != "combined archive" || a.state.Groups[0].Category != "long term" || a.state.Groups[0].NotifyMode != "incoming" || a.state.Groups[0].NotifyMinimum != 100 || a.state.Groups[0].NotifyAfter != 1 {
		t.Fatalf("unexpected combined group: %+v", a.state.Groups)
	}
	if !strings.Contains(a.state.Groups[0].Notes, "first context") || !strings.Contains(a.state.Groups[0].Notes, "second context") {
		t.Fatalf("existing notes were not retained: %q", a.state.Groups[0].Notes)
	}
	for _, watch := range a.state.Watches {
		if watch.GroupID != a.state.Groups[0].ID || watch.Label != "combined archive" {
			t.Fatalf("watch was not reassigned: %+v", watch)
		}
	}
	if len(a.state.Events) != 1 {
		t.Fatalf("shared transaction was not consolidated: %+v", a.state.Events)
	}
	event := a.state.Events[0]
	if event.GroupID != a.state.Groups[0].ID || event.WatchID != "watch-one" || event.Direction != "activity" || event.Received != 0 || event.Sent != 0 || event.Net != 0 || event.Runestone || !event.TelegramSent || !event.NostrSent || event.Historical || event.DetailsVersion != 0 || !event.DetailsPending || len(event.ReceivedAddresses) != 0 || len(event.SpentAddresses) != 0 || len(event.DestinationAddresses) != 0 {
		t.Fatalf("unexpected consolidated event: %+v", event)
	}
}

func TestCombineGroupsDeduplicatesOverlappingAddress(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	address := "bc1qxy2kgdygjrsqtzq2n0yrf2493p83kkfjhx0wlh"
	scriptHash, err := bitcoin.ScriptHash(address)
	if err != nil {
		t.Fatal(err)
	}
	oldCheck := time.Now().Add(-time.Hour)
	newCheck := time.Now()
	a.state.Groups = []WatchGroup{
		{ID: "coinkite", Label: "coinkite products", Category: "hardware", Notes: "first note", Source: address, ScriptType: "address", Count: 1},
		{ID: "seedsigner", Label: "seedsigner", Category: "hardware", Notes: "second note", Source: address, ScriptType: "address", Count: 1},
	}
	a.state.Watches = []Watch{
		{ID: "watch-one", GroupID: "coinkite", Address: address, ScriptHash: scriptHash, Confirmed: 1_000, KnownTx: []string{"tx-one"}, Initialized: true, LastChecked: oldCheck},
		{ID: "watch-two", GroupID: "seedsigner", Address: address, ScriptHash: scriptHash, Confirmed: 2_000, KnownTx: []string{"tx-one", "tx-two"}, Initialized: true, LastChecked: newCheck},
	}
	a.state.Events = []Event{
		{WatchID: "watch-one", GroupID: "coinkite", TxID: "tx-one", Direction: "received", Received: 1_000, Net: 1_000, DetailsVersion: 1},
		{WatchID: "watch-two", GroupID: "seedsigner", TxID: "tx-one", Direction: "received", Received: 1_000, Net: 1_000, DetailsVersion: 1},
	}
	form := url.Values{"group_ids": {"coinkite", "seedsigner"}, "label": {"hardware wallets"}, "category": {"devices"}}
	request := httptest.NewRequest(http.MethodPost, "/groups/combine", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	a.combineGroups(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("combine status %d: %s", response.Code, response.Body.String())
	}
	if len(a.state.Groups) != 1 || a.state.Groups[0].Count != 1 || !strings.Contains(a.state.Groups[0].Notes, "first note") || !strings.Contains(a.state.Groups[0].Notes, "second note") {
		t.Fatalf("overlapping groups were not preserved correctly: %+v", a.state.Groups)
	}
	if len(a.state.Watches) != 1 {
		t.Fatalf("duplicate address remained after combine: %+v", a.state.Watches)
	}
	watch := a.state.Watches[0]
	if watch.ID != "watch-one" || watch.Confirmed != 2_000 || len(watch.KnownTx) != 2 || watch.GroupID != a.state.Groups[0].ID {
		t.Fatalf("canonical watch state was not merged: %+v", watch)
	}
	if len(a.state.Events) != 1 || a.state.Events[0].Received != 0 || !a.state.Events[0].DetailsPending || a.state.Events[0].WatchID != "watch-one" {
		t.Fatalf("duplicate event was not queued for exact recalculation: %+v", a.state.Events)
	}
}

func TestCombineGroupsRequiresConfirmationForMultiAddressWatch(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	a.state.Groups = []WatchGroup{
		{ID: "wallet", Label: "wallet", Category: "cold", Source: "xpub", ScriptType: "native-segwit", Count: 2},
		{ID: "address", Label: "address", Category: "cold", Source: "address", ScriptType: "address", Count: 1, NotifyMode: "off"},
	}
	a.state.Watches = []Watch{{ID: "one", GroupID: "wallet"}, {ID: "two", GroupID: "wallet"}, {ID: "three", GroupID: "address"}}
	form := url.Values{"group_ids": {"wallet", "address"}, "label": {"combined"}, "category": {"cold"}}
	request := httptest.NewRequest(http.MethodPost, "/groups/combine", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response := httptest.NewRecorder()
	a.combineGroups(response, request)
	if response.Code != http.StatusConflict || len(a.state.Groups) != 2 || !strings.Contains(response.Body.String(), "Confirm") {
		t.Fatalf("multi-address confirmation was not enforced: status=%d body=%s groups=%+v", response.Code, response.Body.String(), a.state.Groups)
	}
	form.Set("confirm_multi", "on")
	request = httptest.NewRequest(http.MethodPost, "/groups/combine", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response = httptest.NewRecorder()
	a.combineGroups(response, request)
	if response.Code != http.StatusCreated || len(a.state.Groups) != 1 || !a.state.Groups[0].Combined || a.state.Groups[0].NotifyMode != "off" {
		t.Fatalf("confirmed consolidation failed or differing rules were not disabled: status=%d group=%+v", response.Code, a.state.Groups)
	}
}

func TestIndexIncludesManualConsolidationWarnings(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	a.state.Groups = []WatchGroup{
		{ID: "wallet", Label: "wallet", Category: "cold", Source: "xpub", ScriptType: "native-segwit", Count: 2},
		{ID: "address", Label: "address", Category: "donations", Source: "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", ScriptType: "address", Count: 1},
	}
	a.state.Watches = []Watch{{ID: "one", GroupID: "wallet"}, {ID: "two", GroupID: "wallet"}, {ID: "three", GroupID: "address"}}
	response := httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, expected := range []string{"textContent='Combine'", "Combine selected", "cancelSelection.textContent='Cancel'", "cancelSelection.addEventListener('click',exitCombineSelection)", "item.checkbox.checked=false", "choice.hidden=true", "Select at least two watches", "Include group “", "Smart discovery for this group will stop.", "Existing history will not be resent", "confirm_multi"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("consolidation UI is missing %q: %s", expected, body)
		}
	}
}

func TestLocalMempoolTransactionLinks(t *testing.T) {
	t.Setenv("SWATCHER_MEMPOOL_URLS", `["http://mempool.local","http://privateexample.onion/","javascript:alert(1)","https://user@example.com"]`)
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	a.state.Groups = []WatchGroup{{ID: "wallet", Label: "wallet", Category: "cold", Source: "address", ScriptType: "address", Count: 1}}
	a.state.Watches = []Watch{{ID: "one", GroupID: "wallet", LastTxID: "abc123"}}
	a.state.Events = []Event{{WatchID: "one", GroupID: "wallet", TxID: "abc123", Direction: "received", Received: 42, SeenAt: time.Now()}}

	response := httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, expected := range []string{`data-url="http://mempool.local"`, `data-url="http://privateexample.onion"`, `class="tx-link" data-txid="abc123"`, `'/tx/'+encodeURIComponent`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("local Mempool link UI is missing %q: %s", expected, body)
		}
	}
	for _, rejected := range []string{"javascript:alert", "user@example.com"} {
		if strings.Contains(body, rejected) {
			t.Fatalf("unsafe Mempool URL was rendered: %q", rejected)
		}
	}

	a.state.PrivacyMode = true
	response = httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(response.Body.String(), `data-txid="abc123"`) {
		t.Fatal("privacy mode must not expose a transaction ID through a link")
	}
}

func TestNormalizeWatchMetadata(t *testing.T) {
	if got := normalizeWatchMetadata("  PAYCheck__ Wallet\tONE  "); got != "paycheck__ wallet one" {
		t.Fatalf("unexpected normalized metadata %q", got)
	}
	if !watchMetadataPattern.MatchString("paycheck__ wallet one") {
		t.Fatal("normalized name with spaces and underscores should be valid")
	}
	if watchMetadataPattern.MatchString("paycheck-wallet") {
		t.Fatal("unsupported punctuation should remain invalid")
	}
}

func TestValidateWatchNoteRejectsSecrets(t *testing.T) {
	valid, err := validateWatchNote("Donation address used for the 2025 conference.\nRetire after the event.")
	if err != nil || !strings.Contains(valid, "Retire") {
		t.Fatalf("valid note rejected: %q, %v", valid, err)
	}
	for _, secret := range []string{
		"xprv9s21ZrQH143K3secretmaterial",
		"K" + strings.Repeat("1", 51),
		"abandon ability able about above absent absorb abstract absurd abuse access accident",
	} {
		if _, err := validateWatchNote(secret); err == nil {
			t.Fatalf("secret-like note was accepted: %q", secret)
		}
	}
}

func TestUpdateGroupPreservesDisabledPrivateNote(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	a.state.Groups = []WatchGroup{{ID: "group", Label: "old", Category: "cold", Notes: "keep this context"}}
	form := url.Values{
		"label":          {"renamed"},
		"category":       {"savings"},
		"notify_mode":    {"all"},
		"notify_minimum": {"0"},
		"notify_after":   {"0"},
	}
	request := httptest.NewRequest(http.MethodPost, "/groups/group/update", strings.NewReader(form.Encode()))
	request.SetPathValue("id", "group")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	a.updateGroup(response, request)
	if response.Code != http.StatusSeeOther || a.state.Groups[0].Notes != "keep this context" {
		t.Fatalf("note was not preserved: status=%d group=%+v", response.Code, a.state.Groups[0])
	}
}

func TestUpdateBareXpubAddressTypeRederivesFreshBaseline(t *testing.T) {
	master, err := hdkeychain.NewMaster([]byte("s-watcher edit type test"), &chaincfg.MainNetParams)
	if err != nil {
		t.Fatal(err)
	}
	public, err := master.Neuter()
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	add := url.Values{"label": {"wallet"}, "category": {"savings"}, "source": {public.String()}, "script_type": {"native-segwit"}, "include_change": {"on"}}
	request := httptest.NewRequest(http.MethodPost, "/watches", strings.NewReader(add.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	a.addWatch(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("add status %d: %s", response.Code, response.Body.String())
	}
	oldID := a.state.Watches[0].ID
	a.state.Watches[0].Initialized = true
	a.state.Watches[0].KnownTx = []string{"historical"}
	a.state.Events = []Event{{WatchID: oldID, GroupID: a.state.Groups[0].ID, TxID: "historical"}}

	edit := url.Values{"label": {"wallet"}, "category": {"savings"}, "script_type": {"legacy"}, "notify_mode": {"all"}, "notify_minimum": {"0"}, "notify_after": {"0"}}
	request = httptest.NewRequest(http.MethodPost, "/groups/ignored/update", strings.NewReader(edit.Encode()))
	request.SetPathValue("id", a.state.Groups[0].ID)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response = httptest.NewRecorder()
	a.updateGroup(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("update status %d: %s", response.Code, response.Body.String())
	}
	if a.state.Groups[0].ScriptType != "legacy" || len(a.state.Watches) != defaultDiscoveryGap*2 || len(a.state.Events) != 0 {
		t.Fatalf("unexpected rederived state: group=%+v watches=%d events=%d", a.state.Groups[0], len(a.state.Watches), len(a.state.Events))
	}
	for _, watch := range a.state.Watches {
		if !strings.HasPrefix(watch.Address, "1") || watch.Initialized || len(watch.KnownTx) != 0 || watch.ID == oldID {
			t.Fatalf("watch was not freshly rederived as legacy: %+v", watch)
		}
	}
}

func TestLatestHistoricalTransactionRendersWithAddressType(t *testing.T) {
	a, err := New(t.TempDir(), "127.0.0.1:1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	txID := strings.Repeat("a", 64)
	a.state.Groups = []WatchGroup{{ID: "group", Label: "donation", Category: "public", Source: "1BoatSLRHtKNngkdXEeobR76b53LETtpyT", ScriptType: "address", Count: 1}}
	a.state.Watches = []Watch{{ID: "watch", GroupID: "group", Label: "donation", Address: "1BoatSLRHtKNngkdXEeobR76b53LETtpyT", Initialized: true, LastTxID: txID, LastTxHeight: 800000, LastTxAt: time.Now().Add(-14 * 24 * time.Hour)}}
	response := httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, expected := range []string{"Legacy mainnet", "P2PKH", "bip-0044.mediawiki", "Last transaction · 2 weeks ago", txID} {
		if !strings.Contains(body, expected) {
			t.Fatalf("render is missing %q: %s", expected, body)
		}
	}
}

func TestLatestHistoryPrefersMempoolThenHighestBlock(t *testing.T) {
	history := []electrum.HistoryItem{{TxHash: "old", Height: 10}, {TxHash: "new", Height: 12}}
	if latest, ok := latestHistory(history); !ok || latest.TxHash != "new" {
		t.Fatalf("unexpected confirmed latest: %+v, %v", latest, ok)
	}
	history = append(history, electrum.HistoryItem{TxHash: "pending", Height: 0})
	if latest, ok := latestHistory(history); !ok || latest.TxHash != "pending" {
		t.Fatalf("unexpected mempool latest: %+v, %v", latest, ok)
	}
}

func TestSetThemePersistsValidatedTheme(t *testing.T) {
	dataDir := t.TempDir()
	if err := SetTheme(dataDir, "not-a-theme"); err == nil {
		t.Fatal("invalid theme was accepted")
	}
	if err := SetTheme(dataDir, "paper"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dataDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved state
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Theme != "paper" || selectedTheme(saved.Theme) != "paper" || selectedTheme("unknown") != defaultTheme {
		t.Fatalf("unexpected saved theme: %+v", saved)
	}
}
