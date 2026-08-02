import { setupManifest } from '@start9labs/start-sdk'
import { electrsDescription, long, short } from './i18n'

export const manifest = setupManifest({
  id: 's-watcher',
  title: 's-watcher',
  license: 'AGPL-3.0-only',
  packageRepo: 'https://github.com/StellarStoic/swatcher',
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
  },
})
