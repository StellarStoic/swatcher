// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.0:29',
  releaseNotes: {
    en_US:
      'Fixes watch submissions and adds detailed Telegram and NIP-17 messages with local onion transaction links, protocol markers, OP_RETURN text, balances, and confirmation state.',
    de_DE:
      'Behebt Übermittlungen neuer Überwachungen und ergänzt detaillierte Telegram- und NIP-17-Meldungen mit lokalen Onion-Transaktionslinks, Protokollmarkierungen, OP_RETURN-Text, Salden und Bestätigungsstatus.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
