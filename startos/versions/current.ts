// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { IMPOSSIBLE, VersionInfo } from '@start9labs/start-sdk'

export const current = VersionInfo.of({
  version: '0.1.2:0',
  releaseNotes: {
    en_US:
      'Collapses long transaction address lists to 10 rows, improves wrapped RBF warnings, confirms watch removal, and uses the package icon as the browser favicon.',
    es_ES:
      'Contrae las listas largas de direcciones a 10 filas, mejora los avisos RBF largos, confirma la eliminación de vigilancias y usa el icono del paquete como favicon.',
    de_DE:
      'Klappt lange Transaktionsadresslisten auf 10 Zeilen ein, verbessert lange RBF-Warnungen, bestätigt das Entfernen von Beobachtungen und nutzt das Paketsymbol als Favicon.',
    pl_PL:
      'Zwija długie listy adresów do 10 wierszy, poprawia ostrzeżenia RBF, potwierdza usuwanie obserwacji i używa ikony pakietu jako favicony.',
    fr_FR:
      'Réduit les longues listes d’adresses à 10 lignes, améliore les avertissements RBF, confirme la suppression des surveillances et utilise l’icône du paquet comme favicon.',
  },
  migrations: {
    up: async () => {},
    down: IMPOSSIBLE,
  },
})
