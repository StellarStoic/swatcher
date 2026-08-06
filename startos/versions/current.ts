// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:30',
  releaseNotes: {
    en_US:
      'Fixes NIP-17 delivery to authenticated relays with NIP-42, adds optional StartOS Tor routing for onion relays, and reports actionable per-relay delivery errors.',
    de_DE:
      'Behebt die NIP-17-Zustellung an authentifizierte Relays mit NIP-42, ergänzt optionales StartOS-Tor-Routing für Onion-Relays und meldet aussagekräftige Zustellfehler je Relay.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
