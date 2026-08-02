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

Plain `xpub` imports default to native SegWit unless another address type is
selected. `ypub` and `zpub` imports infer nested and native SegWit respectively.
Descriptors are authoritative and ignore the form's address-type and change
branch settings. Hardened derivation below an xpub is rejected because it is
not mathematically possible.

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
replacing that nsec later. Disabling Nostr does not delete its sender identity.
Generation and public-key derivation happen immediately when the action is
saved; the npub is displayed while the persisted nsec remains masked. A stable
randomized default name such as `swatcher-k7m2qd` is generated and published
with the DiceBear avatar as a kind 0 profile before any alert is required. The
generated avatar URL is visible as a read-only field in the Notifications
action.

The recipient ID can be read directly from the Bot API `getUpdates` response;
the complete token-safe command is documented in `instructions.md`. Telegram
groups remain an optional destination and use their negative chat ID.

Nostr delivery is NIP-17 only: kind 14 rumors are NIP-44 sealed and NIP-59
gift-wrapped as kind 1059. Recipient delivery uses the recipient's kind 10050
relay list discovered through the configured relays. A kind 0 sender profile
containing the configured name and selected DiceBear avatar is published to the
configured relays.
