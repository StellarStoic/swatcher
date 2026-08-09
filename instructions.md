# s/watcher

s/watcher privately monitors Bitcoin addresses for incoming and outgoing
transactions using your own StartOS node, with optional notifications via
NIP-17 encrypted Nostr messages or Telegram.

Copyleft 2026 StellarStoic. This is free software: everyone may use, study,
review, copy, modify, and redistribute it under the GNU Affero General Public
License, version 3 or (at your option) any later version. Modified and
redistributed versions must preserve the license's source-sharing freedoms,
including for users who interact with the software over a network. s/watcher
is provided without warranty. The license and corresponding source code are
available at <https://github.com/StellarStoic/swatcher>.

## Set the web password

Before opening the Web Interface, run **Set Web Password** from the s/watcher
service actions. Enter the same password twice; it must contain at least 5
characters. s/watcher stores only an Argon2id hash. Changing the password later
signs out every existing browser session.

If you forget the password, select **Forgot password?** on the login page. Open
s/watcher in StartOS, select **Actions**, and run **Set Web Password**. Setting
a new password does not remove watches or change notification configuration.

Open the **Web UI**, enter a label and one of the following, then select
**Add watch**:

- A Bitcoin mainnet address
- A bulk list containing 2–10,000 unique Bitcoin mainnet addresses
- An account-level `xpub`, `ypub`, or `zpub`
- A `pkh()`, `wpkh()`, `sh(wpkh())`, or `tr()` output descriptor

Use **Find address** to check whether a Bitcoin mainnet address is already part
of any saved watch. The search reads only s/watcher's persisted local address
records; it makes no Electrs, Mempool, block-explorer, or other network request.
It covers individual and bulk addresses plus every xpub or descriptor child
that s/watcher has already derived and stored. A match shows its wallet and
group, derivation path, address type, source type, address-level balance, scan
state, known transaction count, latest transaction, and note. Privacy Mode
hides balances, notes, and transaction IDs. **Not found** means the address is
outside current local coverage; it does not prove that the address cannot
belong to an xpub beyond the configured discovery gap.

For an address that is not found, select **Check address in local Mempool** to
inspect its blockchain activity through your own StartOS Mempool service. The
link prefers Mempool's onion interface and otherwise uses its private LAN
interface. It never opens a public explorer. The link is shown only while the
optional Mempool dependency is available.

For a plain extended key, choose whether to include the `/1` change branch. A
bare xpub does not identify its address type, so choose Legacy, Nested SegWit,
Native SegWit, or Taproot when the selector appears. ypub and zpub formats
identify their address type automatically; ordinary addresses do not require
derivation settings.

For a list of existing addresses, select **Paste in bulk** and paste entries
separated by lines, spaces, commas, or semicolons. Repeated entries in the same
paste are removed. All addresses appear as one watch with one combined balance,
activity history, note, and notification rule. If any entry is invalid or
already watched, nothing is added. Bulk mode accepts mainnet addresses only,
not xpubs, other extended keys, descriptors, private keys, or seed phrases.
The initial StartOS-local Electrs scan may take time for very large lists.

To group watches that were already added, choose **Combine**. This reveals a
checkbox beside each watch and changes the button to **Combine selected**. That
button remains disabled until at least two rows are selected. Select **Cancel**
to leave selection mode and clear the current choices without refreshing the
page. Select between 2
and 100 rows, containing no more than 10,000 addresses in total, and choose
**Combine selected**. A row may represent one address or an entire xpub,
descriptor, bulk, or previously combined group. The confirmation dialog names
every multi-address group and asks whether it should be included.

Consolidation creates one fixed address collection and cannot automatically
restore the old grouping. Including an xpub or descriptor retains its currently
discovered addresses but stops future smart discovery for that source. Existing
history is merged and is never resent as new notifications. Leave the new note
empty to retain existing notes where possible. Matching notification rules are
inherited; differing rules disable notifications until the combined watch is
edited.
If selected watches contain the same address, s/watcher retains one canonical
address record, preserves known history and notes, and recalculates affected
transactions through local Electrs instead of counting the address twice.

