// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.1:6',
  releaseNotes: {
    en_US:
      'Adds privacy-aware, branded transaction exports for copying readable text or saving a PNG image.',
    es_ES:
      'Añade exportaciones de transacciones con marca y respetuosas con la privacidad, para copiar texto legible o guardar una imagen PNG.',
    de_DE:
      'Fügt datenschutzgerechte, gebrandete Transaktionsexporte zum Kopieren als lesbaren Text oder Speichern als PNG-Bild hinzu.',
    pl_PL:
      'Dodaje eksport transakcji z zachowaniem prywatności i własnym oznaczeniem — do skopiowania jako czytelny tekst lub zapisania jako obraz PNG.',
    fr_FR:
      'Ajoute des exports de transaction personnalisés et respectueux de la vie privée, à copier sous forme de texte lisible ou à enregistrer en image PNG.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
