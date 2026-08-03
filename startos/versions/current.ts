// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:13',
  releaseNotes: {
    en_US:
      'Introduces s/watcher branding, hardens the Web Interface, and improves privacy controls and watch focus.',
    de_DE:
      'Führt das s/watcher-Branding ein, härtet die Weboberfläche und verbessert Datenschutz und Wallet-Fokus.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
