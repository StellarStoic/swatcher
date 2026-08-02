import { notificationConfig } from '../fileModels/notifications.json'
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
    name: 'Telegram chat ID',
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
    default: 's-watcher',
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
    description:
      'Generated from the persistent sender nsec after the service starts',
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
      nostrSenderName: c?.nostrSenderName || 's-watcher',
      nostrSenderNsec: c?.nostrSenderNsec || null,
      nostrSenderNpub: c?.nostrSenderNpub || null,
    } satisfies NotificationInput
  },
  async ({ effects, input }) => {
    const previous = await notificationConfig.read().once()
    const suppliedNsec = input.nostrSenderNsec?.trim() ?? ''
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
      nostrSenderName: input.nostrSenderName.trim(),
      nostrSenderNsec: suppliedNsec || previous?.nostrSenderNsec || '',
      nostrSenderNpub:
        suppliedNsec === previous?.nostrSenderNsec
          ? (previous?.nostrSenderNpub ?? '')
          : '',
      nostrAvatar:
        suppliedNsec === previous?.nostrSenderNsec
          ? (previous?.nostrAvatar ?? '')
          : '',
    })
  },
)