The watch list shows a small linked mainnet type label such as P2PKH,
P2SH-P2WPKH, P2WPKH, P2WSH, or P2TR. Select **Edit** to change the address type
for a bare xpub. This replaces its derived addresses and creates a fresh
historical baseline, so older transactions are not sent as new notifications.
The type is intentionally not editable for ypubs, zpubs, descriptors, or
individual addresses because those inputs already determine it.

s/watcher discovers wallet addresses automatically. Open **Actions → General →
Smart Wallet Discovery** to set the number of consecutive unused addresses it
keeps beyond the highest used index. The default is 20 and the allowed range is
1–500. Larger gaps make more local Electrs queries. Reducing the setting never
deletes addresses already discovered, and newly derived historical addresses
are initialized without sending false activity notifications. The Web
Interface warns if the 500-address-per-branch safety limit prevents satisfying
the selected gap.

Descriptors must contain a public extended key and a non-hardened wildcard path
ending in `/*`. Use `<0;1>` to cover both receive and change branches in one
descriptor, for example `wpkh(xpub.../<0;1>/*)`. Descriptor paths override the
form's address-type and change settings.

If an address or any derived wallet address is already covered by an existing
watch, s/watcher adds nothing and identifies the conflicting watch in a modal.
This prevents duplicated activity notifications.

Names and groups may contain letters, numbers, spaces, and underscores. They
are converted to lowercase when saved, and repeated whitespace is collapsed.
An optional wallet note can contain up to 500 characters of plain-text context
and may use punctuation and multiple lines. Add it with a new watch or select
**Edit** to change it. Never enter a Bitcoin private key or seed phrase;
s/watcher rejects common secret-key and seed-phrase shapes. Privacy Mode masks
notes, and StartOS backups include them with the rest of `/data/state.json`.

Open **Actions → General → Theme** to choose Bitcoin Night, Cypherpunk
Neon, Arctic Node, Forest Ledger, or Paper Ledger. Colored swatches beside each
name preview its palette. The saved theme applies to the login screen and Web
Interface after the service restarts and persists through StartOS backups.
Name and group tags are assigned readable light colors automatically. Matching
text uses the same color regardless of capitalization.
Previously saved valid groups appear as suggestions in the group field. In the
watch list, select **Edit** to reveal the name and group inputs, **Save** to
apply them, or **Cancel** to discard the unsaved values. Use **Sort by** to
order watches by stack size, name, group, date added, latest change, or type.

The same **Edit** panel controls notifications for that wallet. Choose every
transaction, incoming only, outgoing only, or notifications off; set a minimum
amount in satoshis; and choose mempool, 1, 3, or 6 confirmations. The default
is every transaction immediately in the mempool. Confirmation-delayed alerts
remain pending across service restarts.

In the **Notifications** action, enable quiet hours to defer immediate alerts
between the selected start and end hours, or enable **Daily digest** to combine
pending activity into one message per day. Enter the whole-hour offset from UTC
for your local time. Quiet hours may cross midnight. A daily digest is sent at
or after its selected local hour and is tracked separately for Telegram and
Nostr so restarts do not duplicate it.

Open **Actions → General → Privacy Indicators** to control informational
address-reuse, small-deposit, and combined-wallet badges. The small-deposit
threshold defaults to 1,000 sat and is descriptive rather than a claim that an
output is technically uneconomical to spend. Indicators use only local Electrs
data and do not send notifications. Address-reuse counts cover receipts
observed after this feature is installed.

The colored rail on the left of each watch summarizes its latest activity:
gray means no movement or no net change, green means sats were added, and red
means sats were drained.

Each watch also shows the latest transaction found in its complete local
Electrs history and how long ago it occurred. Confirmed transactions use their
block-header time; unconfirmed transactions use the time s/watcher first sees
them. Privacy Mode masks this transaction ID.

