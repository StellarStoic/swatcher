// Copyleft 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { sdk } from '../sdk'
import { notifications } from './notifications'
import { privacyMode } from './privacyMode'
import { privacyIndicators } from './privacyIndicators'
import { smartDiscovery } from './smartDiscovery'
import { theme } from './theme'
import { webPassword } from './webPassword'

export const actions = sdk.Actions.of()
  .addAction(privacyMode)
  .addAction(privacyIndicators)
  .addAction(smartDiscovery)
  .addAction(theme)
  .addAction(notifications)
  .addAction(webPassword)
