# s/watcher

## Documentation

- [s/watcher](https://github.com/StellarStoic/swatcher) — the project's own
  documentation: what each watch type accepts, how discovery works, and the
  full list of supported descriptors.

## What you get on StartOS

s/watcher provides a password-protected Web UI for watch-only Bitcoin activity.
It reads balances, history, raw transactions, and block times from Electrs on
the same StartOS server. Mempool adds optional private explorer links, and Tor
can route NIP-17 delivery through onion relays.

All persistent settings and watch data live in the service's StartOS volume and
are included in backups.

## Getting set up

1. Make sure the required Electrs service is installed, running, and fully
   synchronized.
2. Open **Actions → Set Web Password**. Enter the same password twice; it must
   contain at least five characters.
3. Start s/watcher and open **Web UI**.
4. Add a name, group, optional note, and one supported watch source.
5. Wait for the first local scan. Existing history establishes the baseline and
   is not sent as new activity.
6. Optionally open **Actions → Notifications** and configure Telegram, Nostr,
   or both. Use each channel's test option after saving its settings.

If you forget the Web UI password, run **Set Web Password** again. Replacing it
signs out existing browser sessions but does not remove watches.

## Adding watches

The Web UI accepts:

- A Bitcoin mainnet address.
- An account-level `xpub`, `ypub`, or `zpub`.
- A `pkh()`, `wpkh()`, `sh(wpkh())`, or `tr()` output descriptor.
- A bulk list of between 2 and 10,000 unique mainnet addresses.

A bare `xpub` does not identify its address type, so select Legacy, Nested
SegWit, Native SegWit, or Taproot. `ypub`, `zpub`, descriptors, and individual
addresses determine their type automatically. Enable the change branch when
the imported wallet uses `/1` addresses.

Descriptors must contain a public extended key and a non-hardened wildcard path
ending in `/*`. A `<0;1>` expression covers receive and change branches.
Hardened derivation below an xpub is not possible and is rejected.

Names and groups are normalized to lowercase and may contain letters, numbers,
spaces, and underscores. Notes accept up to 500 plain-text characters. Never
paste a seed phrase, Bitcoin private key, WIF, or extended private key.

Wallet-name and group tags receive a light color derived from their normalized
text. The color is stable across reloads and ignores letter case, so `Peach`,
`peach`, and `PeAch` always use the same color.

### Smart discovery

Extended keys and ranged descriptors derive addresses until the selected number
of consecutive unused indexes exists beyond the highest used address. Change
the gap with **Actions → General → Smart Wallet Discovery**. A larger gap finds
wallets that skipped more indexes but makes more local Electrs queries.

There is no application-level address limit. Large gaps can create substantial
local Electrs traffic and a large state file, so increase them deliberately.
Reducing the global gap never deletes addresses already saved. Newly derived
historical addresses are baselined without false notifications.

To expand one existing extended-key or ranged-descriptor watch, select **Edit**
and increase **Watched indexes per branch**. Existing balances, transactions,
and notes remain intact; only the additional addresses are derived and
baselined. The value cannot be reduced. A receive-and-change watch derives that
many indexes on each branch, so a value of 100 watches 200 addresses.

### Bulk import and combine

Select **Paste in bulk** to add addresses separated by whitespace, commas, or
semicolons. The import is atomic: if any entry is invalid or already watched,
nothing is added.

When pasted addresses overlap existing watches, the warning lists every
duplicate and its current watch. Select **Add N new addresses only** to remove
the listed duplicates and create the bulk watch from the remaining addresses.
The option is not shown when every pasted address is already watched.

Select **Combine** to reveal selection boxes for existing watches. Choose at
least two rows, then **Combine selected**. Combining creates one fixed address
collection; combining an xpub or descriptor retains its current derived
addresses but stops future discovery for that source. **Cancel** exits selection
mode without changing anything.

Select **Remove** to delete a saved watch. s/watcher names the watch and asks
**Are you sure you want to remove this watch?** Choose **No** or press Escape to
keep it; choose **Yes** to remove it.

## Transaction history

Each watch shows its latest transaction. Select **Show all transactions** for
the complete history returned by local Electrs, 100 entries per page. Sort by
time, value, direction, or confirmation state. When a transaction contains more
than 10 input and output address rows, s/watcher shows the first 10 and places a
**Show N more addresses** button underneath. Use it to expand or collapse that
transaction's complete address list.

Each transaction can show:

- Received, sent, and net amounts.
- Input, watched-input, and destination addresses with their exact amounts.
- Confirmation state, block height, time, and RBF status.
- Printable UTF-8 OP_RETURN messages.
- Detection-only Runes and Ordinals markers.
- Address reuse, small-deposit, and combined-wallet information badges.
- A link to the transaction in your optional local Mempool service.
- **Copy text** for a readable, branded summary containing the visible details.
- **Save image** for a branded PNG transaction card you can share or archive.

Select any visible transaction address to copy it. Privacy Mode keeps masked
addresses non-copyable and keeps identifiers and amounts masked in both export
formats. Input addresses identify previous outputs consumed by the transaction;
they do not prove the identity of a real-world sender.

Transactions whose inputs cannot yet be resolved remain visible as **Details
pending** and are retried. Large histories may need several scan cycles.

## Find address

Use **Find address** to search only the addresses already stored by s/watcher,
including derived xpub and descriptor children. A match displays the watch,
derivation path, address type, local balance, known history, and note.
Selecting **Show wallet** centers the matching watch and softly highlights its
row for five seconds. A newly added watch receives the same visual highlight.

**Not found** means the address is outside saved coverage; it does not prove the
address cannot belong to a wallet beyond the current discovery gap. When the
optional Mempool dependency is available, you can open the address in your own
Mempool instance. s/watcher never substitutes a public explorer.

## Notification rules

Select **Edit** on a watch to configure:

- Every transaction, incoming only, outgoing only, or notifications off.
- A minimum activity amount.
- Delivery in the mempool or after 1, 3, or 6 confirmations.

Open **Actions → Notifications** to configure immediate delivery, quiet hours,
or one daily digest. Scheduling uses the whole-hour UTC offset entered there.
Delivery status is saved separately for Telegram and Nostr to avoid resending a
successfully delivered event after restart.

### Telegram

1. Create a bot with Telegram's **@BotFather** and copy its token.
2. Send that bot a direct message.
3. Obtain your numeric user ID from Telegram's Bot API `getUpdates` response or
   a trusted ID bot.
4. Enter the bot token and your user ID as **Telegram recipient ID**.
5. Save, reopen **Notifications**, enable the Telegram test option, and save
   again.

A negative group chat ID may be used instead, but a group is not required for
personal notifications.

### Nostr NIP-17

1. Enter your receiver `npub`. Never enter your personal `nsec`.
2. Keep or replace the default `wss://` relays. The receiver should publish a
   kind `10050` DM relay list discoverable from them.
3. Enable Nostr and save. s/watcher generates a dedicated sender identity,
   unique name, and Pixelbot avatar.
4. Reopen **Notifications** to view the generated sender `npub` and masked
   sender `nsec`, or send a Nostr test message.

The generated sender identity persists when Nostr is disabled and is restored
with the service backup. It is an application messaging identity, not a Bitcoin
key. NIP-17 protects message content, but public relays can still observe
connection metadata.

## Privacy and appearance actions

- **Privacy Mode** masks balances, amounts, notes, addresses, extended keys,
  descriptors, transaction IDs, and the displayed Nostr sender npub. Set the
  web password first. Disabling Privacy Mode requires that password.
- **Privacy Indicators** controls informational address-reuse, small-deposit,
  and combined-wallet badges.
- **Theme** selects Bitcoin Night, Cypherpunk Neon, Arctic Node, Forest Ledger,
  or Paper Ledger.

Privacy Mode changes only the Web UI. Notification messages retain transaction
amounts and details.

## Backups and sensitive data

StartOS backs up every persistent s/watcher file, including watches, derived
addresses, history, password verifier, notification credentials, and the Nostr
sender identity. Treat the backup as sensitive.

s/watcher cannot spend Bitcoin, but an xpub or public descriptor reveals the
wallet's transaction graph. Telegram tokens and the generated Nostr sender
`nsec` also require protection.

## Limitations

- Bitcoin mainnet only.
- Watch-only; no transaction construction, signing, or spending.
- Runes and Ordinals are detected but not decoded or rendered.
- Source input addresses are transaction-level evidence, not sender identity.
- Large xpub histories and bulk groups can require multiple local scan cycles.
