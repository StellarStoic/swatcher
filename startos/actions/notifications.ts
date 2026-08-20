// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { notificationConfig } from '../fileModels/notifications.json'
import {
  ensureNostrIdentity,
  uniqueSenderName,
  validateRecipientNpub,
} from '../nostrIdentity'
import { i18n } from '../i18n'
import { sdk } from '../sdk'
import { torSocksBridge } from '../utils'

const { InputSpec, Value } = sdk
const defaultNostrRelays = [
  'wss://relay.damus.io',
  'wss://nos.lol',
  'wss://auth.nostr1.com',
  'wss://relay.ditto.pub',
]
const inputSpec = InputSpec.of({
  dailyDigest: Value.toggle({
    name: i18n('Daily digest instead of immediate messages'),
    description: i18n(
      'Combines pending wallet activity into one message per day',
    ),
    default: false,
  }),
  digestHour: Value.number({
    name: i18n('Daily digest hour'),
    description: i18n('Hour of day using the UTC offset below'),
    required: true,
    default: 9,
    integer: true,
    min: 0,
    max: 23,
    units: i18n('hour'),
  }),
  quietHours: Value.toggle({
    name: i18n('Enable quiet hours for immediate messages'),
    default: false,
  }),
  quietStart: Value.number({
    name: i18n('Quiet hours start'),
    required: true,
    default: 22,
    integer: true,
    min: 0,
    max: 23,
    units: i18n('hour'),
  }),
  quietEnd: Value.number({
    name: i18n('Quiet hours end'),
    required: true,
    default: 7,
    integer: true,
    min: 0,
    max: 23,
    units: i18n('hour'),
  }),
  utcOffset: Value.number({
    name: i18n('Local UTC offset'),
    description: i18n(
      'Whole-hour offset used for quiet hours and daily digests',
    ),
    required: true,
    default: 0,
    integer: true,
    min: -12,
    max: 14,
    units: i18n('hours'),
  }),
  telegramEnabled: Value.toggle({
    name: i18n('Enable Telegram'),
    default: false,
  }),
  telegramToken: Value.text({
    name: i18n('Telegram bot token'),
    description: i18n('Token from BotFather'),
    required: false,
    default: null,
    masked: true,
  }),
  telegramChatId: Value.text({
    name: i18n('Telegram recipient ID'),
    description: i18n(
      'Your user ID for personal alerts, or an optional negative group chat ID; see Instructions',
    ),
    required: false,
    default: null,
  }),
  telegramTest: Value.dynamicToggle(async () => {
    const config = await notificationConfig.read().once()
    return {
      name: i18n('Send Telegram test message after save'),
      description: i18n(
        'One-time test using the saved Telegram token and recipient ID',
      ),
      default: false,
      disabled: config?.telegramEnabled
        ? false
        : i18n('Enable Telegram and save Notifications first'),
    }
  }),
  nostrEnabled: Value.toggle({
    name: i18n('Enable Nostr NIP-17 messages'),
    default: false,
  }),
  nostrRelays: Value.textarea({
    name: i18n('Nostr relays'),
    description: i18n(
      'One wss:// relay per line, used for discovery and the sender copy',
    ),
    required: false,
    default: defaultNostrRelays.join('\n'),
  }),
  nostrRecipient: Value.text({
    name: i18n('Recipient npub (your Nostr public key)'),
    description: i18n('Only paste an npub here. Never paste your secret nsec.'),
    required: false,
    default: null,
    placeholder: 'npub1…',
  }),
  nostrSenderName: Value.text({
    name: i18n('Sender name'),
    description: i18n(
      'A unique swatcher name is generated after saving unless you choose one',
    ),
    required: true,
    default: 'swatcher',
  }),
  nostrSenderNsec: Value.dynamicText(async () => ({
    name: i18n('Sender private key (nsec)'),
    description: i18n(
      'Generated and retained by s/watcher for use in another Nostr client. Will appear after save if Nostr is enabled.',
    ),
    required: false,
    default: null,
    masked: true,
    placeholder: i18n('Will appear after save if Nostr is enabled.'),
    disabled: i18n('Generated sender keys cannot be changed'),
  })),
  nostrSenderNpub: Value.dynamicText(async () => ({
    name: i18n('Sender public key (npub)'),
    description: i18n('Will appear after save if Nostr is enabled.'),
    required: false,
    default: null,
    placeholder: i18n('Will appear after save if Nostr is enabled.'),
    disabled: i18n('Generated sender keys cannot be changed'),
  })),
  nostrTest: Value.dynamicToggle(async () => {
    const config = await notificationConfig.read().once()
    return {
      name: i18n('Send Nostr test message after save'),
      description: i18n(
        'One-time NIP-17 test using the saved Nostr configuration',
      ),
      default: false,
      disabled: config?.nostrEnabled
        ? false
        : i18n('Enable Nostr and save Notifications first'),
    }
  }),
})
type NotificationInput = typeof inputSpec._TYPE

export const notifications = sdk.Action.withInput(
  'notifications',
  {
    name: i18n('Notifications'),
    description: i18n('Configure Telegram and private NIP-17 alerts'),
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
        i18n(
          'Enter your Nostr public key as an npub before enabling Nostr notifications.',
        ),
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
    return {
      version: '1',
      title: i18n('Notification settings saved'),
      message: i18n(
        'Reopen Notifications to see the generated Nostr sender keys.',
      ),
      result: null,
    }
  },
)
