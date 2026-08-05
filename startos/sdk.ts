// Copyright (C) 2026 StellarStoic
// SPDX-License-Identifier: AGPL-3.0-or-later

import { StartSdk } from '@start9labs/start-sdk'
import { manifest } from './manifest'

export const sdk = StartSdk.of().withManifest(manifest).build(true)
