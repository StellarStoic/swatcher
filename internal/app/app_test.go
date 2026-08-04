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
		Groups:      []WatchGroup{{ID: "group", Label: "Savings", Category: "Cold", Source: address, ScriptType: "address", Count: 1}},
		Watches:     []Watch{{ID: "watch", GroupID: "group", Label: "Savings", Address: address, Confirmed: 123456, Initialized: true}},
		Events:      []Event{{WatchID: "watch", GroupID: "group", TxID: strings.Repeat("a", 64), Direction: "received", Received: 5000, SeenAt: time.Now()}},
	}
	response := httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, secret := range []string{address, "123456 sat", "5000 sat", strings.Repeat("a", 64)} {
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
	a.state.Groups = []WatchGroup{{ID: "group", Label: payload, Category: payload, Source: payload, ScriptType: "address"}}
	a.state.Events = []Event{{GroupID: "group", TxID: strings.Repeat("a", 64), Direction: "received", OPReturn: []string{payload}, SeenAt: time.Now()}}
	response := httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	if strings.Contains(body, payload) || !strings.Contains(body, "&lt;script&gt;") || !strings.Contains(body, "op-return") || !strings.Contains(body, "OP_RETURN") {
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
	if len(a.state.Groups) != 1 || a.state.Groups[0].Count != defaultDiscoveryGap || len(a.state.Watches) != defaultDiscoveryGap*2 {
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
	if response.Code != http.StatusOK || !strings.Contains(body, "test wallet_1") || !strings.Contains(body, "cold storage") || !strings.Contains(body, "monitoring with notifications") || !strings.Contains(body, "njump.me/npub1qqqqqqz7") || !strings.Contains(body, "Sort by") || !strings.Contains(body, ">Edit<") || !strings.Contains(body, "[hidden]{display:none!important}") || !strings.Contains(body, "focus-watches") || !strings.Contains(body, "block:'center'") || !strings.Contains(body, "toLocaleLowerCase()") || !strings.Contains(body, "72% 78%") || !strings.Contains(body, "classList.add('metadata-tag')") || !strings.Contains(body, "smart gap 20") {
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
