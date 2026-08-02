package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/keyer"
	"github.com/nbd-wtf/go-nostr/nip17"
	"github.com/nbd-wtf/go-nostr/nip19"
)

type Config struct {
	TelegramEnabled       bool     `json:"telegramEnabled"`
	TelegramToken         string   `json:"telegramToken"`
	TelegramChatID        string   `json:"telegramChatId"`
	NostrEnabled          bool     `json:"nostrEnabled"`
	NostrRelays           []string `json:"nostrRelays"`
	NostrRecipient        string   `json:"nostrRecipient"`
	NostrSenderName       string   `json:"nostrSenderName"`
	NostrSenderNsec       string   `json:"nostrSenderNsec"`
	NostrSenderNpub       string   `json:"nostrSenderNpub"`
	NostrAvatar           string   `json:"nostrAvatar"`
	NostrProfilePublished bool     `json:"nostrProfilePublished"`
	TelegramTestPending   bool     `json:"telegramTestPending"`
	NostrTestPending      bool     `json:"nostrTestPending"`
}
type Sender struct {
	Path string
	HTTP *http.Client
}

func (s Sender) Load() (Config, error) {
	var c Config
	b, e := os.ReadFile(s.Path)
	if errors.Is(e, os.ErrNotExist) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	return c, json.Unmarshal(b, &c)
}
func (s Sender) EnsureIdentity() (Config, error) {
	c, e := s.Load()
	if e != nil || !c.NostrEnabled {
		return c, e
	}
	var sk string
	if c.NostrSenderNsec == "" {
		sk = nostr.GeneratePrivateKey()
		c.NostrSenderNsec, _ = nip19.EncodePrivateKey(sk)
	} else {
		p, v, x := nip19.Decode(c.NostrSenderNsec)
		if x != nil || p != "nsec" {
			return c, errors.New("invalid Nostr sender nsec")
		}
		sk = v.(string)
	}
	pk, e := nostr.GetPublicKey(sk)
	if e != nil {
		return c, e
	}
	c.NostrSenderNpub, _ = nip19.EncodePublicKey(pk)
	c.NostrAvatar = "https://api.dicebear.com/10.x/pixelbot/svg?seed=" + url.QueryEscape(c.NostrSenderNpub)
	return c, s.save(c)
}
func (s Sender) save(c Config) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.Path + ".tmp"
	if err = os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}

func (s Sender) EnsureProfile(ctx context.Context, c Config) (Config, error) {
	if !c.NostrEnabled || c.NostrProfilePublished {
		return c, nil
	}
	if len(c.NostrRelays) == 0 {
		return c, errors.New("Nostr relays missing for sender profile")
	}
	prefix, secret, err := nip19.Decode(c.NostrSenderNsec)
	if err != nil || prefix != "nsec" {
		return c, errors.New("invalid sender nsec")
	}
	kr, err := keyer.NewPlainKeySigner(secret.(string))
	if err != nil {
		return c, err
	}
	metadata, _ := json.Marshal(map[string]string{"name": c.NostrSenderName, "display_name": c.NostrSenderName, "picture": c.NostrAvatar, "about": "Private Bitcoin activity alerts from s-watcher"})
	pubkey, _ := kr.GetPublicKey(ctx)
	profile := nostr.Event{PubKey: pubkey, CreatedAt: nostr.Now(), Kind: nostr.KindProfileMetadata, Content: string(metadata)}
	if err = kr.SignEvent(ctx, &profile); err != nil {
		return c, err
	}
	pool := nostr.NewSimplePool(ctx)
	defer pool.Close("profile published")
	published := false
	for _, relayURL := range c.NostrRelays {
		if relay, relayErr := pool.EnsureRelay(relayURL); relayErr == nil {
			if relay.Publish(ctx, profile) == nil {
				published = true
			}
		}
	}
	if !published {
		return c, errors.New("could not publish sender profile to any configured relay")
	}
	c.NostrProfilePublished = true
	return c, s.save(c)
}

func (s Sender) DeliverTests(ctx context.Context, c Config, message string) (Config, error) {
	var deliveryErrors []error
	changed := false
	if !c.TelegramEnabled && c.TelegramTestPending {
		c.TelegramTestPending = false
		changed = true
	}
	if !c.NostrEnabled && c.NostrTestPending {
		c.NostrTestPending = false
		changed = true
	}
	if c.TelegramEnabled && c.TelegramTestPending {
		if err := s.Telegram(ctx, c, message); err != nil {
			deliveryErrors = append(deliveryErrors, fmt.Errorf("Telegram test: %w", err))
		} else {
			c.TelegramTestPending = false
			changed = true
		}
	}
	if c.NostrEnabled && c.NostrTestPending {
		if err := s.Nostr(ctx, c, message); err != nil {
			deliveryErrors = append(deliveryErrors, fmt.Errorf("Nostr test: %w", err))
		} else {
			c.NostrTestPending = false
			changed = true
		}
	}
	if changed {
		if err := s.save(c); err != nil {
			deliveryErrors = append(deliveryErrors, fmt.Errorf("save test delivery state: %w", err))
		}
	}
	return c, errors.Join(deliveryErrors...)
}
func (s Sender) Telegram(ctx context.Context, c Config, message string) error {
	if !c.TelegramEnabled {
		return nil
	}
	if c.TelegramToken == "" || c.TelegramChatID == "" {
		return errors.New("Telegram credentials missing")
	}
	b, _ := json.Marshal(map[string]string{"chat_id": c.TelegramChatID, "text": message})
	r, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+c.TelegramToken+"/sendMessage", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	x, e := s.client().Do(r)
	if e != nil {
		return e
	}
	defer x.Body.Close()
	if x.StatusCode/100 != 2 {
		return fmt.Errorf("Telegram returned %s", x.Status)
	}
	return nil
}
func (s Sender) Nostr(ctx context.Context, c Config, message string) error {
	if !c.NostrEnabled {
		return nil
	}
	if len(c.NostrRelays) == 0 || c.NostrRecipient == "" {
		return errors.New("Nostr relays/recipient missing")
	}
	p, rv, e := nip19.Decode(c.NostrRecipient)
	if e != nil || p != "npub" {
		return errors.New("invalid recipient npub")
	}
	p, sv, e := nip19.Decode(c.NostrSenderNsec)
	if e != nil || p != "nsec" {
		return errors.New("invalid sender nsec")
	}
	kr, e := keyer.NewPlainKeySigner(sv.(string))
	if e != nil {
		return e
	}
	pool := nostr.NewSimplePool(ctx)
	defer pool.Close("notification complete")
	their := nip17.GetDMRelays(ctx, rv.(string), pool, c.NostrRelays)
	if len(their) == 0 {
		return errors.New("recipient has no NIP-17 kind 10050 relay list")
	}
	return nip17.PublishMessage(ctx, message, nil, pool, c.NostrRelays, their, kr, rv.(string), nil)
}
func (s Sender) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}
func Message(label, direction string, received, sent uint64, txid string, height int64) string {
	state := "mempool"
	if height > 0 {
		state = fmt.Sprintf("block %d", height)
	}
	return strings.TrimSpace(fmt.Sprintf("s-watcher: %s\n%s · received %d sat · sent %d sat\n%s\n%s", label, direction, received, sent, state, txid))
}