Select **Show all transactions** below the latest transaction to open that
watch group's complete history returned by your local Electrs service,
including transactions from before the watch was added. Historical imports do
not trigger notifications. The page shows 100 transactions at a time with
previous and next navigation. Sort by newest or oldest, largest or smallest
value, incoming or outgoing activity, or mempool and confirmed state. Each
entry includes amounts, external input addresses for incoming transactions,
watched input addresses and external destination addresses for outgoing
transactions, transaction state and time,
the local Mempool link when available, and any RBF, Runes, inscription,
privacy-indicator, or OP_RETURN details already detected by s/watcher. Privacy
Mode masks amounts, transaction IDs, and addresses on this page too.
If Electrs returns a transaction ID but its full inputs cannot yet be decoded,
the transaction remains visible as **Details pending**. s/watcher retries it on
later scans and does not send an incomplete notification.

For incoming transactions, **From input address** is obtained from the previous
output spent by each transaction input. Several input addresses may appear,
and they do not prove a real-world sender identity. Inputs whose scripts cannot
be represented as standard Bitcoin addresses are omitted. Existing transaction
history is enriched automatically after upgrading.

If the optional Mempool dependency is installed and running, visible
transaction IDs in the watch list and New activity table are links to your own
Mempool service. When s/watcher is opened through Tor, it selects Mempool's
onion interface; from LAN it selects a private local interface. It never sends
the transaction ID to a public block explorer. Privacy Mode removes these links
while transaction identifiers are masked.

When newly detected activity contains a human-readable UTF-8 message in an
OP_RETURN output, it appears beneath the transaction ID on a light gray label.
Binary OP_RETURN protocol data is not displayed.

New activity also shows a badge when its raw transaction contains a Runes
runestone marker or one or more Ordinals inscription envelopes. Detection uses
only your StartOS-local Electrs transaction data. s/watcher does not decode,
render, fetch, or link the protocol content.

An incoming unconfirmed transaction that explicitly signals Replace-by-Fee
shows an amber **Replaceable — do not treat as final until confirmed.** badge.
The warning disappears after Electrs reports a block confirmation.

The Web UI supports both StartOS LAN and Tor addresses. Same-origin actions
remain accepted when the StartOS proxy presents a different internal hostname.

s/watcher checks your local Electrs service for confirmed and unconfirmed
activity. Transactions touching multiple addresses in an imported wallet are
combined into one event with exact received, sent, or self-transfer amounts.
Mempool events update when they become confirmed.

The first successful check establishes the initial state. Later transactions
are recorded as activity. Removing a watch also removes its locally stored
activity.

Open the StartOS **Privacy Mode** action under **General** to control persistent
masking. Set the Web Password before enabling privacy mode. Disabling privacy
mode requires entering that password. Privacy mode replaces balances and
activity amounts with randomized Unicode Symbols for Legacy Computing and masks
addresses, extended keys, descriptors, transaction IDs, and the displayed Nostr
npub except for their first and last four characters. The masking is performed before the HTML is
rendered. This setting affects the Web Interface only; Telegram and Nostr notification
amounts remain visible.

Never enter a seed phrase or private key. s/watcher accepts only public data and
cannot spend funds. An xpub or descriptor reveals the wallet's complete public
transaction graph, so protect access to the service and its backups.

## Notifications

Open the StartOS **Notifications** action to configure either channel:

- Telegram requires a BotFather bot token and recipient ID.
- Nostr requires one or more `wss://` discovery relays and the receiver npub.
  All private messages use NIP-17 gift wrapping. The receiver must publish a
  kind 10050 DM relay list discoverable from the configured relays.

**Recipient npub** means your Nostr public key. Never enter your `nsec`: it is
your secret private key. s/watcher rejects an nsec without saving it and asks
for the corresponding npub. The sender nsec and npub fields explain that the
generated values appear after Notifications is saved with Nostr enabled.

The Nostr relay field is prefilled with:

- `wss://relay.damus.io`
- `wss://nos.lol`
- `wss://auth.nostr1.com`
- `wss://relay.ditto.pub`

You may edit this list. Leaving it empty restores these defaults. If Nostr test
delivery still reports that the recipient has no kind 10050 relay list, open
the receiving Nostr client and configure/publish its private-message relays;
ordinary profile or outbox relays do not replace the NIP-17 kind 10050 list.
Relays that request NIP-42 authentication are authenticated with s/watcher's
dedicated sender identity. All recipient relays are attempted concurrently,
and a failed test identifies the reason returned by each relay.

