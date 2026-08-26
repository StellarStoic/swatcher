// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.2:0',
  releaseNotes: {
    en_US:
      'Adds editable per-wallet derivation coverage without the former 500-address cap, plus safer bulk imports, removal confirmation, a favicon, and transaction UI fixes.',
    es_ES:
      'Añade cobertura de derivación editable por cartera sin el límite anterior de 500 direcciones, además de importaciones masivas más seguras y mejoras de interfaz.',
    de_DE:
      'Ergänzt eine bearbeitbare Ableitungsabdeckung pro Wallet ohne die frühere 500-Adressen-Grenze sowie sicherere Massenimporte und UI-Verbesserungen.',
    pl_PL:
      'Dodaje edytowalny zakres derywacji dla portfela bez dawnego limitu 500 adresów oraz bezpieczniejszy import zbiorczy i poprawki interfejsu.',
    fr_FR:
      'Ajoute une couverture de dérivation modifiable par portefeuille sans l’ancienne limite de 500 adresses, ainsi que des imports groupés plus sûrs et des améliorations d’interface.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
