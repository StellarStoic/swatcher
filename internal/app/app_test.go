package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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
	if !strings.Contains(body, "signal-received") {
		t.Fatalf("latest received activity did not render a green signal: %s", body)
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
		"label":          {"testwallet"},
		"source":         {public.String()},
		"script_type":    {"native-segwit"},
		"count":          {"2"},
		"include_change": {"on"},
	}
	request := httptest.NewRequest(http.MethodPost, "/watches", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	a.addWatch(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("add status %d: %s", response.Code, response.Body.String())
	}
	if len(a.state.Groups) != 1 || len(a.state.Watches) != 4 {
		t.Fatalf("unexpected imported state: %d groups, %d watches", len(a.state.Groups), len(a.state.Watches))
	}

	response = httptest.NewRecorder()
	a.index(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "testwallet") || !strings.Contains(body, "Sort by") || !strings.Contains(body, ">Edit<") || !strings.Contains(body, "[hidden]{display:none!important}") || !strings.Contains(body, "focus-watches") {
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
		if attempt == 0 && response.Code != http.StatusSeeOther {
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
