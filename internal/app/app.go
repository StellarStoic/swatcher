// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/s-watcher/s-watcher/internal/bitcoin"
	"github.com/s-watcher/s-watcher/internal/electrum"
	"github.com/s-watcher/s-watcher/internal/notify"
	"github.com/s-watcher/s-watcher/internal/webauth"
)

type Watch struct {
	ID           string    `json:"id"`
	GroupID      string    `json:"groupId,omitempty"`
	Label        string    `json:"label"`
	Address      string    `json:"address"`
	Path         string    `json:"path,omitempty"`
	ScriptHash   string    `json:"scriptHash"`
	Confirmed    int64     `json:"confirmed"`
	Unconfirmed  int64     `json:"unconfirmed"`
	KnownTx      []string  `json:"knownTx"`
	Initialized  bool      `json:"initialized"`
	LastChecked  time.Time `json:"lastChecked,omitempty"`
	LastError    string    `json:"lastError,omitempty"`
	LastTxID     string    `json:"lastTxId,omitempty"`
	LastTxHeight int64     `json:"lastTxHeight,omitempty"`
	LastTxAt     time.Time `json:"lastTxAt,omitempty"`
}

type WatchGroup struct {
	ID             string    `json:"id"`
	Label          string    `json:"label"`
	Category       string    `json:"category,omitempty"`
	Notes          string    `json:"notes,omitempty"`
	Source         string    `json:"source"`
	ScriptType     string    `json:"scriptType,omitempty"`
	Count          int       `json:"count"`
	IncludeChange  bool      `json:"includeChange,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	DiscoveryError string    `json:"discoveryError,omitempty"`
	Bulk           bool      `json:"bulk,omitempty"`
	Combined       bool      `json:"combined,omitempty"`
	NotifyMode     string    `json:"notifyMode,omitempty"`
	NotifyMinimum  uint64    `json:"notifyMinimum,omitempty"`
	NotifyAfter    int       `json:"notifyAfter,omitempty"`
}

type Event struct {
	WatchID           string    `json:"watchId"`
	GroupID           string    `json:"groupId,omitempty"`
	TxID              string    `json:"txid"`
	Height            int64     `json:"height"`
	Direction         string    `json:"direction"`
	Received          uint64    `json:"received"`
	Sent              uint64    `json:"sent"`
	Net               int64     `json:"net"`
	OPReturn          []string  `json:"opReturn,omitempty"`
	Replaceable       bool      `json:"replaceable,omitempty"`
	Runestone         bool      `json:"runestone,omitempty"`
	Inscriptions      int       `json:"inscriptions,omitempty"`
	ReceivedAddresses []string  `json:"receivedAddresses,omitempty"`
	SeenAt            time.Time `json:"seenAt"`
	TelegramSent      bool      `json:"telegramSent,omitempty"`
	NostrSent         bool      `json:"nostrSent,omitempty"`
}

type state struct {
	Watches                     []Watch      `json:"watches"`
	Events                      []Event      `json:"events"`
	Groups                      []WatchGroup `json:"groups,omitempty"`
	PrivacyMode                 bool         `json:"privacyMode,omitempty"`
	DiscoveryGap                int          `json:"discoveryGap,omitempty"`
	LastTelegramDigest          string       `json:"lastTelegramDigest,omitempty"`
	LastNostrDigest             string       `json:"lastNostrDigest,omitempty"`
	PrivacyIndicatorsConfigured bool         `json:"privacyIndicatorsConfigured,omitempty"`
	AddressReuseIndicators      bool         `json:"addressReuseIndicators,omitempty"`
	SmallDepositIndicators      bool         `json:"smallDepositIndicators,omitempty"`
	CombinedWalletIndicators    bool         `json:"combinedWalletIndicators,omitempty"`
	SmallDepositThreshold       uint64       `json:"smallDepositThreshold,omitempty"`
	Theme                       string       `json:"theme,omitempty"`
}

const defaultDiscoveryGap = 20
const maxBulkAddresses = 10000
const maxWatchFormBytes = 2 << 20
const defaultTheme = "bitcoin-night"

var themes = map[string]bool{
	"bitcoin-night": true,
	"cypherpunk":    true,
	"arctic":        true,
	"forest":        true,
	"paper":         true,
}

func selectedTheme(value string) string {
	if themes[value] {
		return value
	}
	return defaultTheme
}

func discoveryGap(value int) int {
	if value < 1 || value > 500 {
		return defaultDiscoveryGap
	}
	return value
}

func privacyIndicatorSettings(value state) (reuse, small, combined bool, threshold uint64) {
	if !value.PrivacyIndicatorsConfigured {
		return true, true, true, 1_000
	}
	threshold = value.SmallDepositThreshold
	if threshold == 0 {
		threshold = 1_000
	}
	return value.AddressReuseIndicators, value.SmallDepositIndicators, value.CombinedWalletIndicators, threshold
}

type groupView struct {
	WatchGroup
	Confirmed         int64
	Unconfirmed       int64
	Addresses         int
	Ready             int
	LastChecked       time.Time
	LastError         string
	DisplaySource     string
	DisplayBalance    string
	LastActivity      time.Time
	ActivitySignal    string
	ScriptTypeKey     string
	CanEditScriptType bool
	AddressTypeName   string
	AddressTypeCode   string
	AddressTypeURL    string
	LastTxID          string
	DisplayLastTxID   string
	LastTxAt          time.Time
	LastTxAgo         string
}

type eventView struct {
	Event
	DisplayAmount         string
	DisplayTxID           string
	ReuseCount            int
	SmallDeposit          bool
	CombinedGroups        int
	SmallDepositThreshold uint64
}

type pageData struct {
	Groups           []groupView
	Events           []eventView
	Nostr            *nostrIdentityView
	PrivacyMode      bool
	Theme            string
	SortMode         string
	GroupSuggestions []string
	CSPNonce         string
}

var watchMetadataPattern = regexp.MustCompile(`^[a-z0-9_]+(?: [a-z0-9_]+)*$`)
var watchWIFPattern = regexp.MustCompile(`(?i)(?:^|[^1-9A-HJ-NP-Za-km-z])(?:5[1-9A-HJ-NP-Za-km-z]{50}|[KL][1-9A-HJ-NP-Za-km-z]{51})(?:$|[^1-9A-HJ-NP-Za-km-z])`)
var seedWordPattern = regexp.MustCompile(`^[A-Za-z]+$`)

func normalizeWatchMetadata(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func validateWatchNote(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 500 {
		return "", errors.New("the note must contain at most 500 valid characters")
	}
	for _, character := range value {
		if unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return "", errors.New("the note contains an unsupported control character")
		}
	}
	lower := strings.ToLower(value)
	for _, prefix := range []string{"xprv", "yprv", "zprv", "tprv", "uprv", "vprv"} {
		if strings.Contains(lower, prefix) {
			return "", errors.New("private keys and seed phrases cannot be stored in wallet notes")
		}
	}
	if watchWIFPattern.MatchString(value) {
		return "", errors.New("private keys and seed phrases cannot be stored in wallet notes")
	}
	words := strings.Fields(value)
	if len(words) == 12 || len(words) == 15 || len(words) == 18 || len(words) == 21 || len(words) == 24 {
		allWords := true
		for _, word := range words {
			if !seedWordPattern.MatchString(word) {
				allWords = false
				break
			}
		}
		if allWords {
			return "", errors.New("private keys and seed phrases cannot be stored in wallet notes")
		}
	}
	return value, nil
}

type nostrIdentityView struct {
	Name   string
	Npub   string
	Avatar string
}

type App struct {
	mu               sync.RWMutex
	state            state
	path             string
	client           *electrum.Client
	interval         time.Duration
	tmpl             *template.Template
	notifier         notify.Sender
	authPath         string
	authMu           sync.Mutex
	failedLogins     int
	loginLockedUntil time.Time
}

func New(dataDir, electrumAddress string, interval time.Duration) (*App, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	a := &App{path: filepath.Join(dataDir, "state.json"), authPath: filepath.Join(dataDir, "auth.json"), client: &electrum.Client{Address: electrumAddress}, interval: interval, notifier: notify.Sender{Path: filepath.Join(dataDir, "notifications.json")}}
	a.tmpl = template.Must(template.New("index").Funcs(template.FuncMap{"watchLabel": a.watchLabel}).Parse(indexHTML))
	if err := a.load(); err != nil {
		return nil, err
	}
	if _, err := a.notifier.EnsureIdentity(); err != nil {
		log.Printf("notification identity: %v", err)
	}
	return a, nil
}

func (a *App) Run(listen string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", a.loginPage)
	mux.HandleFunc("POST /login", a.login)
	mux.HandleFunc("POST /logout", a.logout)
	mux.HandleFunc("GET /health", a.health)
	protected := http.NewServeMux()
	protected.HandleFunc("GET /", a.index)
	protected.HandleFunc("POST /watches", a.addWatch)
	protected.HandleFunc("POST /groups/{id}/update", a.updateGroup)
	protected.HandleFunc("POST /groups/{id}/delete", a.deleteGroup)
	protected.HandleFunc("POST /groups/combine", a.combineGroups)
	mux.Handle("/", a.requireAuth(protected))
	go a.pollLoop()
	server := &http.Server{Addr: listen, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}
	return server.ListenAndServe()
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	sortMode := r.URL.Query().Get("sort")
	if sortMode != "balance" && sortMode != "name" && sortMode != "group" && sortMode != "date" && sortMode != "activity" && sortMode != "type" {
		sortMode = "activity"
	}
	data := pageData{Groups: a.groupViewsLocked(), PrivacyMode: a.state.PrivacyMode, Theme: selectedTheme(a.state.Theme), SortMode: sortMode, CSPNonce: cspNonce(r)}
	suggestions := map[string]bool{}
	for _, group := range a.state.Groups {
		if watchMetadataPattern.MatchString(group.Category) {
			suggestions[group.Category] = true
		}
	}
	for suggestion := range suggestions {
		data.GroupSuggestions = append(data.GroupSuggestions, suggestion)
	}
	sort.Strings(data.GroupSuggestions)
	reuseEnabled, smallEnabled, combinedEnabled, smallThreshold := privacyIndicatorSettings(a.state)
	receiptCounts := map[string]int{}
	txGroups := map[string]map[string]bool{}
	for _, event := range a.state.Events {
		if txGroups[event.TxID] == nil {
			txGroups[event.TxID] = map[string]bool{}
		}
		txGroups[event.TxID][event.GroupID] = true
	}
	for _, event := range a.state.Events {
		view := eventView{Event: event, DisplayTxID: event.TxID}
		view.DisplayAmount = eventAmount(event)
		for _, address := range event.ReceivedAddresses {
			receiptCounts[address]++
			if receiptCounts[address] > view.ReuseCount {
				view.ReuseCount = receiptCounts[address]
			}
		}
		if !reuseEnabled || view.ReuseCount < 2 {
			view.ReuseCount = 0
		}
		view.SmallDeposit = smallEnabled && event.Received > 0 && event.Received < smallThreshold
		view.SmallDepositThreshold = smallThreshold
		if combinedEnabled {
			view.CombinedGroups = len(txGroups[event.TxID])
			if view.CombinedGroups < 2 {
				view.CombinedGroups = 0
			}
		}
		if a.state.PrivacyMode {
			view.DisplayTxID = maskIdentifier(event.TxID)
			view.DisplayAmount = legacyMask(8)
		}
		data.Events = append(data.Events, view)
		for i := range data.Groups {
			if data.Groups[i].ID == event.GroupID && event.SeenAt.After(data.Groups[i].LastActivity) {
				data.Groups[i].LastActivity = event.SeenAt
				data.Groups[i].ActivitySignal = movementSignal(event)
			}
		}
	}
	for i := range data.Groups {
		if data.Groups[i].ActivitySignal == "" {
			data.Groups[i].ActivitySignal = "idle"
		}
		data.Groups[i].DisplaySource = data.Groups[i].Source
		data.Groups[i].DisplayLastTxID = data.Groups[i].LastTxID
		data.Groups[i].LastTxAgo = relativeTime(data.Groups[i].LastTxAt, time.Now())
		data.Groups[i].DisplayBalance = fmt.Sprintf("%d sat", data.Groups[i].Confirmed)
		if data.Groups[i].Unconfirmed != 0 {
			data.Groups[i].DisplayBalance += fmt.Sprintf(" (%d pending)", data.Groups[i].Unconfirmed)
		}
		if a.state.PrivacyMode {
			if !data.Groups[i].Bulk && !data.Groups[i].Combined {
				data.Groups[i].DisplaySource = maskIdentifier(data.Groups[i].Source)
			}
			data.Groups[i].DisplayLastTxID = maskIdentifier(data.Groups[i].LastTxID)
			data.Groups[i].DisplayBalance = legacyMask(8)
		}
	}
	a.mu.RUnlock()
	sortGroups(data.Groups, sortMode)
	if c, err := a.notifier.Load(); err == nil && c.NostrEnabled && c.NostrSenderNpub != "" {
		npub := c.NostrSenderNpub
		if data.PrivacyMode {
			npub = maskIdentifier(npub)
		}
		data.Nostr = &nostrIdentityView{Name: c.NostrSenderName, Npub: npub, Avatar: c.NostrAvatar}
	}
	sort.Slice(data.Events, func(i, j int) bool { return data.Events[i].SeenAt.After(data.Events[j].SeenAt) })
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.Execute(w, data); err != nil {
		log.Printf("render: %v", err)
	}
}

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		config, err := webauth.Load(a.authPath)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		cookie, err := r.Cookie("swatcher_session")
		if err != nil || !webauth.ValidSession(config, cookie.Value, time.Now()) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) loginPage(w http.ResponseWriter, r *http.Request) {
	_, err := webauth.Load(a.authPath)
	a.mu.RLock()
	theme := selectedTheme(a.state.Theme)
	a.mu.RUnlock()
	data := struct {
		Configured bool
		Error      bool
		CSPNonce   string
		Theme      string
	}{Configured: err == nil, Error: r.URL.Query().Get("error") == "1", CSPNonce: cspNonce(r), Theme: theme}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if renderErr := template.Must(template.New("login").Parse(loginHTML)).Execute(w, data); renderErr != nil {
		log.Printf("render login: %v", renderErr)
	}
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	a.authMu.Lock()
	if time.Now().Before(a.loginLockedUntil) {
		a.authMu.Unlock()
		http.Error(w, "Too many failed attempts. Try again in one minute.", http.StatusTooManyRequests)
		return
	}
	a.authMu.Unlock()
	config, err := webauth.Load(a.authPath)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	password := r.FormValue("password")
	if len(password) > 1024 || !webauth.Verify(config, password) {
		a.authMu.Lock()
		a.failedLogins++
		if a.failedLogins >= 5 {
			a.loginLockedUntil = time.Now().Add(time.Minute)
			a.failedLogins = 0
		}
		a.authMu.Unlock()
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}
	a.authMu.Lock()
	a.failedLogins = 0
	a.loginLockedUntil = time.Time{}
	a.authMu.Unlock()
	expires := time.Now().Add(12 * time.Hour)
	token, err := webauth.NewSession(config, expires)
	if err != nil {
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "swatcher_session", Value: token, Path: "/", Expires: expires, MaxAge: 12 * 60 * 60, HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "swatcher_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *App) addWatch(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWatchFormBytes)
	if err := r.ParseForm(); err != nil {
		writeFormError(w, r, http.StatusBadRequest, "The watch form is too large or could not be read.")
		return
	}
	source := strings.TrimSpace(r.FormValue("source"))
	bulkSource := strings.TrimSpace(r.FormValue("bulk_sources"))
	label := normalizeWatchMetadata(r.FormValue("label"))
	category := normalizeWatchMetadata(r.FormValue("category"))
	notes, noteErr := validateWatchNote(r.FormValue("notes"))
	if noteErr != nil {
		writeFormError(w, r, http.StatusBadRequest, noteErr.Error()+". Never enter a private key or seed phrase.")
		return
	}
	if label == "" {
		label = "bitcoinaddress"
	}
	if len(label) > 80 || !watchMetadataPattern.MatchString(label) {
		writeFormError(w, r, http.StatusBadRequest, "The watch name may contain only letters, numbers, spaces, and underscores.")
		return
	}
	if category == "" {
		category = "uncategorized"
	}
	if len(category) > 60 || !watchMetadataPattern.MatchString(category) {
		writeFormError(w, r, http.StatusBadRequest, "The group name may contain only letters, numbers, spaces, and underscores.")
		return
	}
	groupID := randomID()
	newWatches := []Watch{}
	scriptType := strings.TrimSpace(r.FormValue("script_type"))
	count := 1
	bulk := bulkSource != ""
	includeChange := r.FormValue("include_change") == "on"
	if bulk {
		addresses, parseErr := parseBulkAddresses(bulkSource)
		if parseErr != nil {
			writeFormError(w, r, http.StatusBadRequest, parseErr.Error())
			return
		}
		count = len(addresses)
		source = "bulk-address-list"
		scriptType = "bulk"
		includeChange = false
		for _, address := range addresses {
			scriptHash, _ := bitcoin.ScriptHash(address)
			newWatches = append(newWatches, Watch{ID: randomID(), GroupID: groupID, Label: label, Address: address, ScriptHash: scriptHash})
		}
	} else if scriptHash, err := bitcoin.ScriptHash(source); err == nil {
		newWatches = append(newWatches, Watch{ID: randomID(), GroupID: groupID, Label: label, Address: source, ScriptHash: scriptHash})
		scriptType = "address"
	} else {
		a.mu.RLock()
		count = discoveryGap(a.state.DiscoveryGap)
		a.mu.RUnlock()
		derived, deriveErr := bitcoin.DeriveAddresses(source, scriptType, count, includeChange)
		if deriveErr != nil {
			writeFormError(w, r, http.StatusBadRequest, deriveErr.Error()+".")
			return
		}
		if len(derived) > 0 {
			scriptType = scriptTypeForAddress(derived[0].Address)
		}
		for _, child := range derived {
			scriptHash, hashErr := bitcoin.ScriptHash(child.Address)
			if hashErr != nil {
				writeFormError(w, r, http.StatusBadRequest, hashErr.Error()+".")
				return
			}
			newWatches = append(newWatches, Watch{ID: randomID(), GroupID: groupID, Label: label + " " + child.Path, Address: child.Address, Path: child.Path, ScriptHash: scriptHash})
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, group := range a.state.Groups {
		if !bulk && !group.Bulk && group.Source == source && group.ScriptType == scriptType {
			writeFormError(w, r, http.StatusConflict, fmt.Sprintf("This wallet or address is already being watched as %q in %q.", group.Label, group.Category))
			return
		}
	}
	overlaps := map[string]int{}
	owners := make(map[string]string, len(a.state.Watches))
	for _, existing := range a.state.Watches {
		existingGroupID := existing.GroupID
		if existingGroupID == "" {
			existingGroupID = existing.ID
		}
		owners[existing.Address] = existingGroupID
	}
	for _, candidate := range newWatches {
		if existingGroupID := owners[candidate.Address]; existingGroupID != "" {
			overlaps[existingGroupID]++
		}
	}
	if len(overlaps) > 0 {
		groupID, overlapCount := "", 0
		for id, matches := range overlaps {
			if matches > overlapCount {
				groupID, overlapCount = id, matches
			}
		}
		name := "an existing watch"
		for _, group := range a.state.Groups {
			if group.ID == groupID {
				name = group.Label
				if group.Category != "" {
					name = group.Category + " / " + group.Label
				}
				break
			}
		}
		writeFormError(w, r, http.StatusConflict, fmt.Sprintf("This watch overlaps with %q: %d of %d addresses are already being watched. Nothing was added.", name, overlapCount, len(newWatches)))
		return
	}
	a.state.Watches = append(a.state.Watches, newWatches...)
	a.state.Groups = append(a.state.Groups, WatchGroup{ID: groupID, Label: label, Category: category, Notes: notes, Source: source, ScriptType: scriptType, Count: count, IncludeChange: includeChange, CreatedAt: time.Now().UTC(), Bulk: bulk})
	if err := a.saveLocked(); err != nil {
		http.Error(w, "could not save watch", http.StatusInternalServerError)
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": groupID})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func parseBulkAddresses(value string) ([]string, error) {
	parts := strings.FieldsFunc(value, func(character rune) bool {
		return unicode.IsSpace(character) || character == ',' || character == ';'
	})
	if len(parts) < 2 {
		return nil, errors.New("Bulk import requires at least two Bitcoin mainnet addresses")
	}
	addresses := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for index, address := range parts {
		if strings.HasPrefix(strings.ToLower(address), "bc1") {
			address = strings.ToLower(address)
		}
		if _, err := bitcoin.ScriptHash(address); err != nil {
			return nil, fmt.Errorf("Entry %d is not a valid Bitcoin mainnet address; nothing was added", index+1)
		}
		if !seen[address] {
			seen[address] = true
			addresses = append(addresses, address)
			if len(addresses) > maxBulkAddresses {
				return nil, fmt.Errorf("Bulk import supports at most %d unique addresses at once", maxBulkAddresses)
			}
		}
	}
	if len(addresses) < 2 {
		return nil, errors.New("Bulk import requires at least two unique Bitcoin mainnet addresses")
	}
	return addresses, nil
}

func writeFormError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
		return
	}
	http.Error(w, message, status)
}

func scriptTypeForAddress(address string) string {
	switch {
	case strings.HasPrefix(address, "bc1p"):
		return "taproot"
	case strings.HasPrefix(address, "bc1q"):
		return "native-segwit"
	case strings.HasPrefix(address, "3"):
		return "nested-segwit"
	case strings.HasPrefix(address, "1"):
		return "legacy"
	default:
		return "address"
	}
}

func isBareXpub(source string) bool {
	source = strings.TrimSpace(source)
	return strings.HasPrefix(source, "xpub") && !strings.Contains(source, "(")
}

func validScriptType(value string) bool {
	return value == "legacy" || value == "nested-segwit" || value == "native-segwit" || value == "taproot"
}

func (a *App) updateGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	label := normalizeWatchMetadata(r.FormValue("label"))
	category := normalizeWatchMetadata(r.FormValue("category"))
	notes := ""
	noteValues, noteProvided := r.Form["notes"]
	if noteProvided {
		var noteErr error
		notes, noteErr = validateWatchNote(noteValues[0])
		if noteErr != nil {
			writeFormError(w, r, http.StatusBadRequest, noteErr.Error()+". Never enter a private key or seed phrase.")
			return
		}
	}
	if label == "" || len(label) > 80 || !watchMetadataPattern.MatchString(label) || category == "" || len(category) > 60 || !watchMetadataPattern.MatchString(category) {
		writeFormError(w, r, http.StatusBadRequest, "Name and group may contain only letters, numbers, spaces, and underscores.")
		return
	}
	notifyMode := strings.TrimSpace(r.FormValue("notify_mode"))
	if notifyMode == "" {
		notifyMode = "all"
	}
	if notifyMode != "all" && notifyMode != "incoming" && notifyMode != "outgoing" && notifyMode != "off" {
		writeFormError(w, r, http.StatusBadRequest, "Choose a valid notification activity rule.")
		return
	}
	notifyMinimum := uint64(0)
	if value := r.FormValue("notify_minimum"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			writeFormError(w, r, http.StatusBadRequest, "The notification minimum must be a non-negative sat amount.")
			return
		}
		notifyMinimum = parsed
	}
	notifyAfter := 0
	if value := r.FormValue("notify_after"); value != "" {
		parsed, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			writeFormError(w, r, http.StatusBadRequest, "Choose mempool, 1, 3, or 6 confirmations.")
			return
		}
		notifyAfter = parsed
	}
	if notifyAfter != 0 && notifyAfter != 1 && notifyAfter != 3 && notifyAfter != 6 {
		writeFormError(w, r, http.StatusBadRequest, "Choose mempool, 1, 3, or 6 confirmations.")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	groupIndex := -1
	for i := range a.state.Groups {
		if a.state.Groups[i].ID == id {
			groupIndex = i
			break
		}
	}
	if groupIndex < 0 {
		writeFormError(w, r, http.StatusNotFound, "The watch could not be found.")
		return
	}
	group := &a.state.Groups[groupIndex]
	requestedType := strings.TrimSpace(r.FormValue("script_type"))
	typeChanged := false
	var replacement []Watch
	if isBareXpub(group.Source) && requestedType != "" && requestedType != group.ScriptType {
		if !validScriptType(requestedType) {
			writeFormError(w, r, http.StatusBadRequest, "Choose a valid xpub address type.")
			return
		}
		count := group.Count
		if count < 1 {
			count = discoveryGap(a.state.DiscoveryGap)
		}
		derived, err := bitcoin.DeriveAddresses(group.Source, requestedType, count, group.IncludeChange)
		if err != nil {
			writeFormError(w, r, http.StatusBadRequest, err.Error()+".")
			return
		}
		owners := make(map[string]string, len(a.state.Watches))
		for _, watch := range a.state.Watches {
			if watch.GroupID != id {
				owners[watch.Address] = watch.GroupID
			}
		}
		for _, child := range derived {
			if owner, exists := owners[child.Address]; exists {
				ownerName := "another watched wallet"
				for _, candidate := range a.state.Groups {
					if candidate.ID == owner {
						ownerName = candidate.Category + " / " + candidate.Label
						break
					}
				}
				writeFormError(w, r, http.StatusConflict, fmt.Sprintf("The selected address type overlaps %q. Nothing was changed.", ownerName))
				return
			}
			scriptHash, err := bitcoin.ScriptHash(child.Address)
			if err != nil {
				writeFormError(w, r, http.StatusBadRequest, err.Error()+".")
				return
			}
			replacement = append(replacement, Watch{ID: randomID(), GroupID: id, Label: label + " " + child.Path, Address: child.Address, Path: child.Path, ScriptHash: scriptHash})
		}
		typeChanged = true
	}
	group.Label = label
	group.Category = category
	if noteProvided {
		group.Notes = notes
	}
	group.NotifyMode = notifyMode
	group.NotifyMinimum = notifyMinimum
	group.NotifyAfter = notifyAfter
	if typeChanged {
		group.ScriptType = requestedType
		if group.Count < 1 {
			group.Count = discoveryGap(a.state.DiscoveryGap)
		}
		group.DiscoveryError = ""
		keptWatches := a.state.Watches[:0]
		for _, watch := range a.state.Watches {
			if watch.GroupID != id {
				keptWatches = append(keptWatches, watch)
			}
		}
		a.state.Watches = append(keptWatches, replacement...)
		keptEvents := a.state.Events[:0]
		for _, event := range a.state.Events {
			if event.GroupID != id {
				keptEvents = append(keptEvents, event)
			}
		}
		a.state.Events = keptEvents
	}
	for i := range a.state.Watches {
		if a.state.Watches[i].GroupID == id {
			a.state.Watches[i].Label = label
			if a.state.Watches[i].Path != "" {
				a.state.Watches[i].Label += " " + a.state.Watches[i].Path
			}
		}
	}
	if err := a.saveLocked(); err != nil {
		writeFormError(w, r, http.StatusInternalServerError, "The watch settings could not be saved.")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func sortGroups(groups []groupView, mode string) {
	sort.SliceStable(groups, func(i, j int) bool {
		a, b := groups[i], groups[j]
		switch mode {
		case "balance":
			return a.Confirmed+a.Unconfirmed > b.Confirmed+b.Unconfirmed
		case "name":
			return a.Label < b.Label
		case "group":
			if a.Category == b.Category {
				return a.Label < b.Label
			}
			return a.Category < b.Category
		case "date":
			return a.CreatedAt.After(b.CreatedAt)
		case "type":
			if a.ScriptType == b.ScriptType {
				return a.Label < b.Label
			}
			return a.ScriptType < b.ScriptType
		default:
			if a.LastActivity.Equal(b.LastActivity) {
				return a.CreatedAt.After(b.CreatedAt)
			}
			return a.LastActivity.After(b.LastActivity)
		}
	})
}

func (a *App) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.mu.Lock()
	defer a.mu.Unlock()
	removedWatchIDs := map[string]bool{}
	keptWatches := a.state.Watches[:0]
	for _, watch := range a.state.Watches {
		watchGroupID := watch.GroupID
		if watchGroupID == "" {
			watchGroupID = watch.ID
		}
		if watchGroupID == id {
			removedWatchIDs[watch.ID] = true
		} else {
			keptWatches = append(keptWatches, watch)
		}
	}
	a.state.Watches = keptWatches
	keptEvents := a.state.Events[:0]
	for _, event := range a.state.Events {
		if !removedWatchIDs[event.WatchID] {
			keptEvents = append(keptEvents, event)
		}
	}
	a.state.Events = keptEvents
	keptGroups := a.state.Groups[:0]
	for _, group := range a.state.Groups {
		if group.ID != id {
			keptGroups = append(keptGroups, group)
		}
	}
	a.state.Groups = keptGroups
	_ = a.saveLocked()
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) combineGroups(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		writeFormError(w, r, http.StatusBadRequest, "The consolidation request could not be read.")
		return
	}
	label := normalizeWatchMetadata(r.FormValue("label"))
	category := normalizeWatchMetadata(r.FormValue("category"))
	if label == "" || len(label) > 80 || !watchMetadataPattern.MatchString(label) || category == "" || len(category) > 60 || !watchMetadataPattern.MatchString(category) {
		writeFormError(w, r, http.StatusBadRequest, "Name and group may contain only letters, numbers, spaces, and underscores.")
		return
	}
	notes, err := validateWatchNote(r.FormValue("notes"))
	if err != nil {
		writeFormError(w, r, http.StatusBadRequest, err.Error()+". Never enter a private key or seed phrase.")
		return
	}
	selectedIDs := make([]string, 0, len(r.Form["group_ids"]))
	selected := make(map[string]bool, len(r.Form["group_ids"]))
	for _, id := range r.Form["group_ids"] {
		id = strings.TrimSpace(id)
		if id != "" && !selected[id] {
			selected[id] = true
			selectedIDs = append(selectedIDs, id)
		}
	}
	if len(selectedIDs) < 2 || len(selectedIDs) > 100 {
		writeFormError(w, r, http.StatusBadRequest, "Select between 2 and 100 watches to combine.")
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	selectedGroups := make([]WatchGroup, 0, len(selectedIDs))
	for _, id := range selectedIDs {
		found := false
		for _, group := range a.state.Groups {
			if group.ID == id {
				selectedGroups = append(selectedGroups, group)
				found = true
				break
			}
		}
		if !found {
			writeFormError(w, r, http.StatusNotFound, "One of the selected watches no longer exists. Reload the page and try again.")
			return
		}
	}
	addressCounts := make(map[string]int, len(selected))
	selectedWatchIDs := make(map[string]bool)
	for _, watch := range a.state.Watches {
		if selected[watch.GroupID] {
			addressCounts[watch.GroupID]++
			selectedWatchIDs[watch.ID] = true
		}
	}
	totalAddresses := 0
	hasMultiple := false
	for _, id := range selectedIDs {
		totalAddresses += addressCounts[id]
		if addressCounts[id] > 1 {
			hasMultiple = true
		}
	}
	if totalAddresses < 2 {
		writeFormError(w, r, http.StatusBadRequest, "The selected watches do not contain enough addresses to combine.")
		return
	}
	if totalAddresses > maxBulkAddresses {
		writeFormError(w, r, http.StatusBadRequest, fmt.Sprintf("A combined watch may contain at most %d addresses.", maxBulkAddresses))
		return
	}
	if hasMultiple && r.FormValue("confirm_multi") != "on" {
		writeFormError(w, r, http.StatusConflict, "Confirm that the selected multi-address groups should be included.")
		return
	}
	if notes == "" {
		notes = combinedNotes(selectedGroups)
	}

	newID := randomID()
	for i := range a.state.Watches {
		if !selected[a.state.Watches[i].GroupID] {
			continue
		}
		a.state.Watches[i].GroupID = newID
		a.state.Watches[i].Label = label
		if a.state.Watches[i].Path != "" {
			a.state.Watches[i].Label += " " + a.state.Watches[i].Path
		}
	}
	notifyMode, notifyMinimum, notifyAfter := inheritedNotificationRule(selectedGroups)
	keptGroups := a.state.Groups[:0]
	for _, group := range a.state.Groups {
		if !selected[group.ID] {
			keptGroups = append(keptGroups, group)
		}
	}
	a.state.Groups = append(keptGroups, WatchGroup{ID: newID, Label: label, Category: category, Notes: notes, Source: "combined-address-list", ScriptType: "collection", Count: totalAddresses, CreatedAt: time.Now().UTC(), Combined: true, NotifyMode: notifyMode, NotifyMinimum: notifyMinimum, NotifyAfter: notifyAfter})
	a.state.Events = consolidateEvents(a.state.Events, selected, selectedWatchIDs, newID)
	if err := a.saveLocked(); err != nil {
		writeFormError(w, r, http.StatusInternalServerError, "The selected watches could not be combined.")
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": newID})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func combinedNotes(groups []WatchGroup) string {
	var lines []string
	for _, group := range groups {
		if strings.TrimSpace(group.Notes) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("[%s / %s] %s", group.Category, group.Label, group.Notes))
	}
	runes := []rune(strings.Join(lines, "\n"))
	if len(runes) > 500 {
		runes = runes[:497]
		runes = append(runes, '.', '.', '.')
	}
	return string(runes)
}

func inheritedNotificationRule(groups []WatchGroup) (string, uint64, int) {
	if len(groups) == 0 {
		return "off", 0, 0
	}
	mode := groups[0].NotifyMode
	if mode == "" {
		mode = "all"
	}
	minimum, after := groups[0].NotifyMinimum, groups[0].NotifyAfter
	for _, group := range groups[1:] {
		candidateMode := group.NotifyMode
		if candidateMode == "" {
			candidateMode = "all"
		}
		if candidateMode != mode || group.NotifyMinimum != minimum || group.NotifyAfter != after {
			return "off", 0, 0
		}
	}
	return mode, minimum, after
}

func consolidateEvents(events []Event, selectedGroups, selectedWatchIDs map[string]bool, newGroupID string) []Event {
	result := make([]Event, 0, len(events))
	indexes := make(map[string]int)
	for _, event := range events {
		if !selectedGroups[event.GroupID] && !selectedWatchIDs[event.WatchID] {
			result = append(result, event)
			continue
		}
		event.GroupID = newGroupID
		event.TelegramSent, event.NostrSent = true, true
		if index, exists := indexes[event.TxID]; exists {
			merged := &result[index]
			merged.Received += event.Received
			merged.Sent += event.Sent
			merged.Net += event.Net
			merged.Direction = direction(electrum.Effect{Received: merged.Received, Sent: merged.Sent})
			merged.Replaceable = merged.Replaceable || event.Replaceable
			merged.Runestone = merged.Runestone || event.Runestone
			if event.Inscriptions > merged.Inscriptions {
				merged.Inscriptions = event.Inscriptions
			}
			if event.Height > merged.Height {
				merged.Height = event.Height
			}
			merged.OPReturn = appendUniqueStrings(merged.OPReturn, event.OPReturn...)
			merged.ReceivedAddresses = appendUniqueStrings(merged.ReceivedAddresses, event.ReceivedAddresses...)
			continue
		}
		indexes[event.TxID] = len(result)
		result = append(result, event)
	}
	return result
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range additions {
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	return values
}

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	if err := a.client.Ping(ctx); err != nil {
		http.Error(w, "Electrs is unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (a *App) pollLoop() {
	for {
		a.poll()
		time.Sleep(a.interval)
	}
}

type pollResult struct {
	watch        Watch
	groupID      string
	snapshot     electrum.Snapshot
	effects      map[string]electrum.Effect
	lastTxID     string
	lastTxHeight int64
	lastTxAt     time.Time
	checkedAt    time.Time
	err          error
}

func (a *App) poll() {
	a.mu.RLock()
	watches := append([]Watch(nil), a.state.Watches...)
	a.mu.RUnlock()
	groupScripts := make(map[string][][]byte)
	groupScriptAddresses := make(map[string]map[string]string)
	for _, watch := range watches {
		groupID := watch.GroupID
		if groupID == "" {
			groupID = watch.ID
		}
		script, err := bitcoin.ScriptPubKey(watch.Address)
		if err == nil {
			groupScripts[groupID] = append(groupScripts[groupID], script)
			if groupScriptAddresses[groupID] == nil {
				groupScriptAddresses[groupID] = map[string]string{}
			}
			groupScriptAddresses[groupID][string(script)] = watch.Address
		}
	}
	workerCount := 8
	if len(watches) < workerCount {
		workerCount = len(watches)
	}
	jobs := make(chan Watch)
	results := make(chan pollResult, len(watches))
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			for watch := range jobs {
				results <- a.fetchWatch(watch, groupScripts)
			}
		}()
	}
	for _, watch := range watches {
		jobs <- watch
	}
	close(jobs)
	pollResults := make([]pollResult, 0, len(watches))
	for range watches {
		pollResults = append(pollResults, <-results)
	}

	a.mu.Lock()
	watchIndexes := make(map[string]int, len(a.state.Watches))
	for index := range a.state.Watches {
		watchIndexes[a.state.Watches[index].ID] = index
	}
	for _, result := range pollResults {
		i, exists := watchIndexes[result.watch.ID]
		if !exists {
			continue
		}
		current := &a.state.Watches[i]
		current.LastChecked = result.checkedAt
		if result.err != nil {
			current.LastError = result.err.Error()
			log.Printf("poll %s: %v", result.watch.Label, result.err)
			continue
		}
		known := make(map[string]bool, len(current.KnownTx))
		for _, txid := range current.KnownTx {
			known[txid] = true
		}
		for _, item := range result.snapshot.History {
			if current.Initialized && !known[item.TxHash] {
				if !a.eventExistsLocked(result.groupID, item.TxHash) {
					effect := result.effects[item.TxHash]
					receivedAddresses := make([]string, 0, len(effect.ReceivedScripts))
					for _, script := range effect.ReceivedScripts {
						if address := groupScriptAddresses[result.groupID][script]; address != "" {
							receivedAddresses = append(receivedAddresses, address)
						}
					}
					a.state.Events = append(a.state.Events, Event{
						WatchID:           current.ID,
						GroupID:           result.groupID,
						TxID:              item.TxHash,
						Height:            item.Height,
						Direction:         direction(effect),
						Received:          effect.Received,
						Sent:              effect.Sent,
						Net:               int64(effect.Received) - int64(effect.Sent),
						OPReturn:          effect.OPReturn,
						Replaceable:       effect.Replaceable,
						Runestone:         effect.Runestone,
						Inscriptions:      effect.Inscriptions,
						ReceivedAddresses: receivedAddresses,
						SeenAt:            result.checkedAt,
					})
				}
			}
			if !known[item.TxHash] {
				current.KnownTx = append(current.KnownTx, item.TxHash)
				known[item.TxHash] = true
			}
		}
		heights := make(map[string]int64, len(result.snapshot.History))
		for _, item := range result.snapshot.History {
			heights[item.TxHash] = item.Height
		}
		for i := range a.state.Events {
			if a.state.Events[i].WatchID == current.ID || (a.state.Events[i].GroupID != "" && a.state.Events[i].GroupID == result.groupID) {
				if height, ok := heights[a.state.Events[i].TxID]; ok {
					a.state.Events[i].Height = height
				}
			}
		}
		current.Confirmed, current.Unconfirmed = result.snapshot.Balance.Confirmed, result.snapshot.Balance.Unconfirmed
		current.LastTxID, current.LastTxHeight, current.LastTxAt = result.lastTxID, result.lastTxHeight, result.lastTxAt
		current.Initialized, current.LastError = true, ""
	}
	if err := a.expandDiscoveryLocked(); err != nil {
		log.Printf("smart discovery: %v", err)
	}
	if err := a.saveLocked(); err != nil {
		log.Printf("save smart discovery: %v", err)
	}
	a.mu.Unlock()
	a.deliverPending()
}

func (a *App) fetchWatch(watch Watch, groupScripts map[string][][]byte) pollResult {
	groupID := watch.GroupID
	if groupID == "" {
		groupID = watch.ID
	}
	result := pollResult{watch: watch, groupID: groupID, effects: make(map[string]electrum.Effect), lastTxID: watch.LastTxID, lastTxHeight: watch.LastTxHeight, lastTxAt: watch.LastTxAt, checkedAt: time.Now().UTC()}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	result.snapshot, result.err = a.client.Snapshot(ctx, watch.ScriptHash)
	if result.err != nil {
		return result
	}
	if latest, ok := latestHistory(result.snapshot.History); ok {
		result.lastTxID, result.lastTxHeight = latest.TxHash, latest.Height
		if latest.TxHash != watch.LastTxID || latest.Height != watch.LastTxHeight || watch.LastTxAt.IsZero() {
			if latest.Height > 0 {
				if blockTime, err := a.client.BlockTime(ctx, latest.Height); err == nil {
					result.lastTxAt = blockTime
				} else {
					result.lastTxAt = time.Time{}
					log.Printf("latest transaction time %s: %v", watch.Label, err)
				}
			} else {
				result.lastTxAt = result.checkedAt
			}
		}
	} else {
		result.lastTxID, result.lastTxHeight, result.lastTxAt = "", 0, time.Time{}
	}
	if !watch.Initialized {
		return result
	}
	known := make(map[string]bool, len(watch.KnownTx))
	for _, txID := range watch.KnownTx {
		known[txID] = true
	}
	for _, item := range result.snapshot.History {
		if known[item.TxHash] {
			continue
		}
		effect, err := a.client.TransactionEffect(ctx, item.TxHash, groupScripts[groupID])
		if err != nil {
			result.err = err
			return result
		}
		result.effects[item.TxHash] = effect
	}
	return result
}

func latestHistory(history []electrum.HistoryItem) (electrum.HistoryItem, bool) {
	var latest electrum.HistoryItem
	found := false
	for _, item := range history {
		if !found || (item.Height <= 0 && latest.Height > 0) || (item.Height <= 0 && latest.Height <= 0) || (item.Height > latest.Height && latest.Height > 0) {
			latest, found = item, true
		}
	}
	return latest, found
}

func relativeTime(value, now time.Time) string {
	if value.IsZero() {
		return "time unavailable"
	}
	d := now.Sub(value)
	if d < 0 || d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		minutes := int(d / time.Minute)
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}
	if d < 24*time.Hour {
		hours := int(d / time.Hour)
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	if d < 14*24*time.Hour {
		days := int(d / (24 * time.Hour))
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
	if d < 60*24*time.Hour {
		weeks := int(d / (7 * 24 * time.Hour))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	}
	if d < 730*24*time.Hour {
		months := int(d / (30 * 24 * time.Hour))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
	years := int(d / (365 * 24 * time.Hour))
	if years == 1 {
		return "1 year ago"
	}
	return fmt.Sprintf("%d years ago", years)
}

func (a *App) expandDiscoveryLocked() error {
	gap := discoveryGap(a.state.DiscoveryGap)
	addressOwners := make(map[string]string, len(a.state.Watches))
	for _, watch := range a.state.Watches {
		addressOwners[watch.Address] = watch.GroupID
	}
	for groupIndex := range a.state.Groups {
		group := &a.state.Groups[groupIndex]
		if group.ScriptType == "address" || group.Bulk || group.Combined {
			continue
		}
		currentCount := group.Count
		if currentCount < 1 {
			currentCount = 1
		}
		highestUsed := -1
		for _, watch := range a.state.Watches {
			if watch.GroupID != group.ID || len(watch.KnownTx) == 0 {
				continue
			}
			if index, ok := derivedPathIndex(watch.Path); ok && index > highestUsed {
				highestUsed = index
			}
		}
		desired := gap
		if highestUsed >= 0 && highestUsed+1+gap > desired {
			desired = highestUsed + 1 + gap
		}
		limitError := ""
		if desired > 500 {
			desired = 500
			limitError = fmt.Sprintf("smart discovery reached the 500-address limit before satisfying gap %d", gap)
		}
		if desired <= currentCount {
			group.DiscoveryError = limitError
			continue
		}
		group.DiscoveryError = ""
		derived, err := bitcoin.DeriveAddresses(group.Source, group.ScriptType, desired, group.IncludeChange)
		if err != nil {
			group.DiscoveryError = err.Error()
			continue
		}
		for _, child := range derived {
			if owner, exists := addressOwners[child.Address]; exists {
				if owner != group.ID {
					group.DiscoveryError = "a newly derived address overlaps another watch"
				}
				continue
			}
			scriptHash, err := bitcoin.ScriptHash(child.Address)
			if err != nil {
				group.DiscoveryError = err.Error()
				continue
			}
			a.state.Watches = append(a.state.Watches, Watch{ID: randomID(), GroupID: group.ID, Label: group.Label + " " + child.Path, Address: child.Address, Path: child.Path, ScriptHash: scriptHash})
			addressOwners[child.Address] = group.ID
		}
		if group.DiscoveryError == "" {
			group.Count = desired
			group.DiscoveryError = limitError
		}
	}
	return nil
}

func derivedPathIndex(path string) (int, bool) {
	separator := strings.LastIndexByte(path, '/')
	if separator < 0 || separator+1 == len(path) {
		return 0, false
	}
	index, err := strconv.Atoi(path[separator+1:])
	return index, err == nil && index >= 0
}

func (a *App) deliverPending() {
	c, err := a.notifier.EnsureIdentity()
	if err != nil {
		log.Printf("notification configuration: %v", err)
		return
	}
	if c.NostrEnabled && !c.NostrProfilePublished {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		updated, profileErr := a.notifier.EnsureProfile(ctx, c)
		cancel()
		if profileErr != nil {
			log.Printf("Nostr sender profile: %v", profileErr)
		} else {
			c = updated
		}
	}
	a.mu.RLock()
	events := append([]Event(nil), a.state.Events...)
	groups := append([]WatchGroup(nil), a.state.Groups...)
	a.mu.RUnlock()
	labels := map[string]string{}
	rules := map[string]WatchGroup{}
	for _, g := range groups {
		rules[g.ID] = g
		if g.Category != "" {
			labels[g.ID] = g.Category + " / " + g.Label
		} else {
			labels[g.ID] = g.Label
		}
	}
	tipHeight := int64(0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if height, tipErr := a.client.TipHeight(ctx); tipErr == nil {
		tipHeight = height
	}
	cancel()
	if c.DailyDigest {
		a.deliverDigest(c, events, labels, rules, tipHeight, time.Now().UTC())
		return
	}
	if c.QuietHours && notificationQuietNow(c, time.Now().UTC()) {
		return
	}
	for _, event := range events {
		if (!c.TelegramEnabled || event.TelegramSent) && (!c.NostrEnabled || event.NostrSent) {
			continue
		}
		label := labels[event.GroupID]
		if label == "" {
			label = "Bitcoin watch"
		}
		eligible, waiting := notificationEligible(rules[event.GroupID], event, tipHeight)
		if waiting {
			continue
		}
		if !eligible {
			a.mu.Lock()
			for i := range a.state.Events {
				if a.state.Events[i].GroupID == event.GroupID && a.state.Events[i].TxID == event.TxID {
					if c.TelegramEnabled {
						a.state.Events[i].TelegramSent = true
					}
					if c.NostrEnabled {
						a.state.Events[i].NostrSent = true
					}
				}
			}
			_ = a.saveLocked()
			a.mu.Unlock()
			continue
		}
		msg := notify.Message(label, event.Direction, event.Received, event.Sent, event.TxID, event.Height)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		tg, nr := event.TelegramSent, event.NostrSent
		if c.TelegramEnabled && !tg {
			if e := a.notifier.Telegram(ctx, c, msg); e != nil {
				log.Printf("Telegram delivery failed: %v", e)
			} else {
				tg = true
			}
		}
		if c.NostrEnabled && !nr {
			if e := a.notifier.Nostr(ctx, c, msg); e != nil {
				log.Printf("NIP-17 delivery failed: %v", e)
			} else {
				nr = true
			}
		}
		cancel()
		a.mu.Lock()
		for i := range a.state.Events {
			if a.state.Events[i].GroupID == event.GroupID && a.state.Events[i].TxID == event.TxID {
				a.state.Events[i].TelegramSent = tg
				a.state.Events[i].NostrSent = nr
			}
		}
		_ = a.saveLocked()
		a.mu.Unlock()
	}
}

func notificationQuietNow(c notify.Config, now time.Time) bool {
	hour := now.Add(time.Duration(c.UTCOffset) * time.Hour).Hour()
	if c.QuietStart == c.QuietEnd {
		return true
	}
	if c.QuietStart < c.QuietEnd {
		return hour >= c.QuietStart && hour < c.QuietEnd
	}
	return hour >= c.QuietStart || hour < c.QuietEnd
}

func (a *App) deliverDigest(c notify.Config, events []Event, labels map[string]string, rules map[string]WatchGroup, tipHeight int64, now time.Time) {
	local := now.Add(time.Duration(c.UTCOffset) * time.Hour)
	date := local.Format("2006-01-02")
	if local.Hour() < c.DigestHour {
		return
	}
	a.mu.RLock()
	lastTelegram, lastNostr := a.state.LastTelegramDigest, a.state.LastNostrDigest
	a.mu.RUnlock()
	ready := make([]Event, 0, len(events))
	for _, event := range events {
		if (!c.TelegramEnabled || event.TelegramSent) && (!c.NostrEnabled || event.NostrSent) {
			continue
		}
		eligible, waiting := notificationEligible(rules[event.GroupID], event, tipHeight)
		if waiting {
			continue
		}
		if !eligible {
			a.markEventDelivery(event, c.TelegramEnabled, c.NostrEnabled)
			continue
		}
		ready = append(ready, event)
	}
	if len(ready) == 0 {
		return
	}
	message := digestMessage(ready, labels)
	tgSent, nrSent := false, false
	if c.TelegramEnabled && lastTelegram != date {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		if err := a.notifier.Telegram(ctx, c, message); err != nil {
			log.Printf("Telegram digest failed: %v", err)
		} else {
			tgSent = true
		}
		cancel()
	}
	if c.NostrEnabled && lastNostr != date {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		if err := a.notifier.Nostr(ctx, c, message); err != nil {
			log.Printf("NIP-17 digest failed: %v", err)
		} else {
			nrSent = true
		}
		cancel()
	}
	if !tgSent && !nrSent {
		return
	}
	a.mu.Lock()
	for i := range a.state.Events {
		for _, event := range ready {
			if a.state.Events[i].GroupID == event.GroupID && a.state.Events[i].TxID == event.TxID {
				if tgSent {
					a.state.Events[i].TelegramSent = true
				}
				if nrSent {
					a.state.Events[i].NostrSent = true
				}
			}
		}
	}
	if tgSent {
		a.state.LastTelegramDigest = date
	}
	if nrSent {
		a.state.LastNostrDigest = date
	}
	_ = a.saveLocked()
	a.mu.Unlock()
}

func (a *App) markEventDelivery(event Event, telegram, nostr bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i := range a.state.Events {
		if a.state.Events[i].GroupID == event.GroupID && a.state.Events[i].TxID == event.TxID {
			if telegram {
				a.state.Events[i].TelegramSent = true
			}
			if nostr {
				a.state.Events[i].NostrSent = true
			}
		}
	}
	_ = a.saveLocked()
}

func digestMessage(events []Event, labels map[string]string) string {
	var message strings.Builder
	fmt.Fprintf(&message, "s/watcher daily digest: %d activities", len(events))
	limit := len(events)
	if limit > 10 {
		limit = 10
	}
	for _, event := range events[:limit] {
		label := labels[event.GroupID]
		if label == "" {
			label = "Bitcoin watch"
		}
		fmt.Fprintf(&message, "\n\n%s\n%s · received %d sat · sent %d sat\n%s", label, event.Direction, event.Received, event.Sent, event.TxID)
	}
	if len(events) > limit {
		fmt.Fprintf(&message, "\n\n…and %d more activities.", len(events)-limit)
	}
	return message.String()
}

func notificationEligible(group WatchGroup, event Event, tipHeight int64) (eligible, waiting bool) {
	mode := group.NotifyMode
	if mode == "" {
		mode = "all"
	}
	if mode == "off" {
		return false, false
	}
	if mode == "incoming" && event.Direction != "received" {
		return false, false
	}
	if mode == "outgoing" && event.Direction != "sent" {
		return false, false
	}
	amount := event.Received
	if event.Sent > amount {
		amount = event.Sent
	}
	if amount < group.NotifyMinimum {
		return false, false
	}
	if group.NotifyAfter > 0 {
		if event.Height <= 0 {
			return false, true
		}
		confirmations := 1
		if tipHeight >= event.Height {
			confirmations = int(tipHeight-event.Height) + 1
		}
		if confirmations < group.NotifyAfter {
			return false, true
		}
	}
	return true, false
}

func (a *App) eventExistsLocked(groupID, txID string) bool {
	for _, event := range a.state.Events {
		eventGroupID := event.GroupID
		if eventGroupID == "" {
			eventGroupID = event.WatchID
		}
		if eventGroupID == groupID && event.TxID == txID {
			return true
		}
	}
	return false
}

func direction(effect electrum.Effect) string {
	switch {
	case effect.Received > 0 && effect.Sent > 0:
		return "self-transfer"
	case effect.Received > 0:
		return "received"
	case effect.Sent > 0:
		return "sent"
	default:
		return "activity"
	}
}

func (a *App) groupViewsLocked() []groupView {
	views := make([]groupView, 0, len(a.state.Groups))
	indexes := make(map[string]int, len(a.state.Groups))
	for _, group := range a.state.Groups {
		scriptTypeKey := group.ScriptType
		if group.Category == "" {
			group.Category = "uncategorized"
		}
		if group.Combined {
			group.Source = fmt.Sprintf("%d addresses combined manually", group.Count)
			group.ScriptType = "combined watch group"
		} else if group.Bulk {
			group.Source = fmt.Sprintf("%d addresses pasted in bulk", group.Count)
			group.ScriptType = "bulk address list"
		} else if group.ScriptType != "address" {
			group.ScriptType += fmt.Sprintf(" · smart gap %d", discoveryGap(a.state.DiscoveryGap))
		}
		indexes[group.ID] = len(views)
		typeName, typeCode, typeURL := addressTypeDetails(group.Source, scriptTypeKey)
		if group.Bulk || group.Combined {
			typeName, typeCode, typeURL = "", "", ""
		}
		views = append(views, groupView{WatchGroup: group, LastError: group.DiscoveryError, ScriptTypeKey: scriptTypeKey, CanEditScriptType: isBareXpub(group.Source), AddressTypeName: typeName, AddressTypeCode: typeCode, AddressTypeURL: typeURL})
	}
	for _, watch := range a.state.Watches {
		groupID := watch.GroupID
		if groupID == "" {
			groupID = watch.ID
		}
		index, ok := indexes[groupID]
		if !ok {
			indexes[groupID] = len(views)
			views = append(views, groupView{WatchGroup: WatchGroup{
				ID:         groupID,
				Label:      watch.Label,
				Category:   "uncategorized",
				Source:     watch.Address,
				ScriptType: "address",
				Count:      1,
			}})
			index = len(views) - 1
		}
		view := &views[index]
		if view.Bulk || view.Combined {
			typeName, typeCode, typeURL := addressTypeDetails(watch.Address, "address")
			if view.AddressTypeCode == "" {
				view.AddressTypeName, view.AddressTypeCode, view.AddressTypeURL = typeName, typeCode, typeURL
			} else if view.AddressTypeCode != typeCode {
				view.AddressTypeName, view.AddressTypeCode, view.AddressTypeURL = "Mixed mainnet address types", "multiple types", "https://github.com/bitcoin/bips"
			}
		}
		view.Addresses++
		view.Confirmed += watch.Confirmed
		view.Unconfirmed += watch.Unconfirmed
		if watch.Initialized {
			view.Ready++
		}
		if watch.LastChecked.After(view.LastChecked) {
			view.LastChecked = watch.LastChecked
		}
		if watch.LastTxAt.After(view.LastTxAt) || (view.LastTxID == "" && watch.LastTxID != "") {
			view.LastTxID, view.LastTxAt = watch.LastTxID, watch.LastTxAt
		}
		if view.LastError == "" && watch.LastError != "" {
			view.LastError = watch.LastError
		}
	}
	return views
}

func addressTypeDetails(source, scriptType string) (string, string, string) {
	if scriptType == "address" {
		switch {
		case strings.HasPrefix(source, "1"):
			return "Legacy mainnet · Pay-to-Public-Key-Hash", "P2PKH", "https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki"
		case strings.HasPrefix(source, "3"):
			return "Script Hash mainnet · Pay-to-Script-Hash", "P2SH", "https://github.com/bitcoin/bips/blob/master/bip-0016.mediawiki"
		case strings.HasPrefix(source, "bc1p"):
			return "Taproot mainnet · Pay-to-Taproot", "P2TR", "https://github.com/bitcoin/bips/blob/master/bip-0086.mediawiki"
		case strings.HasPrefix(source, "bc1q") && len(source) > 50:
			return "SegWit mainnet · Pay-to-Witness-Script-Hash", "P2WSH", "https://github.com/bitcoin/bips/blob/master/bip-0173.mediawiki"
		case strings.HasPrefix(source, "bc1q"):
			return "SegWit mainnet · Pay-to-Witness-Public-Key-Hash", "P2WPKH", "https://github.com/bitcoin/bips/blob/master/bip-0173.mediawiki"
		}
	}
	switch scriptType {
	case "legacy":
		return "Legacy mainnet · Pay-to-Public-Key-Hash", "P2PKH", "https://github.com/bitcoin/bips/blob/master/bip-0044.mediawiki"
	case "nested-segwit":
		return "Nested SegWit mainnet · Pay-to-Script-Hash wrapped SegWit", "P2SH-P2WPKH", "https://github.com/bitcoin/bips/blob/master/bip-0049.mediawiki"
	case "native-segwit":
		return "Native SegWit mainnet · Pay-to-Witness-Public-Key-Hash", "P2WPKH", "https://github.com/bitcoin/bips/blob/master/bip-0084.mediawiki"
	case "taproot":
		return "Taproot mainnet · Pay-to-Taproot", "P2TR", "https://github.com/bitcoin/bips/blob/master/bip-0086.mediawiki"
	default:
		return "Bitcoin mainnet", "Address", "https://github.com/bitcoin/bips"
	}
}

func (a *App) watchLabel(id string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, w := range a.state.Watches {
		if w.ID == id {
			return w.Label
		}
	}
	for _, group := range a.state.Groups {
		if group.ID == id {
			return group.Label
		}
	}
	return "Removed watch"
}

func (a *App) load() error {
	b, err := os.ReadFile(a.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	if err := json.Unmarshal(b, &a.state); err != nil {
		return fmt.Errorf("decode state: %w", err)
	}
	return nil
}

func (a *App) saveLocked() error {
	b, err := json.MarshalIndent(a.state, "", "  ")
	if err != nil {
		return err
	}
	tmp := a.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, a.path)
}

func SetPrivacyMode(dataDir string, enabled bool, password string) error {
	auth, err := webauth.Load(filepath.Join(dataDir, "auth.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("set the Web Password before enabling Privacy Mode")
		}
		return fmt.Errorf("load web password: %w", err)
	}
	if !enabled && !webauth.Verify(auth, password) {
		return errors.New("the Web Password is incorrect; Privacy Mode remains enabled")
	}
	path := filepath.Join(dataDir, "state.json")
	var current state
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &current); err != nil {
			return fmt.Errorf("decode state: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read state: %w", err)
	}
	current.PrivacyMode = enabled
	b, err = json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func SetDiscoveryGap(dataDir string, gap int) error {
	if gap < 1 || gap > 500 {
		return errors.New("discovery gap must be between 1 and 500")
	}
	path := filepath.Join(dataDir, "state.json")
	var current state
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &current); err != nil {
			return fmt.Errorf("decode state: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read state: %w", err)
	}
	current.DiscoveryGap = gap
	b, err = json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func SetPrivacyIndicators(dataDir string, threshold uint64, reuse, small, combined bool) error {
	if threshold == 0 {
		return errors.New("small-deposit threshold must be positive")
	}
	path := filepath.Join(dataDir, "state.json")
	var current state
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &current); err != nil {
			return fmt.Errorf("decode state: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read state: %w", err)
	}
	current.PrivacyIndicatorsConfigured = true
	current.AddressReuseIndicators = reuse
	current.SmallDepositIndicators = small
	current.CombinedWalletIndicators = combined
	current.SmallDepositThreshold = threshold
	b, err = json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func SetTheme(dataDir, theme string) error {
	if !themes[theme] {
		return errors.New("unknown theme")
	}
	path := filepath.Join(dataDir, "state.json")
	var current state
	b, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(b, &current); err != nil {
			return fmt.Errorf("decode state: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read state: %w", err)
	}
	current.Theme = theme
	b, err = json.MarshalIndent(current, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	return nil
}

func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func maskIdentifier(value string) string {
	runes := []rune(value)
	if len(runes) <= 8 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + strings.Repeat("*", len(runes)-8) + string(runes[len(runes)-4:])
}

func legacyMask(length int) string {
	symbols := []rune("🬀🬁🬂🬃🬄🬅🬆🬇🬈🬉🬊🬋🬌🬍🬎🬏🬐🬑🬒🬓🬔🬕🬖🬗🬘🬙🬚🬛🬜🬝🬞🬟")
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return strings.Repeat("*", length)
	}
	result := make([]rune, length)
	for i := range result {
		result[i] = symbols[int(b[i])%len(symbols)]
	}
	return string(result)
}

func eventAmount(event Event) string {
	switch event.Direction {
	case "received":
		return fmt.Sprintf("+%d sat", event.Received)
	case "sent":
		return fmt.Sprintf("-%d sat", event.Sent)
	case "self-transfer":
		return fmt.Sprintf("net %d sat · in %d / out %d", event.Net, event.Received, event.Sent)
	default:
		return "—"
	}
}

func movementSignal(event Event) string {
	switch {
	case event.Received > event.Sent:
		return "received"
	case event.Sent > event.Received:
		return "sent"
	default:
		return "idle"
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonceBytes := make([]byte, 18)
		if _, err := rand.Read(nonceBytes); err != nil {
			http.Error(w, "could not secure response", http.StatusInternalServerError)
			return
		}
		nonce := base64.RawStdEncoding.EncodeToString(nonceBytes)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; object-src 'none'; connect-src 'self'; img-src 'self' https://api.dicebear.com data:; style-src 'unsafe-inline'; script-src 'nonce-"+nonce+"'; form-action 'self'; frame-ancestors 'none'")
		if r.Method == http.MethodPost {
			fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
			if fetchSite == "cross-site" {
				http.Error(w, "cross-site request rejected", http.StatusForbidden)
				return
			}
			// StartOS terminates Tor and LAN requests at its reverse proxy. The
			// browser sees the public .onion or LAN origin while the application
			// can see an internal Host value. Fetch Metadata is supplied by modern
			// browsers before that proxy boundary and is therefore authoritative.
			if origin := r.Header.Get("Origin"); origin != "" && fetchSite == "" {
				u, err := url.Parse(origin)
				expectedHost := r.Host
				if forwardedHost := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0]); forwardedHost != "" {
					expectedHost = forwardedHost
				}
				if err != nil || !strings.EqualFold(u.Host, expectedHost) {
					http.Error(w, "cross-site request rejected", http.StatusForbidden)
					return
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cspNonceKey{}, nonce)))
	})
}

type cspNonceKey struct{}

func cspNonce(r *http.Request) string {
	nonce, _ := r.Context().Value(cspNonceKey{}).(string)
	return nonce
}
