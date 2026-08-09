# s/watcher

`s/watcher` privately monitors Bitcoin addresses for any incoming and outgoing
transactions using your own StartOS node, with optional notifications via
NIP-17 encrypted Nostr messages or Telegram. It talks only to the Electrs
service installed on the same StartOS server.

Copyleft 2026 StellarStoic. This is free software: everyone may use, study,
review, copy, modify, and redistribute it under the GNU Affero General Public
License, version 3 or (at your option) any later version. Modified and
redistributed versions must preserve the license's source-sharing freedoms,
including for users who interact with the software over a network. s/watcher
is provided without warranty. See [`LICENSE`](LICENSE) for the complete terms
and the [s/watcher repository](https://github.com/StellarStoic/swatcher) for
corresponding source code.

## Current milestone

- Add and remove grouped mainnet Bitcoin addresses, xpubs, ypubs, zpubs, and
  supported output descriptors.
- Paste as many as 10,000 unique mainnet addresses into one logical watch group
  with one combined balance, activity history, note, and notification policy.
- Discover wallet addresses automatically with a configurable 1–500 address
  gap, defaulting to 20 consecutive unused addresses.
- Optionally monitor both the `/0` receive and `/1` change branches.
- Support `pkh()`, `wpkh()`, `sh(wpkh())`, and `tr()` descriptor forms with
  non-hardened paths ending in `/*`, including `<0;1>` branch expressions.
- Convert addresses locally to Electrum script hashes.
- Poll Electrs for confirmed and mempool history.
- Resolve transaction inputs and outputs through Electrs.
- Decode and display printable UTF-8 messages from OP_RETURN transaction
  outputs while ignoring binary payloads.
- Consolidate activity across every derived address in a watch, classify new
  transactions as received, sent, or self-transfer, and show exact amounts.
- Keep watch names and groups as read-only summaries until explicitly edited,
  attach private notes, suggest existing groups, and sort watches by six useful
  criteria.
- Show a gray, green, or red activity rail on each watch row based on its latest
  detected net movement.
- Update mempool events when they receive a block confirmation.
- Mark explicitly replaceable incoming mempool transactions with an amber
  BIP125 warning until they confirm.
- Show balance and newly detected activity in a local web UI.
- Display amounts below 1,000,000 sat in sat and amounts at or above that
  threshold in BTC without floating-point rounding or unnecessary trailing
  zeros.
- Link visible transaction IDs to the optional StartOS-local Mempool service,
  matching Tor sessions to its onion interface and LAN sessions to its local
  interface.
- Persist watches and events in `/data/state.json`.
- Deliver retryable Telegram Bot API alerts and NIP-17 private Nostr messages.
- Include tracked transaction details in immediate alerts and daily digests:
  direction, received/sent/net amounts, confirmation or RBF state, current
  balance, detection time, Runes and inscription markers, printable OP_RETURN
  messages, and the transaction ID, formatted as readable Markdown with bold
  labels and one detail per line.
- Persist per-channel delivery state so restarts do not duplicate alerts.
- Configure each watched wallet for all, incoming-only, outgoing-only, or no
  alerts, with a minimum sat amount and mempool/1/3/6-confirmation timing.
- Generate and retain a dedicated Nostr sender identity under `/data`, with a
  deterministic DiceBear avatar and configurable sender name.
- Require an Argon2id-protected password for the Web Interface and provide a
  server-rendered privacy mode for balances and identifiers.
- Choose from five persistent Web Interface themes through a StartOS action,
  with color swatches beside every theme name.
- Find whether a Bitcoin address belongs to any saved watch using only the
  addresses already persisted by s/watcher, including xpub- and
  descriptor-derived children.

No private keys are accepted or stored. Public extended keys reveal an entire
wallet's transaction graph, so the state file and UI should still be treated as
private information.

## Find address

Use **Find address** in the Web Interface to check whether a Bitcoin mainnet
address belongs to any watch already saved by s/watcher. The lookup searches
`/data/state.json` only. It does not ask Electrs, Mempool, a public explorer, or
any other service and therefore cannot discover addresses beyond the coverage
s/watcher has already derived and stored.

A match reports the wallet name and group, derivation path when available,
address type, watch source type, address-level balance, scan state, last check,
known transaction count, latest transaction, and wallet note. This includes
children of xpubs and descriptors, bulk imports, and combined watches. Several
matches are displayed if legacy state contains the same script more than once.
Privacy Mode omits the note and transaction ID and hides the balance. An
address that is not found may still belong to a wallet beyond its currently
stored smart-discovery range.

When the optional local Mempool dependency is available, a **Check address in
local Mempool** link appears for addresses not found in saved watches. The link
prefers Mempool's StartOS onion address and falls back to its private LAN
address when no onion interface is available. It never links to a public
explorer. If Mempool is unavailable, s/watcher explains that instead of
offering an external link.

## Bulk address groups

Select **Paste in bulk** in the add-watch form to paste between 2 and 10,000
unique Bitcoin mainnet addresses. Entries may be separated by lines, spaces,
commas, or semicolons. Repeated addresses inside the same paste are removed.
The complete import is atomic: if an entry is invalid or any address is already
watched, s/watcher adds nothing and explains the problem without echoing the
pasted value.

A bulk import appears as one watch row and shares one name, group, note,
combined balance, activity history, and notification policy. It may contain a
mix of legacy, Script Hash, SegWit, and Taproot addresses. Bulk mode accepts
addresses only, not extended keys or descriptors. The addresses persist in
`/data/state.json` and are included in StartOS backups.

To keep large groups practical, polling uses at most eight concurrent
connections to StartOS-local Electrs and writes state once after each completed
scan cycle. Initializing thousands of addresses can still take time and does
not turn historical transactions into new alerts.

## Combining existing watches

Choose **Combine** to enter selection mode. The previously hidden checkboxes
then appear and the button becomes **Combine selected**, remaining disabled
until at least two rows are selected. Choose **Cancel** at any time to leave
selection mode, clear the checkboxes, and remain at the same scroll position.
Choose 2–100 existing watches and then
**Combine selected** to turn them into one fixed collection, up to 10,000 total
addresses. This is useful when addresses were originally added one at a time.
The dialog requests a new name and group and explicitly names every selected
multi-address group before consolidation.

Combining an xpub or descriptor group retains every address discovered so far
but stops smart discovery for that source because the result is a fixed address
collection. The operation cannot reconstruct the original grouping
automatically. Existing transactions touching multiple selected watches are
merged, and all existing history is suppressed from notification delivery so
the operation cannot generate old alerts.
If selected watches overlap on the same Bitcoin script, Combine retains one
canonical address record, unions its known history, and asks StartOS-local
Electrs to recalculate affected transactions once. This repairs legacy
overlaps without doubling balances or activity amounts.

Leave the new note empty to retain existing notes where possible. If every
selected watch has the same notification rule, that rule is inherited. If the
rules differ, notifications are disabled on the new watch until reviewed with
**Edit**.

## Smart wallet discovery

Extended public keys and ranged descriptors use smart discovery instead of a
fixed derivation count. s/watcher initially derives the configured gap on each
selected branch. After Electrs reports history, it keeps deriving until that
many consecutive unused indexes remain beyond the highest used index. Newly
derived addresses are baselined before notifications begin, preventing old
wallet history from being reported as new activity.

Set **Address discovery gap** under **Actions → General → Smart Wallet
Discovery**. The default is 20 and the accepted range is 1–500. Increasing it
can find wallets that skipped more indexes but performs more local Electrs
queries. Decreasing it never deletes addresses already discovered. A maximum
of 500 addresses per branch bounds resource usage. Plain addresses are not
affected. If activity near the ceiling prevents satisfying the selected gap,
the Web Interface shows a warning on that watch.

## Per-wallet notification rules

Select **Edit** on a watched wallet to choose which activity produces Telegram
or NIP-17 alerts. Rules support every transaction, incoming only, outgoing
only, or disabled; a minimum sat amount; and delivery in the mempool or after
1, 3, or 6 confirmations. The default remains every transaction in the
mempool. Waiting and delivery state persists in `/data/state.json`, preventing
duplicate alerts after a restart.

## Wallet notes

Each wallet or address can carry an optional 500-character note for context
that does not fit its short name or group. Add the note with a new watch or
select **Edit** to change it. Notes support punctuation and multiple lines,
remain escaped as plain text in the Web Interface, persist in
`/data/state.json`, and are included in StartOS backups. Privacy Mode masks
notes along with balances and wallet identifiers.

Never enter a Bitcoin private key or seed phrase. s/watcher rejects common
extended-private-key, WIF, and seed-phrase shapes as an additional safeguard.

## Themes

Open **Actions → General → Theme** to select Bitcoin Night, Cypherpunk
Neon, Arctic Node, Forest Ledger, or Paper Ledger. Colored swatches beside the
option names preview each palette. The choice is stored in `/data/state.json`,
applies to both the login screen and dashboard after the automatic restart,
and is included in StartOS backups. Existing installations default to Bitcoin
Night.

## Notification schedule

The **Notifications** action can keep immediate delivery, optionally queue it
during quiet hours, or combine eligible activity into one daily digest. Hours
use the configured whole-hour UTC offset. Quiet hours may cross midnight;
queued immediate alerts are sent after the quiet period ends. Daily digests are
sent once per local date at or after the selected hour, include up to ten
activity details plus a remaining count, and persist independent Telegram and
Nostr delivery dates in `/data/state.json`.

## Privacy indicators

The **Privacy Indicators** action controls informational badges for repeated
receipts to the same observed address, incoming amounts below a configurable
threshold (1,000 sat by default), and transactions involving more than one
watched wallet. These observations use only locally fetched transaction data.
They do not trigger notifications. Address-reuse counts begin with activity
recorded by a version that supports the indicator; existing historical events
without received-address metadata are not guessed.

## Runes and Ordinals detection

For newly detected activity, s/watcher marks transactions containing the Runes
runestone output marker or Ordinals inscription envelopes in transaction
witnesses. Detection uses raw transaction data fetched from the StartOS-local
Electrs service. s/watcher does not decode, render, fetch, or link protocol
content.

A plain `xpub` does not encode an address type, so the Web UI asks for one only
when a bare xpub is entered. `ypub` and `zpub` imports infer nested and native
SegWit respectively, while descriptors are authoritative. Duplicate addresses
and overlapping wallet imports are rejected without changing the watch list and
explained in an in-interface modal. Hardened derivation below an xpub is
rejected because it is not mathematically possible.

Each watch shows a compact, linked mainnet address-type description: P2PKH,
P2SH, P2SH-P2WPKH, P2WPKH, P2WSH, or P2TR as appropriate. The **Edit** form can
change the derivation type of a bare xpub. Because that selection produces a
different set of addresses, s/watcher replaces the old derived coverage and
establishes a fresh historical baseline; it does not emit old transactions as
new notifications. Address types encoded by ypubs, zpubs, descriptors, and
individual addresses cannot be changed in the editor.

Watch names and groups accept letters, numbers, spaces, and underscores. Input
is normalized to lowercase when saved, and repeated whitespace is collapsed.
Wallet-name and group tags receive automatic light colors. Color assignment is
deterministic and case-insensitive, so the same normalized text always uses the
same color across watches and page reloads.
Existing valid group names appear as suggestions when adding or editing a
watch. The watch list can be sorted by stack size, name, group, date
added, latest detected activity, or address type. Editing is opt-in per row;
Cancel restores the saved values without a request.

POST requests use browser Fetch Metadata for cross-site protection so StartOS
Tor and LAN proxy hostnames can differ from the application's internal host
without blocking legitimate same-origin actions.

Edit inputs are hidden until **Edit** is selected. Changing the sort order
returns the viewport to the first watch row rather than the top of the page.

Each watch row has a slim activity rail: gray means no detected movement or a
net-neutral transaction, green means the latest transaction added sats, and
red means it drained sats. The rail reflects the latest detected event for the
whole watch group.

The watch status also shows its most recent historical transaction and a
relative time such as **2 weeks ago**. Confirmed times come from the matching
block header fetched through StartOS-local Electrs; an unconfirmed transaction
uses the time s/watcher first observes it. Privacy Mode masks the displayed
transaction ID along with the other identifiers.

**Show all transactions** beneath that latest transaction opens a dedicated
history page for the watch group. s/watcher imports the complete history
returned by the StartOS-local Electrs service, including transactions that
predate adding the watch; imported history is never sent as a new notification.
The page includes received, sent, and net amounts, external input addresses for
incoming transactions, watched input addresses and external destination
addresses for outgoing transactions. Every displayed input and output address
has its exact transaction amount beside it; repeated appearances of the same
address are summed into one row. Values below one million sats are shown in
sats and larger values are shown in BTC.
confirmation or mempool state, RBF status, Runes and inscription markers,
OP_RETURN text, transaction time, and the local transaction link. It
sorts by newest or oldest time, largest or smallest value, incoming or outgoing
direction, and mempool or confirmed state. Results are paginated at 100
transactions per page, and Privacy Mode masks amounts, transaction IDs, and
addresses before the page is rendered. A transaction returned by Electrs is
listed immediately even if its inputs cannot yet be decoded; it is marked
**Details pending**, retried on later scans, and withheld from notifications
until its amounts and direction are known.

For incoming transactions, **From input address** is derived from the previous
output spent by each transaction input. A transaction can list several input
addresses, and these identify transaction inputs rather than proving the
real-world sender. Non-address input scripts cannot be displayed. Existing
history is enriched with per-address amounts automatically after this upgrade.

When the optional Mempool dependency is installed and running, visible
transaction IDs in both the watch status and activity table open that
transaction in the Mempool service on the same StartOS server. s/watcher uses
Mempool's onion interface when opened through Tor and its private LAN interface
otherwise; it never falls back to a public explorer. Transaction links are
disabled while Privacy Mode masks identifiers.

## Architecture

```text
browser -> s/watcher:8080 -> Electrs:50001 -> Bitcoin
     |            |
     |            +-> /data/state.json
     +-> optional local Mempool /tx/<txid>
```

The package resolves Electrs through the StartOS 0.4 LXC bridge using package
id `electrs`, host id `electrum`, and internal port `50001`.
Mempool is an optional StartOS dependency used only to discover its private Web
UI addresses; transaction data still comes exclusively from local Electrs.

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

Bitcoin activity alerts include the watch name and group, direction,
received/sent/net amounts, confirmation state, current confirmed and pending
balance, detection time, RBF status for replaceable incoming mempool
transactions, Runes and inscription-envelope detections, and every printable
OP_RETURN message retained by s/watcher (up to five per alert). If StartOS
provides an onion interface for the optional local Mempool dependency, the
transaction line uses its `/tx/` URL and is clickable in clients that recognize
links. Without that onion interface, the transaction ID is formatted as an
inline-code string so Telegram users can copy it easily; s/watcher never
inserts a clearnet explorer link. Daily digests use the same
details and stop before Telegram's message-size limit, reporting how many
additional activities remain. Telegram and Nostr messages use Markdown with a
bold heading and field labels, with each tracked detail on its own line. Wallet
names and transaction content are escaped before formatting so they cannot
alter the message markup.
StartOS logs record a privacy-safe started, succeeded, or failed entry for each
Telegram or NIP-17 activity alert, daily digest, and requested test message.
These delivery logs never include credentials, recipients, wallet metadata,
addresses, transaction IDs, or message bodies.
Amounts at or above 1,000,000 sat are shown in BTC in both the Web Interface and
notifications; smaller amounts remain in sat.

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
configured relays. Relays that require NIP-42 are authenticated with the
dedicated sender identity before delivery is retried. Recipient relays are
attempted concurrently, and failed tests report the reason returned by each
relay.

Tor is an optional StartOS dependency. When it is installed and running,
s/watcher routes only `.onion` Nostr relay connections through Tor's internal
SOCKS bridge; clearnet Nostr relays and Telegram remain direct. If the
recipient's kind 10050 list contains only onion relays, Tor must be available
for notifications and the Notifications test action to succeed.

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
selected theme, wallet notes, and session-signing state.
