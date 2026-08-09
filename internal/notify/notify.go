// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/keyer"
	"github.com/nbd-wtf/go-nostr/nip17"
	"github.com/nbd-wtf/go-nostr/nip19"
	"golang.org/x/net/proxy"
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
	DailyDigest           bool     `json:"dailyDigest"`
	DigestHour            int      `json:"digestHour"`
	QuietHours            bool     `json:"quietHours"`
	QuietStart            int      `json:"quietStart"`
	QuietEnd              int      `json:"quietEnd"`
	UTCOffset             int      `json:"utcOffset"`
}
type Sender struct {
	Path string
	HTTP *http.Client
}

// ConfigureTorSOCKS routes only .onion HTTP and WebSocket connections through
// the StartOS Tor SOCKS bridge. Clearnet Telegram and Nostr traffic remains
// direct. It must be called before any notification connections are opened.
func ConfigureTorSOCKS(address string) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		return fmt.Errorf("invalid Tor SOCKS address: %w", err)
	}

	direct := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	tor, err := proxy.SOCKS5("tcp", address, nil, direct)
	if err != nil {
		return fmt.Errorf("configure Tor SOCKS: %w", err)
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return errors.New("configure Tor SOCKS: unsupported default HTTP transport")
	}
	configured := transport.Clone()
	configured.DialContext = conditionalDialContext(direct, tor)
	http.DefaultTransport = configured
	return nil
}

func conditionalDialContext(direct *net.Dialer, tor proxy.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(strings.ToLower(strings.TrimSuffix(host, ".")), ".onion") {
			return direct.DialContext(ctx, network, address)
		}
		if contextual, ok := tor.(proxy.ContextDialer); ok {
			return contextual.DialContext(ctx, network, address)
		}
		return tor.Dial(network, address)
	}
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
	metadata, _ := json.Marshal(map[string]string{"name": c.NostrSenderName, "display_name": c.NostrSenderName, "picture": c.NostrAvatar, "about": "Private Bitcoin activity alerts from s/watcher"})
	pubkey, _ := kr.GetPublicKey(ctx)
	profile := nostr.Event{PubKey: pubkey, CreatedAt: nostr.Now(), Kind: nostr.KindProfileMetadata, Content: string(metadata)}
	if err = kr.SignEvent(ctx, &profile); err != nil {
		return c, err
	}
	pool := nostr.NewSimplePool(ctx)
	defer pool.Close("profile published")
	if err := publishToAny(ctx, pool, c.NostrRelays, profile, kr); err != nil {
		return c, fmt.Errorf("could not publish sender profile: %w", err)
	}
	c.NostrProfilePublished = true
	return c, s.save(c)
}

func (s Sender) Telegram(ctx context.Context, c Config, message string) error {
	if !c.TelegramEnabled {
		return nil
	}
	if c.TelegramToken == "" || c.TelegramChatID == "" {
		return errors.New("Telegram credentials missing")
	}
	b, _ := json.Marshal(map[string]string{
		"chat_id":    c.TelegramChatID,
		"text":       telegramMarkdownV2(message),
		"parse_mode": "MarkdownV2",
	})
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

func telegramMarkdownV2(message string) string {
	var formatted strings.Builder
	formatted.Grow(len(message) + len(message)/8)
	bold := false
	code := false
	for index := 0; index < len(message); {
		if !code && strings.HasPrefix(message[index:], "**") {
			formatted.WriteByte('*')
			bold = !bold
			index += 2
			continue
		}
		if message[index] == '\\' && index+1 < len(message) {
			formatted.WriteByte('\\')
			formatted.WriteByte(message[index+1])
			index += 2
			continue
		}
		character := message[index]
		if character == '`' {
			formatted.WriteByte(character)
			code = !code
			index++
			continue
		}
		if code {
			formatted.WriteByte(character)
			index++
			continue
		}
		if strings.ContainsRune("_*[]()~`>#+-=|{}.!", rune(character)) {
			formatted.WriteByte('\\')
		}
		formatted.WriteByte(character)
		index++
	}
	if bold || code {
		// Treat an unmatched CommonMark marker as literal input.
		return strings.NewReplacer("*", "\\*", "`", "\\`").Replace(formatted.String())
	}
	return formatted.String()
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
	toUs, toThem, e := nip17.PrepareMessage(ctx, message, nil, kr, rv.(string), nil)
	if e != nil {
		return fmt.Errorf("prepare NIP-17 message: %w", e)
	}
	if e = publishToAny(ctx, pool, c.NostrRelays, toUs, kr); e != nil {
		return fmt.Errorf("store sender copy: %w", e)
	}
	if e = publishToAny(ctx, pool, their, toThem, kr); e != nil {
		return fmt.Errorf("deliver to recipient: %w", e)
	}
	return nil
}

type relayResult struct {
	url string
	err error
}

func publishToAny(ctx context.Context, pool *nostr.SimplePool, relayURLs []string, event nostr.Event, signer nostr.Keyer) error {
	urls := uniqueRelayURLs(relayURLs)
	if len(urls) == 0 {
		return errors.New("no relays configured")
	}

	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan relayResult, len(urls))
	for _, relayURL := range urls {
		go func() {
			results <- relayResult{url: relayURL, err: publishWithAuth(attemptCtx, pool, relayURL, event, signer)}
		}()
	}

	failures := make([]string, 0, len(urls))
	for range urls {
		result := <-results
		if result.err == nil {
			cancel()
			return nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", displayRelayURL(result.url), result.err))
	}
	sort.Strings(failures)
	return fmt.Errorf("no relay accepted the event (%s)", strings.Join(failures, "; "))
}

func publishWithAuth(ctx context.Context, pool *nostr.SimplePool, relayURL string, event nostr.Event, signer nostr.Keyer) error {
	relay, err := pool.EnsureRelay(relayURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	if err = relay.Publish(ctx, event); !isAuthRequired(err) {
		return err
	}
	if err = relay.Auth(ctx, func(authEvent *nostr.Event) error {
		return signer.SignEvent(ctx, authEvent)
	}); err != nil {
		return fmt.Errorf("NIP-42 authentication: %w", err)
	}
	if err = relay.Publish(ctx, event); err != nil {
		return fmt.Errorf("publish after NIP-42 authentication: %w", err)
	}
	return nil
}

func isAuthRequired(err error) bool {
	if err == nil {
		return false
	}
	reason := strings.TrimSpace(err.Error())
	reason = strings.TrimPrefix(reason, "msg: ")
	return strings.HasPrefix(reason, "auth-required:")
}

func uniqueRelayURLs(relayURLs []string) []string {
	seen := make(map[string]struct{}, len(relayURLs))
	unique := make([]string, 0, len(relayURLs))
	for _, relayURL := range relayURLs {
		relayURL = strings.TrimSpace(relayURL)
		if relayURL == "" {
			continue
		}
		if _, exists := seen[relayURL]; exists {
			continue
		}
		seen[relayURL] = struct{}{}
		unique = append(unique, relayURL)
	}
	return unique
}

func displayRelayURL(relayURL string) string {
	parsed, err := url.Parse(relayURL)
	if err != nil {
		return relayURL
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func (s Sender) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}
