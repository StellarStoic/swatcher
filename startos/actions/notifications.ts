import { notificationConfig } from '../fileModels/notifications.json'
import { ensureNostrIdentity, uniqueSenderName } from '../nostrIdentity'
import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
const defaultNostrRelays = [
  'wss://relay.damus.io',
  'wss://nos.lol',
  'wss://auth.nostr1.com',
  'wss://relay.ditto.pub',
]
const inputSpec = InputSpec.of({
  telegramEnabled: Value.toggle({ name: 'Enable Telegram', default: false }),
  telegramToken: Value.text({
    name: 'Telegram bot token',
    description: 'Token from BotFather',
    required: false,
    default: null,
    masked: true,
  }),
  telegramChatId: Value.text({
    name: 'Telegram recipient ID',
    description:
      'Your user ID for personal alerts, or an optional negative group chat ID; see Instructions',
    required: false,
    default: null,
  }),
  nostrEnabled: Value.toggle({
    name: 'Enable Nostr NIP-17 messages',
    default: false,
  }),
  nostrRelays: Value.textarea({
    name: 'Nostr relays',
    description:
      'One wss:// relay per line, used for discovery and the sender copy',
    required: false,
    default: defaultNostrRelays.join('\n'),
  }),
  nostrRecipient: Value.text({
    name: 'Recipient npub',
    required: false,
    default: null,
  }),
  nostrSenderName: Value.text({
    name: 'Sender name',
    description:
      'A unique swatcher name is generated after saving unless you choose one',
    required: true,
    default: 'swatcher',
  }),
  nostrSenderNpub: Value.text({
    name: 'Sender public key (npub)',
    description:
      'Generated after saving with Nostr enabled; the private key is managed internally',
    required: false,
    default: null,
    immutable: true,
  }),
  nostrAvatar: Value.text({
    name: 'Sender avatar URL',
    description:
      'Generated after saving with Nostr enabled and published in the Nostr profile',
    required: false,
    default: null,
    immutable: true,
  }),
})
type NotificationInput = typeof inputSpec._TYPE

export const notifications = sdk.Action.withInput(
  'notifications',
  {
    name: 'Notifications',
    description: 'Configure Telegram and private NIP-17 alerts',
    warning: null,
    allowedStatuses: 'any',
    group: null,
    visibility: 'enabled',
  },
  inputSpec,
  async ({ effects }) => {
    const c = await notificationConfig.read().const(effects)
    const configuredRelays = c?.nostrRelays ?? []
    return {
      telegramEnabled: c?.telegramEnabled ?? false,
      telegramToken: c?.telegramToken || null,
      telegramChatId: c?.telegramChatId || null,
      nostrEnabled: c?.nostrEnabled ?? false,
      nostrRelays:
        configuredRelays.length > 0
          ? configuredRelays.join('\n')
          : defaultNostrRelays.join('\n'),
      nostrRecipient: c?.nostrRecipient || null,
      nostrSenderName: c?.nostrSenderName || 'swatcher',
      nostrSenderNpub: c?.nostrSenderNpub || null,
      nostrAvatar: c?.nostrAvatar || null,
    } satisfies NotificationInput
  },
  async ({ effects, input }) => {
    const previous = await notificationConfig.read().once()
    const selectedNsec = previous?.nostrSenderNsec || ''
    const identity =
      input.nostrEnabled || selectedNsec
        ? ensureNostrIdentity(selectedNsec)
        : null
    const requestedName = input.nostrSenderName.trim()
    const previousName = previous?.nostrSenderName || ''
    const previousNameIsDefault =
      previousName === 's-watcher' || previousName === 'swatcher'
    const requestedNameIsDefault =
      requestedName === 's-watcher' || requestedName === 'swatcher'
    const senderName =
      (!previousNameIsDefault && previousName) ||
      (requestedName && !requestedNameIsDefault
        ? requestedName
        : uniqueSenderName())
    const effectiveName =
      requestedName && !requestedNameIsDefault ? requestedName : senderName
    const identityChanged = identity?.npub !== (previous?.nostrSenderNpub || '')
    const profileChanged =
      effectiveName !== previous?.nostrSenderName || identityChanged
    const requestedRelays = (input.nostrRelays ?? '')
      .split(/\s+/)
      .map((x) => x.trim())
      .filter(Boolean)
    await notificationConfig.write(effects, {
      telegramEnabled: input.telegramEnabled,
      telegramToken: input.telegramToken?.trim() ?? '',
      telegramChatId: input.telegramChatId?.trim() ?? '',
      nostrEnabled: input.nostrEnabled,
      nostrRelays:
        requestedRelays.length > 0 ? requestedRelays : defaultNostrRelays,
      nostrRecipient: input.nostrRecipient?.trim() ?? '',
      nostrSenderName: effectiveName,
      nostrSenderNsec: identity?.nsec ?? '',
      nostrSenderNpub: identity?.npub ?? '',
      nostrAvatar: identity?.avatar ?? '',
      nostrProfilePublished:
        profileChanged || !input.nostrEnabled
          ? false
          : (previous?.nostrProfilePublished ?? false),
      telegramTestPending: input.telegramEnabled,
      nostrTestPending: input.nostrEnabled,
    })
    await sdk.restart(effects)
  },
)
