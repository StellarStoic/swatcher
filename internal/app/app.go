package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/s-watcher/s-watcher/internal/bitcoin"
	"github.com/s-watcher/s-watcher/internal/electrum"
	"github.com/s-watcher/s-watcher/internal/notify"
)

type Watch struct {
	ID          string    `json:"id"`
	GroupID     string    `json:"groupId,omitempty"`
	Label       string    `json:"label"`
	Address     string    `json:"address"`
	Path        string    `json:"path,omitempty"`
	ScriptHash  string    `json:"scriptHash"`
	Confirmed   int64     `json:"confirmed"`
	Unconfirmed int64     `json:"unconfirmed"`
	KnownTx     []string  `json:"knownTx"`
	Initialized bool      `json:"initialized"`
	LastChecked time.Time `json:"lastChecked,omitempty"`
	LastError   string    `json:"lastError,omitempty"`
}

type WatchGroup struct {
	ID            string    `json:"id"`
	Label         string    `json:"label"`
	Category      string    `json:"category,omitempty"`
	Source        string    `json:"source"`
	ScriptType    string    `json:"scriptType,omitempty"`
	Count         int       `json:"count"`
	IncludeChange bool      `json:"includeChange,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type Event struct {
	WatchID      string    `json:"watchId"`
	GroupID      string    `json:"groupId,omitempty"`
	TxID         string    `json:"txid"`
	Height       int64     `json:"height"`
	Direction    string    `json:"direction"`
	Received     uint64    `json:"received"`
	Sent         uint64    `json:"sent"`
	Net          int64     `json:"net"`
	SeenAt       time.Time `json:"seenAt"`
	TelegramSent bool      `json:"telegramSent,omitempty"`
	NostrSent    bool      `json:"nostrSent,omitempty"`
}

type state struct {
	Watches []Watch      `json:"watches"`
	Events  []Event      `json:"events"`
	Groups  []WatchGroup `json:"groups,omitempty"`
}

type groupView struct {
	WatchGroup
	Confirmed   int64
	Unconfirmed int64
	Addresses   int
	Ready       int
	LastChecked time.Time
	LastError   string
}

type pageData struct {
	Groups []groupView
	Events []Event
	Nostr  *nostrIdentityView
}

type nostrIdentityView struct {
	Name   string
	Npub   string
	Avatar string
}

type App struct {
	mu       sync.RWMutex
	state    state
	path     string
	client   *electrum.Client
	interval time.Duration
	tmpl     *template.Template
	notifier notify.Sender
}

func New(dataDir, electrumAddress string, interval time.Duration) (*App, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	a := &App{path: filepath.Join(dataDir, "state.json"), client: &electrum.Client{Address: electrumAddress}, interval: interval, notifier: notify.Sender{Path: filepath.Join(dataDir, "notifications.json")}}
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
	mux.HandleFunc("GET /", a.index)
	mux.HandleFunc("POST /watches", a.addWatch)
	mux.HandleFunc("POST /groups/{id}/update", a.updateGroup)
	mux.HandleFunc("POST /groups/{id}/delete", a.deleteGroup)
	mux.HandleFunc("GET /health", a.health)
	go a.pollLoop()
	server := &http.Server{Addr: listen, Handler: securityHeaders(mux), ReadHeaderTimeout: 5 * time.Second}
	return server.ListenAndServe()
}

func (a *App) index(w http.ResponseWriter, r *http.Request) {
	a.mu.RLock()
	data := pageData{Groups: a.groupViewsLocked(), Events: append([]Event(nil), a.state.Events...)}
	a.mu.RUnlock()
	if c, err := a.notifier.Load(); err == nil && c.NostrEnabled && c.NostrSenderNpub != "" {
		data.Nostr = &nostrIdentityView{Name: c.NostrSenderName, Npub: c.NostrSenderNpub, Avatar: c.NostrAvatar}
	}
	sort.Slice(data.Events, func(i, j int) bool { return data.Events[i].SeenAt.After(data.Events[j].SeenAt) })
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.tmpl.Execute(w, data); err != nil {
		log.Printf("render: %v", err)
	}
}

func (a *App) addWatch(w http.ResponseWriter, r *http.Request) {
	source := strings.TrimSpace(r.FormValue("source"))
	label := strings.TrimSpace(r.FormValue("label"))
	category := strings.TrimSpace(r.FormValue("category"))
	if label == "" {
		label = "Bitcoin address"
	}
	if len(label) > 80 {
		http.Error(w, "label is too long", http.StatusBadRequest)
		return
	}
	if category == "" {
		category = "Uncategorized"
	}
	if len(category) > 60 {
		http.Error(w, "category is too long", http.StatusBadRequest)
		return
	}
	groupID := randomID()
	newWatches := []Watch{}
	scriptType := strings.TrimSpace(r.FormValue("script_type"))
	count := 1
	includeChange := r.FormValue("include_change") == "on"
	if scriptHash, err := bitcoin.ScriptHash(source); err == nil {
		newWatches = append(newWatches, Watch{ID: randomID(), GroupID: groupID, Label: label, Address: source, ScriptHash: scriptHash})
		scriptType = "address"
	} else {
		parsedCount, parseErr := strconv.Atoi(r.FormValue("count"))
		if parseErr != nil {
			http.Error(w, "derivation count must be a number", http.StatusBadRequest)
			return
		}
		count = parsedCount
		derived, deriveErr := bitcoin.DeriveAddresses(source, scriptType, count, includeChange)
		if deriveErr != nil {
			http.Error(w, deriveErr.Error(), http.StatusBadRequest)
			return
		}
		for _, child := range derived {
			scriptHash, hashErr := bitcoin.ScriptHash(child.Address)
			if hashErr != nil {
				http.Error(w, hashErr.Error(), http.StatusBadRequest)
				return
			}
			newWatches = append(newWatches, Watch{ID: randomID(), GroupID: groupID, Label: label + " " + child.Path, Address: child.Address, Path: child.Path, ScriptHash: scriptHash})
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, existing := range a.state.Watches {
		for _, candidate := range newWatches {
			if existing.Address == candidate.Address {
				http.Error(w, "one or more derived addresses are already watched", http.StatusConflict)
				return
			}
		}
	}
	a.state.Watches = append(a.state.Watches, newWatches...)
	a.state.Groups = append(a.state.Groups, WatchGroup{ID: groupID, Label: label, Category: category, Source: source, ScriptType: scriptType, Count: count, IncludeChange: includeChange, CreatedAt: time.Now().UTC()})
	if err := a.saveLocked(); err != nil {
		http.Error(w, "could not save watch", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *App) updateGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	label := strings.TrimSpace(r.FormValue("label"))
	category := strings.TrimSpace(r.FormValue("category"))
	if label == "" || len(label) > 80 || category == "" || len(category) > 60 {
		http.Error(w, "name and category are required and must fit their limits", http.StatusBadRequest)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	found := false
	for i := range a.state.Groups {
		if a.state.Groups[i].ID == id {
			a.state.Groups[i].Label = label
			a.state.Groups[i].Category = category
			found = true
		}
	}
	if !found {
		http.Error(w, "watch group not found", http.StatusNotFound)
		return
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
		http.Error(w, "could not save watch metadata", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

func (a *App) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	if err := a.client.Ping(ctx); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (a *App) pollLoop() {
	a.poll()
	ticker := time.NewTicker(a.interval)
	defer ticker.Stop()
	for range ticker.C {
		a.poll()
	}
}

func (a *App) poll() {
	a.mu.RLock()
	watches := append([]Watch(nil), a.state.Watches...)
	a.mu.RUnlock()
	groupScripts := make(map[string][][]byte)
	for _, watch := range watches {
		groupID := watch.GroupID
		if groupID == "" {
			groupID = watch.ID
		}
		script, err := bitcoin.ScriptPubKey(watch.Address)
		if err == nil {
			groupScripts[groupID] = append(groupScripts[groupID], script)
		}
	}
	for _, watch := range watches {
		groupID := watch.GroupID
		if groupID == "" {
			groupID = watch.ID
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		snapshot, err := a.client.Snapshot(ctx, watch.ScriptHash)
		effects := make(map[string]electrum.Effect)
		if err == nil && watch.Initialized {
			known := make(map[string]bool, len(watch.KnownTx))
			for _, txID := range watch.KnownTx {
				known[txID] = true
			}
			for _, item := range snapshot.History {
				if err != nil || known[item.TxHash] {
					continue
				}
				effect, effectErr := a.client.TransactionEffect(ctx, item.TxHash, groupScripts[groupID])
				if effectErr != nil {
					err = effectErr
					break
				}
				effects[item.TxHash] = effect
			}
		}
		cancel()
		a.mu.Lock()
		for i := range a.state.Watches {
			current := &a.state.Watches[i]
			if current.ID != watch.ID {
				continue
			}
			current.LastChecked = time.Now().UTC()
			if err != nil {
				current.LastError = err.Error()
				log.Printf("poll %s: %v", watch.Label, err)
				break
			}
			known := make(map[string]bool, len(current.KnownTx))
			for _, txid := range current.KnownTx {
				known[txid] = true
			}
			for _, item := range snapshot.History {
				if current.Initialized && !known[item.TxHash] {
					if !a.eventExistsLocked(groupID, item.TxHash) {
						effect := effects[item.TxHash]
						a.state.Events = append(a.state.Events, Event{
							WatchID:   current.ID,
							GroupID:   groupID,
							TxID:      item.TxHash,
							Height:    item.Height,
							Direction: direction(effect),
							Received:  effect.Received,
							Sent:      effect.Sent,
							Net:       int64(effect.Received) - int64(effect.Sent),
							SeenAt:    time.Now().UTC(),
						})
					}
				}
				if !known[item.TxHash] {
					current.KnownTx = append(current.KnownTx, item.TxHash)
					known[item.TxHash] = true
				}
			}
			heights := make(map[string]int64, len(snapshot.History))
			for _, item := range snapshot.History {
				heights[item.TxHash] = item.Height
			}
			for i := range a.state.Events {
				if a.state.Events[i].WatchID == current.ID || (a.state.Events[i].GroupID != "" && a.state.Events[i].GroupID == groupID) {
					if height, ok := heights[a.state.Events[i].TxID]; ok {
						a.state.Events[i].Height = height
					}
				}
			}
			current.Confirmed, current.Unconfirmed = snapshot.Balance.Confirmed, snapshot.Balance.Unconfirmed
			current.Initialized, current.LastError = true, ""
			break
		}
		if saveErr := a.saveLocked(); saveErr != nil {
			log.Printf("save: %v", saveErr)
		}
		a.mu.Unlock()
	}
	a.deliverPending()
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
	if c.TelegramTestPending || c.NostrTestPending {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		updated, testErr := a.notifier.DeliverTests(ctx, c, "You receive this message because you enabled Notifications in s-watcher. Consider this a test message.")
		cancel()
		c = updated
		if testErr != nil {
			log.Printf("notification test delivery: %v", testErr)
		}
	}
	a.mu.RLock()
	events := append([]Event(nil), a.state.Events...)
	groups := append([]WatchGroup(nil), a.state.Groups...)
	a.mu.RUnlock()
	labels := map[string]string{}
	for _, g := range groups {
		if g.Category != "" {
			labels[g.ID] = g.Category + " / " + g.Label
		} else {
			labels[g.ID] = g.Label
		}
	}
	for _, event := range events {
		if (!c.TelegramEnabled || event.TelegramSent) && (!c.NostrEnabled || event.NostrSent) {
			continue
		}
		label := labels[event.GroupID]
		if label == "" {
			label = "Bitcoin watch"
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
		if group.Category == "" {
			group.Category = "Uncategorized"
		}
		indexes[group.ID] = len(views)
		views = append(views, groupView{WatchGroup: group})
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
				Category:   "Uncategorized",
				Source:     watch.Address,
				ScriptType: "address",
				Count:      1,
			}})
			index = len(views) - 1
		}
		view := &views[index]
		view.Addresses++
		view.Confirmed += watch.Confirmed
		view.Unconfirmed += watch.Unconfirmed
		if watch.Initialized {
			view.Ready++
		}
		if watch.LastChecked.After(view.LastChecked) {
			view.LastChecked = watch.LastChecked
		}
		if view.LastError == "" && watch.LastError != "" {
			view.LastError = watch.LastError
		}
	}
	return views
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

func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https://api.dicebear.com data:; style-src 'unsafe-inline'; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
