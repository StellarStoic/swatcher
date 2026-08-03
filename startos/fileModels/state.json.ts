// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { FileHelper, z } from '@start9labs/start-sdk'
import { sdk } from '../sdk'

export const stateConfig = FileHelper.json(
  { base: sdk.volumes.main, subpath: '/state.json' },
  z
    .object({
      privacyMode: z.boolean().catch(false),
    })
    .passthrough(),
)
