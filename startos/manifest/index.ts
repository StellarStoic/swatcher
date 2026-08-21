// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { setupManifest } from '@start9labs/start-sdk'
import {
  electrsDescription,
  long,
  mempoolDescription,
  short,
  torDescription,
} from './i18n'

export const manifest = setupManifest({
  id: 's-watcher',
  title: 's/watcher',
  license: 'AGPL-3.0-or-later',
  packageRepo: 'https://github.com/Start9-Community/swatcher',
  upstreamRepo: 'https://github.com/StellarStoic/swatcher',
  marketingUrl: 'https://github.com/StellarStoic/swatcher',
  donationUrl: null,
  description: { short, long },
  volumes: ['main'],
  images: {
    's-watcher': {
      source: {
        dockerBuild: {
          dockerfile: 'Dockerfile',
          workdir: '.',
        },
      },
      arch: ['x86_64', 'aarch64'],
    },
  },
  dependencies: {
    electrs: {
      description: electrsDescription,
      optional: false,
      metadata: {
        title: 'Electrs',
        icon: 'https://raw.githubusercontent.com/Start9Labs/electrs-startos/refs/heads/master/icon.svg',
      },
    },
    mempool: {
      description: mempoolDescription,
      optional: true,
      metadata: {
        title: 'Mempool',
        icon: 'https://raw.githubusercontent.com/Start9Labs/mempool-startos/refs/heads/master/icon.svg',
      },
    },
    tor: {
      description: torDescription,
      optional: true,
      metadata: {
        title: 'Tor',
        icon: 'https://raw.githubusercontent.com/Start9Labs/tor-startos/refs/heads/master/icon.svg',
      },
    },
  },
})
