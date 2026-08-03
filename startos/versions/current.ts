// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:15',
  releaseNotes: {
    en_US:
      'Fixes legitimate Web Interface actions being rejected when accessed through the StartOS Tor proxy.',
    de_DE:
      'Behebt abgelehnte legitime Aktionen der Weboberfläche beim Zugriff über den StartOS-Tor-Proxy.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
