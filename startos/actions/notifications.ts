import { notificationConfig } from '../fileModels/notifications.json'
import { ensureNostrIdentity, uniqueSenderName } from '../nostrIdentity'
import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
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
    default: null,
  }),
  nostrRecipient: Value.text({
    name: 'Recipient npub',
    required: false,
    default: null,
  }),
  nostrSenderName: Value.text({
    name: 'Sender name',
    required: true,
    default: 'swatcher',
  }),
  nostrSenderNsec: Value.text({
    name: 'Sender nsec',
    description:
      'Leave empty to generate a persistent sender. Replace to change identity.',
    required: false,
    default: null,
    masked: true,
  }),
  nostrSenderNpub: Value.text({
    name: 'Sender npub',
    description: 'Generated immediately from the persistent sender nsec',
    required: false,
    default: null,
    immutable: true,
  }),
  nostrAvatar: Value.text({
    name: 'Sender avatar URL',
    description: 'Generated DiceBear avatar published in the Nostr profile',
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
    return {
      telegramEnabled: c?.telegramEnabled ?? false,
      telegramToken: c?.telegramToken || null,
      telegramChatId: c?.telegramChatId || null,
      nostrEnabled: c?.nostrEnabled ?? false,
      nostrRelays: c?.nostrRelays.join('\n') || null,
      nostrRecipient: c?.nostrRecipient || null,
      nostrSenderName: c?.nostrSenderName || 'swatcher',
      nostrSenderNsec: c?.nostrSenderNsec || null,
      nostrSenderNpub: c?.nostrSenderNpub || null,
      nostrAvatar: c?.nostrAvatar || null,
    } satisfies NotificationInput
  },
  async ({ effects, input }) => {
    const previous = await notificationConfig.read().once()
    const suppliedNsec = input.nostrSenderNsec?.trim() ?? ''
    const selectedNsec = suppliedNsec || previous?.nostrSenderNsec || ''
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
    await notificationConfig.write(effects, {
      telegramEnabled: input.telegramEnabled,
      telegramToken: input.telegramToken?.trim() ?? '',
      telegramChatId: input.telegramChatId?.trim() ?? '',
      nostrEnabled: input.nostrEnabled,
      nostrRelays: (input.nostrRelays ?? '')
        .split(/\s+/)
        .map((x) => x.trim())
        .filter(Boolean),
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
