import { sdk } from '../sdk'

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
    async () => ({
      name: channel === 'telegram' ? 'Test Telegram' : 'Test Nostr',
      description: `Send a test message using the saved ${channel === 'telegram' ? 'Telegram' : 'Nostr'} configuration`,
      warning: null,
      allowedStatuses: 'any',
      group: 'Notifications',
      visibility: 'enabled',
    }),
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
