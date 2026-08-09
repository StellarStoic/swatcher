// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { notificationConfig } from '../fileModels/notifications.json'
import {
  ensureNostrIdentity,
  uniqueSenderName,
  validateRecipientNpub,
} from '../nostrIdentity'
import { torSocksBridge } from '../utils'
import { sdk } from '../sdk'

const { InputSpec, Value } = sdk
const defaultNostrRelays = [
  'wss://relay.damus.io',
  'wss://nos.lol',
  'wss://auth.nostr1.com',
  'wss://relay.ditto.pub',
]
const inputSpec = InputSpec.of({
  dailyDigest: Value.toggle({
    name: 'Daily digest instead of immediate messages',
    description: 'Combines pending wallet activity into one message per day',
    default: false,
  }),
  digestHour: Value.number({
    name: 'Daily digest hour',
    description: 'Hour of day using the UTC offset below',
    required: true,
    default: 9,
    integer: true,
    min: 0,
    max: 23,
    units: 'hour',
  }),
  quietHours: Value.toggle({
    name: 'Enable quiet hours for immediate messages',
    default: false,
  }),
  quietStart: Value.number({
    name: 'Quiet hours start',
    required: true,
    default: 22,
    integer: true,
    min: 0,
    max: 23,
    units: 'hour',
  }),
  quietEnd: Value.number({
    name: 'Quiet hours end',
    required: true,
    default: 7,
    integer: true,
    min: 0,
    max: 23,
    units: 'hour',
  }),
  utcOffset: Value.number({
    name: 'Local UTC offset',
    description: 'Whole-hour offset used for quiet hours and daily digests',
    required: true,
    default: 0,
    integer: true,
    min: -12,
    max: 14,
    units: 'hours',
  }),
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
  telegramTest: Value.dynamicToggle(async () => {
    const config = await notificationConfig.read().once()
    return {
      name: 'Send Telegram test message after save',
      description:
        'One-time test using the saved Telegram token and recipient ID',
      default: false,
      disabled: config?.telegramEnabled
        ? false
        : 'Enable Telegram and save Notifications first',
    }
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
    name: 'Recipient npub (your Nostr public key)',
    description: 'Only paste an npub here. Never paste your secret nsec.',
    required: false,
    default: null,
    placeholder: 'npub1…',
  }),
  nostrSenderName: Value.text({
    name: 'Sender name',
    description:
      'A unique swatcher name is generated after saving unless you choose one',
    required: true,
    default: 'swatcher',
  }),
  nostrSenderNsec: Value.dynamicText(async () => ({
    name: 'Sender private key (nsec)',
    description:
      'Generated and retained by s/watcher for use in another Nostr client. Will appear after save if Nostr is enabled.',
    required: false,
    default: null,
    masked: true,
    placeholder: 'Will appear after save if Nostr is enabled.',
    disabled: 'Generated sender keys cannot be changed',
  })),
  nostrSenderNpub: Value.dynamicText(async () => ({
    name: 'Sender public key (npub)',
    description: 'Will appear after save if Nostr is enabled.',
    required: false,
    default: null,
    placeholder: 'Will appear after save if Nostr is enabled.',
    disabled: 'Generated sender keys cannot be changed',
  })),
  nostrTest: Value.dynamicToggle(async () => {
    const config = await notificationConfig.read().once()
    return {
      name: 'Send Nostr test message after save',
      description: 'One-time NIP-17 test using the saved Nostr configuration',
      default: false,
      disabled: config?.nostrEnabled
        ? false
        : 'Enable Nostr and save Notifications first',
    }
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
      dailyDigest: c?.dailyDigest ?? false,
      digestHour: c?.digestHour ?? 9,
      quietHours: c?.quietHours ?? false,
      quietStart: c?.quietStart ?? 22,
      quietEnd: c?.quietEnd ?? 7,
      utcOffset: c?.utcOffset ?? 0,
      telegramEnabled: c?.telegramEnabled ?? false,
      telegramToken: c?.telegramToken || null,
      telegramChatId: c?.telegramChatId || null,
      telegramTest: false,
      nostrEnabled: c?.nostrEnabled ?? false,
      nostrRelays:
        configuredRelays.length > 0
          ? configuredRelays.join('\n')
          : defaultNostrRelays.join('\n'),
      nostrRecipient: c?.nostrRecipient || null,
      nostrSenderName: c?.nostrSenderName || 'swatcher',
      nostrSenderNsec: c?.nostrSenderNsec || null,
      nostrSenderNpub: c?.nostrSenderNpub || null,
      nostrTest: false,
    } satisfies NotificationInput
  },
  async ({ effects, input }) => {
    const requestedRecipient = input.nostrRecipient?.trim() ?? ''
    const nostrRecipient = requestedRecipient
      ? validateRecipientNpub(requestedRecipient)
      : ''
    if (input.nostrEnabled && !nostrRecipient) {
      throw new Error(
        'Enter your Nostr public key as an npub before enabling Nostr notifications.',
      )
    }
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
      dailyDigest: input.dailyDigest,
      digestHour: input.digestHour,
      quietHours: input.quietHours,
      quietStart: input.quietStart,
      quietEnd: input.quietEnd,
      utcOffset: input.utcOffset,
      telegramEnabled: input.telegramEnabled,
      telegramToken: input.telegramToken?.trim() ?? '',
      telegramChatId: input.telegramChatId?.trim() ?? '',
      nostrEnabled: input.nostrEnabled,
      nostrRelays:
        requestedRelays.length > 0 ? requestedRelays : defaultNostrRelays,
      nostrRecipient,
      nostrSenderName: effectiveName,
      nostrSenderNsec: identity?.nsec ?? '',
      nostrSenderNpub: identity?.npub ?? '',
      nostrAvatar: identity?.avatar ?? '',
      nostrProfilePublished:
        profileChanged || !input.nostrEnabled
          ? false
          : (previous?.nostrProfilePublished ?? false),
    })
    const testChannels: Array<'telegram' | 'nostr'> = []
    if (input.telegramEnabled && input.telegramTest) {
      testChannels.push('telegram')
    }
    if (input.nostrEnabled && input.nostrTest) {
      testChannels.push('nostr')
    }
    if (testChannels.length > 0) {
      const torSocks = await torSocksBridge(effects)
      const mounts = sdk.Mounts.of().mountVolume({
        volumeId: 'main',
        subpath: null,
        mountpoint: '/data',
        readonly: false,
      })
      await sdk.SubContainer.withTemp(
        effects,
        { imageId: 's-watcher' },
        mounts,
        'test-notifications',
        async (sub) => {
          for (const channel of testChannels) {
            await sub.execFail(['s-watcher', 'test-notification', channel], {
              env: {
                SWATCHER_DATA: '/data',
                TOR_SOCKS_ADDR: torSocks ?? '',
              },
              user: 'root',
            })
          }
        },
      )
    }
  },
)
