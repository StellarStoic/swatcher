// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.2:0',
  releaseNotes: {
    en_US:
      'Collapses long transaction address lists to 10 rows, improves wrapped RBF warnings, and confirms watch removal.',
    es_ES:
      'Contrae las listas largas de direcciones a 10 filas, mejora los avisos RBF largos y confirma la eliminación de vigilancias.',
    de_DE:
      'Klappt lange Transaktionsadresslisten auf 10 Zeilen ein, verbessert lange RBF-Warnungen und bestätigt das Entfernen von Beobachtungen.',
    pl_PL:
      'Zwija długie listy adresów do 10 wierszy, poprawia ostrzeżenia RBF i potwierdza usuwanie obserwacji.',
    fr_FR:
      'Réduit les longues listes d’adresses à 10 lignes, améliore les avertissements RBF et confirme la suppression des surveillances.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
