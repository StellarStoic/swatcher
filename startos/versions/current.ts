// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.1:0',
  releaseNotes: {
    en_US:
      'Adds a local Find address tool that identifies saved watches, including already-derived xpub and descriptor addresses, without network requests.',
    de_DE:
      'Ergänzt eine lokale Adresssuche für gespeicherte Watches einschließlich bereits abgeleiteter xpub- und Descriptor-Adressen ohne Netzwerkanfragen.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
