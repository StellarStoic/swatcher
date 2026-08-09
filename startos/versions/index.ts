// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { VersionGraph } from '@start9labs/start-sdk'
import { current } from './current'

export const versionGraph = VersionGraph.of({ current, other: [] })
