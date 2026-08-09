// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { sdk } from './sdk'

export const setDependencies = sdk.setupDependencies(async () => ({
  electrs: {
    kind: 'running',
    versionRange: '>=0.11.1:11',
    healthChecks: ['electrs', 'sync'],
  },
  mempool: {
    kind: 'running',
    versionRange: '>=3.3.1:11',
    healthChecks: ['webui'],
  },
  tor: {
    kind: 'running',
    versionRange: '>=0.4.9.11:1',
    healthChecks: ['tor'],
  },
}))
