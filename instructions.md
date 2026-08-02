# s-watcher

## Set the web password

Before opening the Web Interface, run **Set Web Password** from the s-watcher
service actions. Enter the same password twice; it must contain at least 12
characters. s-watcher stores only an Argon2id hash. Changing the password later
signs out every existing browser session.

If you forget the password, select **Forgot password?** on the login page. Open
s-watcher in StartOS, select **Actions**, and run **Set Web Password**. Setting
a new password does not remove watches or change notification configuration.

Open the **Web UI**, enter a label and one of the following, then select
**Add watch**:

- A Bitcoin mainnet address
- An account-level `xpub`, `ypub`, or `zpub`
- A `pkh()`, `wpkh()`, `sh(wpkh())`, or `tr()` output descriptor

For a plain extended key, select how many addresses to derive per branch and
whether to include the `/1` change branch. A bare xpub does not identify its
address type, so choose Legacy, Nested SegWit, Native SegWit, or Taproot when
the selector appears. ypub and zpub formats identify their address type
automatically; ordinary addresses do not require derivation settings.

Descriptors must contain a public extended key and a non-hardened wildcard path
ending in `/*`. Use `<0;1>` to cover both receive and change branches in one
descriptor, for example `wpkh(xpub.../<0;1>/*)`. Descriptor paths override the
form's address-type and change settings.

If an address or any derived wallet address is already covered by an existing
watch, s-watcher adds nothing and identifies the conflicting watch in a modal.
This prevents duplicated activity notifications.

Names and groups may contain only lowercase letters `a-z` and numbers `0-9`.
Previously saved valid groups appear as suggestions in the group field. In the
watch list, select **Edit** to reveal the name and group inputs, **Save** to
apply them, or **Cancel** to discard the unsaved values. Use **Sort by** to
order watches by stack size, name, group, date added, latest change, or type.

s-watcher checks your local Electrs service for confirmed and unconfirmed
activity. Transactions touching multiple addresses in an imported wallet are
combined into one event with exact received, sent, or self-transfer amounts.
Mempool events update when they become confirmed.

The first successful check establishes the initial state. Later transactions
are recorded as activity. Removing a watch also removes its locally stored
activity.

Use **Hide balances and identifiers** after signing in to enable privacy mode.
It replaces balances and activity amounts with randomized legacy-computing
symbols and masks addresses, extended keys, descriptors, transaction IDs, and
the displayed Nostr npub except for their first and last four characters. The
masking is performed before the HTML is rendered. Select **Show balances and
identifiers** to reveal them again. This setting affects the Web Interface only;
Telegram and Nostr notification amounts remain visible.

Never enter a seed phrase or private key. s-watcher accepts only public data and
cannot spend funds. An xpub or descriptor reveals the wallet's complete public
transaction graph, so protect access to the service and its backups.

## Notifications

Open the StartOS **Notifications** action to configure either channel:

- Telegram requires a BotFather bot token and recipient ID.
- Nostr requires one or more `wss://` discovery relays and the receiver npub.
  All private messages use NIP-17 gift wrapping. The receiver must publish a
  kind 10050 DM relay list discoverable from the configured relays.

**Recipient npub** means your Nostr public key. Never enter your `nsec`: it is
your secret private key. s-watcher rejects an nsec without saving it and asks
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
in the s-watcher Web UI; its URL is not shown as a form field. The name and
avatar are published as the sender's Nostr profile without waiting for the
first Bitcoin alert. Open the s-watcher Web Interface to see the generated
Pixelbot avatar; StartOS 0.4 action forms cannot display an inline image in the
Notifications settings.

Successful delivery is recorded separately for each channel. Failed deliveries
are retried during later polling cycles without duplicating successful ones.
Saving the **Notifications** action only stores the configuration. To verify a
channel, enable and save it first, then run its **Send Telegram test message**
or **Send Nostr test message** action. Disabled channels do not show a test
action. The selected action immediately sends: “You receive this message because you
enabled Notifications in s-watcher. Consider this a test message.” A delivery
error is shown by the action and is not queued for automatic retry. Saving again
or restarting s-watcher does not send another test.
