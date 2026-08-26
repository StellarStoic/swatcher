// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.2:0',
  releaseNotes: {
    en_US:
      'Adds stable case-insensitive tag colors, editable derivation coverage, safer bulk imports, removal confirmation, a favicon, transaction fixes, and wallet highlighting.',
    es_ES:
      'Añade colores de etiqueta estables sin distinguir mayúsculas, cobertura de derivación editable, importaciones más seguras y resaltado de cartera.',
    de_DE:
      'Ergänzt stabile Tag-Farben ohne Beachtung der Großschreibung, bearbeitbare Ableitungsabdeckung, sicherere Massenimporte und Wallet-Hervorhebung.',
    pl_PL:
      'Dodaje trwałe kolory etykiet niezależne od wielkości liter, edytowalny zakres derywacji, bezpieczniejszy import i wyróżnienie portfela.',
    fr_FR:
      'Ajoute des couleurs d’étiquette stables sans distinction de casse, une dérivation modifiable, des imports plus sûrs et la surbrillance du portefeuille.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