Tor is an optional s/watcher dependency. When StartOS Tor is installed and
running, `.onion` relay connections use its internal SOCKS proxy while clearnet
Nostr and Telegram connections remain direct. Tor is required when every relay
in the recipient's kind 10050 list is an onion address.

### Telegram personal notifications

1. Create the bot with `@BotFather` and copy its token.
2. Open the new bot's private chat and press **Start**, or send `/start`. A bot
   cannot initiate a conversation until you do this once.
3. On a trusted computer with `curl` and `jq`, run the commands below. Paste
   the BotFather token only at the hidden prompt; do not put it directly in the
   command or share the command output.

   ```sh
   read -rsp "Telegram bot token: " SWATCHER_TG_TOKEN; echo
   curl --silent --show-error "https://api.telegram.org/bot${SWATCHER_TG_TOKEN}/getUpdates" \
     | jq -r '.result[] | select(.message.chat.type == "private") | "\(.message.from.username // .message.from.first_name)\t\(.message.chat.id)"'
   unset SWATCHER_TG_TOKEN
   ```

4. Copy the numeric ID shown beside your name into **Telegram recipient ID** in
   the StartOS **Notifications** action. For a direct bot conversation, this is
   your Telegram user ID and private-chat ID.

If nothing is returned, send `/start` to the bot again and repeat the lookup.
Groups are optional: to notify several people, add the bot to a group, send
`/start@your_bot_username`, and use that update's negative group chat ID instead.
Avoid third-party “ID finder” bots. The bot token is a secret: revoke it with
`@BotFather` if it is ever exposed.

When Nostr is first enabled, the action immediately generates and persists a
dedicated nsec/npub. Both keys appear as disabled fields after the action saves;
the nsec is masked and neither key can be changed. The private sender key is
generated randomly and stored internally. The default sender name
is a unique `swatcher-xxxxxx` name and can be changed. Disabling Nostr preserves
the identity. A DiceBear Pixelbot avatar is generated from the npub and rendered
in the s/watcher Web UI; its URL is not shown as a form field. The name and
avatar are published as the sender's Nostr profile without waiting for the
first Bitcoin alert. Open the s/watcher Web Interface to see the generated
Pixelbot avatar; StartOS 0.4 action forms cannot display an inline image in the
Notifications settings.

Successful delivery is recorded separately for each channel. Failed deliveries
are retried during later polling cycles without duplicating successful ones.
Saving the **Notifications** action normally only stores the configuration. To
verify a channel, enable and save it, reopen Notifications, select that
channel's **Send test message after save** switch, and save once more. It sends:
“You receive this message because you enabled Notifications in s/watcher.
Consider this a test message.” The switch resets to off, delivery errors are
shown immediately, and tests are not queued for automatic retry. Restarting
s/watcher does not send another test.

Activity notifications contain the watch name and group, incoming/outgoing or
self-transfer direction, received and sent amounts, net change, confirmation
state, current confirmed and pending balance, detection time, and the plain
transaction ID. They also include an RBF warning for replaceable incoming
mempool transactions, Runes and inscription-envelope detections, and printable
OP_RETURN messages when present. Up to five OP_RETURN messages are included in
one alert. Telegram and Nostr messages use Markdown with a bold heading and
field labels, with each tracked detail on its own line.
The Web Interface and notifications show amounts at or above 1,000,000 sat in
BTC; smaller amounts remain in sat.

When StartOS exposes an onion address for the optional local Mempool service,
the transaction line becomes that local `/tx/` URL so Telegram and compatible
Nostr clients can open it. If no Mempool onion interface is available, the
transaction remains a non-linkable inline-code ID that is easy to copy in
Telegram. s/watcher never substitutes a clearnet explorer. Daily digests
contain the same details and summarize omitted events
before reaching Telegram's message-size limit.
The service logs every Telegram and NIP-17 send attempt and whether it
succeeded or failed, identifying activity, digest, and test messages. Logs do
not include credentials, recipients, wallet names, addresses, transaction IDs,
or message content.
