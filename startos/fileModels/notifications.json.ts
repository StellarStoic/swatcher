import { FileHelper, z } from '@start9labs/start-sdk'
import { sdk } from '../sdk'

export const notificationConfig = FileHelper.json(
  { base: sdk.volumes.main, subpath: '/notifications.json' },
  z.object({
    telegramEnabled: z.boolean().catch(false),
    telegramToken: z.string().catch(''),
    telegramChatId: z.string().catch(''),
    nostrEnabled: z.boolean().catch(false),
    nostrRelays: z.array(z.string()).catch([]),
    nostrRecipient: z.string().catch(''),
    nostrSenderName: z.string().catch('s-watcher'),
    nostrSenderNsec: z.string().catch(''),
    nostrSenderNpub: z.string().catch(''),
    nostrAvatar: z.string().catch(''),
    nostrProfilePublished: z.boolean().catch(false),
    telegramTestPending: z.boolean().catch(false),
    nostrTestPending: z.boolean().catch(false),
  }),
)
