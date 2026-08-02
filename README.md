# s-watcher

`s-watcher` is a small, self-hosted Bitcoin watch-only activity monitor for
StartOS 0.4. It talks only to the Electrs service installed on the same StartOS
server.

Licensed under the GNU Affero General Public License v3.0 only.

## Current milestone

- Add and remove grouped mainnet Bitcoin addresses, xpubs, ypubs, zpubs, and
  supported output descriptors.
- Derive a user-selected 1–500 addresses per branch from extended public keys.
- Optionally monitor both the `/0` receive and `/1` change branches.
- Support `pkh()`, `wpkh()`, `sh(wpkh())`, and `tr()` descriptor forms with
  non-hardened paths ending in `/*`, including `<0;1>` branch expressions.
- Convert addresses locally to Electrum script hashes.
- Poll Electrs for confirmed and mempool history.
- Resolve transaction inputs and outputs through Electrs.
- Consolidate activity across every derived address in a watch, classify new
  transactions as received, sent, or self-transfer, and show exact amounts.
- Update mempool events when they receive a block confirmation.
- Show balance and newly detected activity in a local web UI.
- Persist watches and events in `/data/state.json`.
- Deliver retryable Telegram Bot API alerts and NIP-17 private Nostr messages.
- Persist per-channel delivery state so restarts do not duplicate alerts.
- Generate and retain a dedicated Nostr sender identity under `/data`, with a
  deterministic DiceBear avatar and configurable sender name.

No private keys are accepted or stored. Public extended keys reveal an entire
wallet's transaction graph, so the state file and UI should still be treated as
private information.

A plain `xpub` does not encode an address type, so the Web UI asks for one only
when a bare xpub is entered. `ypub` and `zpub` imports infer nested and native
SegWit respectively, while descriptors are authoritative. Duplicate addresses
and overlapping wallet imports are rejected without changing the watch list and
explained in an in-interface modal. Hardened derivation below an xpub is
rejected because it is not mathematically possible.

## Architecture

```text
browser -> s-watcher:8080 -> Electrs:50001 -> Bitcoin
                  |
                  +-> /data/state.json
```

The package resolves Electrs through the StartOS 0.4 LXC bridge using package
id `electrs`, host id `electrum`, and internal port `50001`.

## Development

```sh
go test ./...
go run ./cmd/s-watcher
```

Local environment variables:

- `SWATCHER_LISTEN` (default `:8080`)
- `SWATCHER_DATA` (default `./data`)
- `ELECTRUM_ADDR` (default `127.0.0.1:50001`)
- `SWATCHER_POLL_INTERVAL` (default `30s`)

Build the StartOS package with `make` after installing the StartOS packaging
toolchain and npm dependencies.

## Notifications

The StartOS **Notifications** action configures Telegram and Nostr. Telegram
personal alerts require a BotFather token and recipient user ID. The user must
press **Start** in the bot's private chat once before it can send alerts. Nostr
accepts discovery relays and a
recipient npub, generates a dedicated nsec/npub when first enabled, and permits
the service to retain that identity when disabled. The random private sender
key is shown masked in a disabled field; both key fields are read-only.
Generation and public-key derivation happen immediately when the action is
saved; the npub is displayed while the persisted nsec is not returned to the
UI. A stable randomized default name such as `swatcher-k7m2qd` is generated and
published with a DiceBear Pixelbot avatar as a kind 0 profile before any alert
is required. The actual image is rendered in the s-watcher Web UI instead of
showing its URL as a configuration field. StartOS 0.4 action forms do not
provide an inline image field, so the Notifications action cannot render the
avatar between its text inputs.

Saving Notifications only persists the settings. The separate **Test
Telegram** and **Test Nostr** actions send the test message on demand and report
delivery failures immediately; configuration saves and service restarts never
send test messages automatically.

At container startup, a minimal bootstrap step grants the unprivileged
`swatcher` process ownership of `/data`. The service then drops privileges
before starting, allowing it to atomically update files created by StartOS
actions without running the application as root.

The recipient ID can be read directly from the Bot API `getUpdates` response;
the complete token-safe command is documented in `instructions.md`. Telegram
groups remain an optional destination and use their negative chat ID.

Nostr discovery defaults to `wss://relay.damus.io`, `wss://nos.lol`,
`wss://auth.nostr1.com`, and `wss://relay.ditto.pub`. NIP-17 delivery still
requires the recipient to publish a kind 10050 private-message relay list.

Nostr delivery is NIP-17 only: kind 14 rumors are NIP-44 sealed and NIP-59
gift-wrapped as kind 1059. Recipient delivery uses the recipient's kind 10050
relay list discovered through the configured relays. A kind 0 sender profile
containing the configured name and selected DiceBear avatar is published to the
configured relays.
