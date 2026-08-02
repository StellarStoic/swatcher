package notify

import (
	"bytes"
	"context"
	"crypto/sha256"
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
	TelegramEnabled bool     `json:"telegramEnabled"`
	TelegramToken   string   `json:"telegramToken"`
	TelegramChatID  string   `json:"telegramChatId"`
	NostrEnabled    bool     `json:"nostrEnabled"`
	NostrRelays     []string `json:"nostrRelays"`
	NostrRecipient  string   `json:"nostrRecipient"`
	NostrSenderName string   `json:"nostrSenderName"`
	NostrSenderNsec string   `json:"nostrSenderNsec"`
	NostrSenderNpub string   `json:"nostrSenderNpub"`
	NostrAvatar     string   `json:"nostrAvatar"`
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
	styles := []string{"bottts", "identicon", "shapes", "rings", "thumbs"}
	sum := sha256.Sum256([]byte(pk))
	c.NostrAvatar = "https://api.dicebear.com/9.x/" + styles[int(sum[0])%len(styles)] + "/svg?seed=" + url.QueryEscape(c.NostrSenderNpub)
	b, _ := json.MarshalIndent(c, "", "  ")
	tmp := s.Path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return c, e
	}
	return c, os.Rename(tmp, s.Path)
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
	metadata, _ := json.Marshal(map[string]string{"name": c.NostrSenderName, "display_name": c.NostrSenderName, "picture": c.NostrAvatar, "about": "Private Bitcoin activity alerts from s-watcher"})
	pubkey, _ := kr.GetPublicKey(ctx)
	profile := nostr.Event{PubKey: pubkey, CreatedAt: nostr.Now(), Kind: nostr.KindProfileMetadata, Content: string(metadata)}
	if signErr := kr.SignEvent(ctx, &profile); signErr == nil {
		for _, relayURL := range c.NostrRelays {
			if relay, relayErr := pool.EnsureRelay(relayURL); relayErr == nil {
				_ = relay.Publish(ctx, profile)
			}
		}
	}
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
