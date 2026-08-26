// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.2:0',
  releaseNotes: {
    en_US:
      'Adds editable derivation coverage without the former 500-address cap, safer bulk imports, removal confirmation, a favicon, transaction fixes, and five-second wallet highlighting.',
    es_ES:
      'Añade cobertura de derivación editable sin el límite anterior de 500 direcciones, importaciones más seguras y un resaltado de cartera de cinco segundos.',
    de_DE:
      'Ergänzt eine bearbeitbare Ableitungsabdeckung ohne die frühere 500-Adressen-Grenze, sicherere Massenimporte und eine fünfsekündige Wallet-Hervorhebung.',
    pl_PL:
      'Dodaje edytowalny zakres derywacji bez dawnego limitu 500 adresów, bezpieczniejszy import zbiorczy i pięciosekundowe wyróżnienie portfela.',
    fr_FR:
      'Ajoute une couverture de dérivation modifiable sans l’ancienne limite de 500 adresses, des imports plus sûrs et une surbrillance du portefeuille pendant cinq secondes.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
