import { sdk } from '../sdk'
import { notificationConfig } from '../fileModels/notifications.json'

const mounts = () =>
  sdk.Mounts.of().mountVolume({
    volumeId: 'main',
    subpath: null,
    mountpoint: '/data',
    readonly: false,
  })

const testNotification = (channel: 'telegram' | 'nostr') =>
  sdk.Action.withoutInput(
    `test-${channel}`,
    async () => {
      const config = await notificationConfig.read().once()
      const enabled =
        channel === 'telegram'
          ? (config?.telegramEnabled ?? false)
          : (config?.nostrEnabled ?? false)
      const channelName = channel === 'telegram' ? 'Telegram' : 'Nostr'
      return {
        name: `Send ${channelName} test message`,
        description: `Send a test message using the saved ${channelName} configuration`,
        warning: null,
        allowedStatuses: 'any',
        group: `Notifications · ${channelName}`,
        visibility: enabled ? ('enabled' as const) : ('hidden' as const),
      }
    },
    async ({ effects }) => {
      await sdk.SubContainer.withTemp(
        effects,
        { imageId: 's-watcher' },
        mounts(),
        `test-${channel}`,
        async (sub) => {
          await sub.execFail(['s-watcher', 'test-notification', channel], {
            env: { SWATCHER_DATA: '/data' },
          })
        },
      )
      return {
        version: '1',
        title: `${channel === 'telegram' ? 'Telegram' : 'Nostr'} test sent`,
        message: 'The test notification was delivered successfully.',
        result: null,
      }
    },
  )

export const testTelegram = testNotification('telegram')
export const testNostr = testNotification('nostr')
