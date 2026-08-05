// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { i18n } from './i18n'
import { sdk } from './sdk'
import { electrumBridge, localMempoolUrls, uiPort } from './utils'
import { notificationConfig } from './fileModels/notifications.json'

export const main = sdk.setupMain(async ({ effects }) => {
  console.info(i18n('Starting s-watcher'))
  const electrum = await electrumBridge(effects)
  const mempoolUrls = await localMempoolUrls(effects)
  await notificationConfig.read().const(effects)

  return sdk.Daemons.of(effects).addDaemon('primary', {
    subcontainer: sdk.SubContainer.of(
      effects,
      { imageId: 's-watcher' },
      sdk.Mounts.of().mountVolume({
        volumeId: 'main',
        subpath: null,
        mountpoint: '/data',
        readonly: false,
      }),
      's-watcher',
    ),
    exec: {
      command: [
        'sh',
        '-c',
        'chown swatcher:swatcher /data && exec su-exec swatcher s-watcher',
      ],
      user: 'root',
      env: {
        SWATCHER_LISTEN: `:${uiPort}`,
        SWATCHER_DATA: '/data',
        ELECTRUM_ADDR: electrum ?? '127.0.0.1:50001',
        SWATCHER_MEMPOOL_URLS: JSON.stringify(mempoolUrls),
      },
    },
    ready: {
      display: i18n('Web Interface'),
      fn: () =>
        sdk.healthCheck.checkPortListening(effects, uiPort, {
          successMessage: i18n('The web interface is ready'),
          errorMessage: i18n('The web interface is not ready'),
        }),
    },
    requires: [],
  })
})
