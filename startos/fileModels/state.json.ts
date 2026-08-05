// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { FileHelper, z } from '@start9labs/start-sdk'
import { sdk } from '../sdk'

export const stateConfig = FileHelper.json(
  { base: sdk.volumes.main, subpath: '/state.json' },
  z
    .object({
      privacyMode: z.boolean().catch(false),
      discoveryGap: z.number().int().min(1).max(500).catch(20),
      privacyIndicatorsConfigured: z.boolean().catch(false),
      addressReuseIndicators: z.boolean().catch(true),
      smallDepositIndicators: z.boolean().catch(true),
      combinedWalletIndicators: z.boolean().catch(true),
      smallDepositThreshold: z.number().int().positive().catch(1000),
      theme: z
        .enum(['bitcoin-night', 'cypherpunk', 'arctic', 'forest', 'paper'])
        .catch('bitcoin-night'),
    })
    .passthrough(),
)
