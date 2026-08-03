# s/watcher

`s/watcher` is a small, self-hosted Bitcoin watch-only activity monitor for
StartOS 0.4. It talks only to the Electrs service installed on the same StartOS
server.

Copyright (C) 2026 StellarStoic.

Licensed under the GNU Affero General Public License, version 3 or (at your
option) any later version. s/watcher is provided without warranty. See
[`LICENSE`](LICENSE) for the complete terms. Users interacting with s/watcher
over a network can obtain its corresponding source from the
[s/watcher repository](https://github.com/StellarStoic/swatcher).

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
- Keep watch names and groups as read-only summaries until explicitly edited,
  suggest existing groups, and sort watches by six useful criteria.
- Show a gray, green, or red activity rail on each watch row based on its latest
  detected net movement.
- Update mempool events when they receive a block confirmation.
- Show balance and newly detected activity in a local web UI.
- Persist watches and events in `/data/state.json`.
- Deliver retryable Telegram Bot API alerts and NIP-17 private Nostr messages.
- Persist per-channel delivery state so restarts do not duplicate alerts.
- Generate and retain a dedicated Nostr sender identity under `/data`, with a
  deterministic DiceBear avatar and configurable sender name.
- Require an Argon2id-protected password for the Web Interface and provide a
  server-rendered privacy mode for balances and identifiers.

No private keys are accepted or stored. Public extended keys reveal an entire
wallet's transaction graph, so the state file and UI should still be treated as
private information.

A plain `xpub` does not encode an address type, so the Web UI asks for one only
when a bare xpub is entered. `ypub` and `zpub` imports infer nested and native
SegWit respectively, while descriptors are authoritative. Duplicate addresses
and overlapping wallet imports are rejected without changing the watch list and
explained in an in-interface modal. Hardened derivation below an xpub is
rejected because it is not mathematically possible.

Watch names and groups accept letters, numbers, spaces, and underscores. Input
is normalized to lowercase when saved, and repeated whitespace is collapsed.
Wallet-name and group tags receive automatic light colors. Color assignment is
deterministic and case-insensitive, so the same normalized text always uses the
same color across watches and page reloads.
Existing valid group names appear as suggestions when adding or editing a
watch. The watch list can be sorted by stack size, name, group, date
added, latest detected activity, or address type. Editing is opt-in per row;
Cancel restores the saved values without a request.

Edit inputs are hidden until **Edit** is selected. Changing the sort order
returns the viewport to the first watch row rather than the top of the page.

Each watch row has a slim activity rail: gray means no detected movement or a
net-neutral transaction, green means the latest transaction added sats, and
red means it drained sats. The rail reflects the latest detected event for the
whole watch group.

## Architecture

```text
browser -> s/watcher:8080 -> Electrs:50001 -> Bitcoin
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
is required. The actual image is rendered in the s/watcher Web UI instead of
showing its URL as a configuration field. StartOS 0.4 action forms do not
provide an inline image field, so the Notifications action cannot render the
avatar between its text inputs.

Saving Notifications normally only persists the settings. After a channel has
been enabled and saved, its section exposes a one-shot **Send test message after
save** switch. Selecting it sends during that submission and reports the real
delivery error in the Notifications action. The switch resets to off whenever
the form is reopened; service restarts never send tests automatically.

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

The recipient field accepts and validates only an `npub` public key. An
accidentally pasted `nsec` is rejected before configuration is written and a
security warning explains why secret keys must never be pasted into websites.
Empty generated sender-key fields show that their values will appear after
saving with Nostr enabled.

## Web password and privacy mode

Run the StartOS **Set Web Password** action before opening the Web Interface.
The password must contain at least five characters. Only an Argon2id salt and
hash are stored in `/data/auth.json`; changing the
password rotates the session secret and signs out every browser. Sessions use
HttpOnly, SameSite=Strict cookies, expire after 12 hours, and are temporarily
rate-limited after repeated failures. The StartOS health endpoint remains
available without authentication.

If the password is forgotten, **Forgot password?** on the login page explains
how to run **Set Web Password** from the authenticated StartOS service actions.
This replaces the password without changing watches or notification settings;
notifications are not required for account recovery.

The StartOS **Privacy Mode** action controls persistent masking in the Web
Interface. A Web Password must be configured before privacy mode can be
enabled, and disabling it requires that password. Balances and activity amounts
are replaced server-side with randomized Unicode Symbols for Legacy Computing.
Bitcoin addresses, extended keys,
descriptors, transaction IDs, and the displayed Nostr npub retain only their
first and last four characters. The unmasked values are not included in the
rendered HTML. Notification messages are not altered by privacy mode.

The StartOS backup includes the complete `main` volume: `state.json`,
`notifications.json`, and `auth.json`. Restores therefore retain watches,
notification credentials and sender keys, the web-password hash, privacy mode,
and session-signing state.
