// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"encoding/json"
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
	"github.com/s-watcher/s-watcher/internal/electrum"
	"github.com/s-watcher/s-watcher/internal/notify"
	"github.com/s-watcher/s-watcher/internal/webauth"
)

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
	message := digestMessage([]Event{{GroupID: "g", Direction: "received", Received: 42, TxID: "txid"}}, map[string]string{"g": "donations / website"})
	if !strings.Contains(message, "daily digest: 1 activities") || !strings.Contains(message, "donations / website") || !strings.Contains(message, "42 sat") {
		t.Fatalf("unexpected digest: %s", message)
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
	if response.Code != http.StatusOK || !strings.Contains(body, "test wallet_1") || !strings.Contains(body, "cold storage") || !strings.Contains(body, "Long-term savings; verify annually.") || !strings.Contains(body, "theme-bitcoin-night") || !strings.Contains(body, "monitoring with notifications") || !strings.Contains(body, "njump.me/npub1qqqqqqz7") || !strings.Contains(body, "Sort by") || !strings.Contains(body, ">Edit<") || !strings.Contains(body, "[hidden]{display:none!important}") || !strings.Contains(body, "focus-watches") || !strings.Contains(body, "block:'center'") || !strings.Contains(body, "toLocaleLowerCase()") || !strings.Contains(body, "72% 78%") || !strings.Contains(body, "classList.add('metadata-tag')") || !strings.Contains(body, "smart gap 20") || !strings.Contains(body, "notify_minimum") || !strings.Contains(body, "3 confirmations") {
		t.Fatalf("render status %d: %s", response.Code, response.Body.String())
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
		{WatchID: "watch-one", GroupID: "one", TxID: "shared", Direction: "received", Received: 10, Net: 10, OPReturn: []string{"hello"}, ReceivedAddresses: []string{"address-one"}},
		{WatchID: "watch-two", GroupID: "two", TxID: "shared", Direction: "sent", Sent: 3, Net: -3, Runestone: true, ReceivedAddresses: []string{"address-two"}},
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
	if event.GroupID != a.state.Groups[0].ID || event.Direction != "self-transfer" || event.Received != 10 || event.Sent != 3 || event.Net != 7 || !event.Runestone || !event.TelegramSent || !event.NostrSent || len(event.ReceivedAddresses) != 2 {
		t.Fatalf("unexpected consolidated event: %+v", event)
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
	for _, expected := range []string{"Combine selected", "Select at least two watches", "Include group “", "Smart discovery for this group will stop.", "Existing history will not be resent", "confirm_multi"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("consolidation UI is missing %q: %s", expected, body)
		}
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
