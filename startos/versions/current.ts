// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.2:0',
  releaseNotes: {
    en_US:
      'Collapses long transaction address lists to 10 rows and makes wrapped RBF warnings easier to read.',
    es_ES:
      'Contrae las listas largas de direcciones a 10 filas y facilita la lectura de los avisos RBF largos.',
    de_DE:
      'Klappt lange Transaktionsadresslisten auf 10 Zeilen ein und verbessert die Lesbarkeit langer RBF-Warnungen.',
    pl_PL:
      'Zwija długie listy adresów do 10 wierszy i poprawia czytelność długich ostrzeżeń RBF.',
    fr_FR:
      'Réduit les longues listes d’adresses à 10 lignes et améliore la lisibilité des longs avertissements RBF.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
