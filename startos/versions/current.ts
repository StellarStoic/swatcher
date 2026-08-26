// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.2:0',
  releaseNotes: {
    en_US:
      'Collapses long transaction address lists to 10 rows, with an on-demand control to reveal the remaining addresses.',
    es_ES:
      'Contrae las listas largas de direcciones de transacciones a 10 filas, con un control para mostrar las direcciones restantes.',
    de_DE:
      'Klappt lange Transaktionsadresslisten auf 10 Zeilen ein und zeigt die übrigen Adressen bei Bedarf an.',
    pl_PL:
      'Zwija długie listy adresów transakcji do 10 wierszy i pozwala wyświetlić pozostałe adresy na żądanie.',
    fr_FR:
      'Réduit les longues listes d’adresses de transaction à 10 lignes, avec un bouton permettant d’afficher les adresses restantes.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
