// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { sdk } from '../sdk'
import { notifications } from './notifications'
import { webPassword } from './webPassword'

export const actions = sdk.Actions.of()
  .addAction(notifications)
  .addAction(webPassword)
