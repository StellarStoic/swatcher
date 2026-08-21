// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { notificationConfig } from './fileModels/notifications.json'
import { sdk } from './sdk'
import { mempoolHost, usesOnionRelay } from './utils'

// Everything returned here warns the user while it is missing or stopped, so
// the two optional dependencies are declared only once they are actually in
// use — otherwise a feature nobody opted into reports the service as degraded.
export const setDependencies = sdk.setupDependencies(async ({ effects }) => {
  const mempoolInstalled = !!(await mempoolHost(effects))
  const notifications = await notificationConfig.read().const(effects)
  const needsTor =
    !!notifications?.nostrEnabled && usesOnionRelay(notifications.nostrRelays)

  return {
    electrs: {
      kind: 'running',
      versionRange: '>=0.11.1:11',
      healthChecks: ['electrs', 'sync'],
    },
    ...(mempoolInstalled && {
      mempool: {
        kind: 'running',
        versionRange: '>=3.3.1:11',
        healthChecks: ['webui'],
      },
    }),
    ...(needsTor && {
      tor: {
        kind: 'running',
        versionRange: '>=0.4.9.11:1',
        healthChecks: ['tor'],
      },
    }),
  }
})
