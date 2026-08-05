// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

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
    dailyDigest: z.boolean().catch(false),
    digestHour: z.number().int().min(0).max(23).catch(9),
    quietHours: z.boolean().catch(false),
    quietStart: z.number().int().min(0).max(23).catch(22),
    quietEnd: z.number().int().min(0).max(23).catch(7),
    utcOffset: z.number().int().min(-12).max(14).catch(0),
  }),
)
